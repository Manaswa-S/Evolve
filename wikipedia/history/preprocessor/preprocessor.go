package preprocessor

import (
	"context"
	"encoding/json"
	"evolve/debugger"
	"evolve/wikipedia/history"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type Metrics struct {
	UsersFetched  int `json:"Users Fetched"`
	RevsParsed    int `json:"Revs Parsed"`
	RevsProcessed int `json:"Revs Processed"`
	RevsCleaned   int `json:"Revs Cleaned"`
}

type Preprocessor struct {
	ROOT_URL string
	BASE_URL string
	USER_URL *url.URL

	ctx    context.Context
	cancel context.CancelFunc
	grp    *errgroup.Group
	grpctx context.Context

	// The path of the file that has the ids' metadata
	rawFilePath    string
	dumpDir        string
	analysisFName  string
	rawRevsDumpDir string
	cleanDumpDir   string

	// Raw input channel
	rawMetaChan chan *RevisionMeta

	// Users data fetch channel
	usersURLChan chan string
	// Users data fetched, cached
	usersFetchedChan chan struct{}

	// Process the revision
	processChan chan *ProcessCtx

	// Caches userID to UserData
	userCache map[int]*UserData

	metrics *Metrics

	debugger *debugger.Debugger
}

func NewWikiPreprocessor(filePath string, metaChan chan *RevisionMeta, rootDumpDir string, debugger *debugger.Debugger) (*Preprocessor, error) {
	if filePath != "" && metaChan != nil {
		return nil, fmt.Errorf("either file or metachan is to be provided")
	}

	if filePath == "" && metaChan == nil {
		return nil, fmt.Errorf("no file path and metachan provided")
	}

	if filePath != "" && metaChan == nil {
		// process file
		metaChan = make(chan *RevisionMeta, 100)
	}

	p := &Preprocessor{
		ROOT_URL:         history.ROOT_URL,
		rawFilePath:      filePath,
		dumpDir:          rootDumpDir,
		rawMetaChan:      metaChan,
		usersURLChan:     make(chan string, 1),
		usersFetchedChan: make(chan struct{}, 1),
		processChan:      make(chan *ProcessCtx, 100), // Gauge this properly depending on your processing times
		userCache:        make(map[int]*UserData),
		metrics:          new(Metrics),
		debugger:         debugger,
	}

	var err error
	p.BASE_URL, err = p.prepareRootURLs()
	if err != nil {
		return nil, err
	}

	p.USER_URL, err = p.prepareUserURL()
	if err != nil {
		return nil, err
	}

	p.analysisFName = filepath.Join(p.dumpDir, "0analysis.json")
	p.rawRevsDumpDir = filepath.Join(p.dumpDir, "revs")
	p.cleanDumpDir = filepath.Join(p.dumpDir, "clean")

	return p, nil
}

// Prepare
func (s *Preprocessor) prepareRootURLs() (string, error) {
	u, err := url.Parse(s.ROOT_URL)
	if err != nil {
		return "", err
	}

	baseQueries := map[string]string{
		"action":        "query",
		"format":        "json",
		"formatversion": "2",
	}
	params := url.Values{}
	for key, val := range baseQueries {
		params.Set(key, val)
	}
	u.RawQuery = params.Encode()

	return u.String(), nil
}

func (s *Preprocessor) prepareUserURL() (*url.URL, error) {
	u, err := url.Parse(s.BASE_URL)
	if err != nil {
		return nil, err
	}

	pageQueries := map[string]string{
		"list":      "users",
		"usprop":    "groups|editcount|registration",
		"ususerids": "",
	}
	params := u.Query()
	for key, val := range pageQueries {
		params.Set(key, val)
	}
	u.RawQuery = params.Encode()

	return u, nil
}

func (s *Preprocessor) Run() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.grp, s.grpctx = errgroup.WithContext(s.ctx)

	stopped := func(str string) {
		fmt.Printf("\n%s STOPPED.\n", str)
	}

	s.grp.Go(func() error {
		err := s.consumeRaw()
		stopped("consumeRaw")
		return err
	})

	s.grp.Go(func() error {
		err := s.fetchUsersData()
		stopped("fetchUsersData")
		return err
	})

	s.grp.Go(func() error {
		err := s.processRev()
		stopped("processRev")
		return err
	})

	if s.rawFilePath != "" {
		s.grp.Go(func() error {
			err := s.startRevsFile()
			stopped("startRevsFile")
			return err
		})
	}

	return nil
}

func (s *Preprocessor) Stop() error {
	s.cancel()

	err := s.grp.Wait()
	if err != nil {
		return err
	}

	fmt.Printf("\nALL PROCESSES HAVE STOPPED.\n")

	return nil
}

func (s *Preprocessor) PrintMetrics() error {
	fmt.Printf("\n\n")
	fmt.Printf("Metrics:\n")

	data, err := json.MarshalIndent(s.metrics, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", string(data))

	return nil
}

// The file should be of []RevisionMeta. Size should be relatively small.
func (s *Preprocessor) startRevsFile() error {
	data, err := os.ReadFile(s.rawFilePath)
	if err != nil {
		return err
	}

	revisions := make([]*RevisionMeta, 0)
	err = json.Unmarshal(data, &revisions)
	if err != nil {
		return err
	}

	if len(revisions) == 0 {
		return fmt.Errorf("empty file")
	}

	fmt.Println("Pushing Revisions")
outer:
	for i := len(revisions) - 1; i >= 0; i-- {
		rev := revisions[i]
		select {
		case <-s.ctx.Done():
			break outer
		case s.rawMetaChan <- rev:
			s.metrics.RevsParsed += 1
		}
	}

	close(s.rawMetaChan)

	return nil
}

// Consume
func (s *Preprocessor) consumeRaw() error {
	userBatchLim := 20
	userBatch := make([]int, 0)
	ususersParam := "ususerids"

	processBatchLim := 20
	processBatch := make([]*ProcessCtx, 0)

	url := *s.USER_URL
	params := url.Query()

	getURL := func(batch []int) string {
		var sb strings.Builder
		for i, p := range batch {
			if i > 0 {
				sb.WriteByte('|')
			}
			sb.WriteString(strconv.Itoa(p))
		}
		params.Set(ususersParam, sb.String())
		url.RawQuery = params.Encode()

		return url.String()
	}

	// flushes the buffer, fetches the results and caches them
	flushUserBatch := func() {
		s.usersURLChan <- getURL(userBatch)
		// The fetchUsersData() should complete before proceeding as
		// further cache misses can trigger unwanted errors.
		// Using a dedicated channel instead of a direct call has no advantage.
		// TODO:
		<-s.usersFetchedChan
		userBatch = userBatch[:0]
	}

	flushProcessBatch := func() error {
		// This can be debated to be optional, but it's better to group all data.
		// Storage and compute is both cheap right now.
		for _, proc := range processBatch {
			user, ok := s.userCache[proc.Meta.UserID]
			if !ok {
				return fmt.Errorf("user not cached, impossible condition")
			}
			proc.User = user
			// flush the processBatch as well now.
			s.processChan <- proc
		}

		processBatch = processBatch[:0]

		return nil
	}

outer:
	for meta := range s.rawMetaChan {
		select {
		case <-s.ctx.Done():
			break outer
		default:
			processBatch = append(processBatch, &ProcessCtx{
				Meta: meta,
			})
			// check if that userID is already cached
			_, ok := s.userCache[meta.UserID]
			if !ok {
				userBatch = append(userBatch, meta.UserID)
			}

			if len(processBatch) == processBatchLim ||
				len(userBatch) == userBatchLim {
				flushUserBatch()
				if err := flushProcessBatch(); err != nil {
					return err
				}
			}
		}
	}

	if len(userBatch) > 0 {
		flushUserBatch()
		if err := flushProcessBatch(); err != nil {
			return err
		}
	}

	close(s.usersURLChan)
	close(s.usersFetchedChan)
	close(s.processChan)

	return nil
}

// Fetch user data
func (s *Preprocessor) fetchUsersData() error {
	client := http.Client{}
	rate := 3                     // Max number of requests in 1 second
	timeBtn := int64(1000 / rate) // Minimum time gap between 2 requests to maintain rate
	lastReq := int64(0)           // UnixMilliseconds of last request

	for url := range s.usersURLChan {
		// Fetch the URL
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", history.USER_AGENT)

		diff := time.Now().UnixMilli() - lastReq
		if diff < timeBtn {
			time.Sleep(time.Duration(timeBtn-diff) * time.Millisecond)
		}

		fmt.Printf("Fetching user data: %s : ", url)

		lastReq = time.Now().UnixMilli()
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		fmt.Printf("%dms : %d\n", time.Now().UnixMilli()-lastReq, resp.StatusCode)

		// TODO: proper fallback and wait
		if resp.StatusCode != 200 {
			return fmt.Errorf("status non-200")
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		batch := new(UserDataBatch)
		err = json.Unmarshal(body, batch)
		if err != nil {
			return err
		}

		if batch.Query == nil || batch.Query.Users == nil || len(batch.Query.Users) == 0 {
			return fmt.Errorf("users is empty")
		}

		// set all fetched users in the cache
		for _, user := range batch.Query.Users {
			s.metrics.UsersFetched += 1
			s.userCache[user.UserID] = user
		}

		s.usersFetchedChan <- struct{}{}
	}

	return nil
}

// Process revisions coordinator
func (s *Preprocessor) processRev() error {
	var err error
	analysisF, err := os.OpenFile(s.analysisFName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	_, err = analysisF.Write([]byte("["))
	if err != nil {
		return err
	}
	defer func() {
		analysisF.Write([]byte("\n]"))
		analysisF.Close()
	}()
	first := true

	for proc := range s.processChan {
		s.metrics.RevsProcessed += 1

		revCtx := &RevisionCtx{
			Process:    proc,
			Tags:       new(RevisionTags),
			Confidence: new(RevisionConfidence),
			Diffs:      new(RevisionDiffs),
			Debug:      new(RevisionDebug),
		}

		if err = s.analyzeRev(revCtx); err != nil {
			revCtx.Debug.Errors = append(revCtx.Debug.Errors, err)
		}

		data, err := json.MarshalIndent(revCtx, "", "  ")
		if err != nil {
			return err
		}
		if first {
			first = false
		} else {
			analysisF.Write([]byte(",\n"))
		}
		_, err = analysisF.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Preprocessor) analyzeRev(r *RevisionCtx) error {
	var err error

	err = s.analyzeUser(r)
	if err != nil {
		return err
	}

	revClean, err := s.cleanRev(r)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return err
	}

	err = s.analyzeDiff(r, revClean)
	if err != nil {
		return err
	}

	return nil
}

// groups: the 'bot' flag.
// inclusion of the 'use this bot' word in the comments

// bot detection
// char delta threshold crossed
// section changed detection
// confidence depending on the flags like 'reviewer' , etc.
//
