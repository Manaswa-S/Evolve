package preprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"evolve/debugger"
	"evolve/wikipedia/history"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

type Metrics struct {
	RevsParsed int `json:"Revs Parsed"`

	UsersCacheFound int `json:"Users Cache Found"`
	UsersFetched    int `json:"Users Fetched"`

	RevsProcessed    int       `json:"Revs Processed"`
	RevUsersAnalysed int       `json:"Rev Users Analysed"`
	RevsCleaned      int       `json:"Revs Cleaned"`
	RevsDiffed       int       `json:"Revs Diffed"`
	ProcessStart     time.Time `json:"Process Start"`
	ProcessEnd       time.Time `json:"Process End"`
}

type Commons struct {
	ctx      context.Context
	metrics  *Metrics
	debugger *debugger.Debugger
	dumpDir  string
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
	inpFilePath   string
	dumpDir       string
	analysisFName string

	rawRevsDumpDir string
	cleanDumpDir   string

	//
	fetchUsersChan chan *RevisionMeta
	processRevChan chan *RevisionMeta

	//User cache file path
	userCacheFPath string
	// Caches userID to UserData
	userCache map[int]*UserData
	//

	metrics  *Metrics
	debugger *debugger.Debugger

	Cleaner *Cleaner
	Differ  *Differ
	User    *UserAnalyzer
}

func NewWikiPreprocessor(inpFile string, metaChan chan *RevisionMeta, rootDumpDir string, debugger *debugger.Debugger) (*Preprocessor, error) {
	if inpFile != "" && metaChan != nil {
		return nil, fmt.Errorf("either file or metachan is to be provided")
	}
	if inpFile == "" && metaChan == nil {
		return nil, fmt.Errorf("no file path and metachan provided")
	}

	p := &Preprocessor{
		ROOT_URL:       history.ROOT_URL,
		inpFilePath:    inpFile,
		dumpDir:        rootDumpDir,
		fetchUsersChan: make(chan *RevisionMeta, 10),
		processRevChan: make(chan *RevisionMeta, 10),
		userCache:      make(map[int]*UserData),
		metrics:        new(Metrics),
		debugger:       debugger,
	}

	var err error
	if p.BASE_URL, err = p.prepareRootURLs(); err != nil {
		return nil, err
	}
	if p.USER_URL, err = p.prepareUserURL(); err != nil {
		return nil, err
	}

	p.analysisFName = filepath.Join(p.dumpDir, "0analysis.json")
	p.rawRevsDumpDir = filepath.Join(p.dumpDir, "revs")
	p.cleanDumpDir = filepath.Join(p.dumpDir, "clean")
	p.userCacheFPath = filepath.Join(p.dumpDir, "0users.json")

	return p, nil
}

func (s *Preprocessor) initStages() error {
	commons := &Commons{
		ctx:      s.ctx,
		metrics:  s.metrics,
		debugger: s.debugger,
		dumpDir:  s.dumpDir,
	}
	var err error
	s.Cleaner, err = NewCleaner(commons, s.rawRevsDumpDir, s.cleanDumpDir)
	s.Differ = NewDiffer(commons)
	s.User = NewUserAnalyzer(commons)

	return err
}

func (s *Preprocessor) Run() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.grp, s.grpctx = errgroup.WithContext(s.ctx)

	stopped := func(str string, err error) error {
		s.debugger.Print("\n%s STOPPED. ERROR: %v\n", str, err)
		return err
	}

	s.initStages()

	s.grp.Go(func() error {
		return stopped("consumeForUsers", s.consumeForUsers())
	})

	s.grp.Go(func() error {
		return stopped("consumeForProcess", s.consumeForProcess())
	})

	if s.inpFilePath != "" {
		s.grp.Go(func() error {
			return stopped("startRevsFile", s.startRevsFile())
		})
	}

	s.grp.Go(func() error {
		<-s.grpctx.Done()
		fmt.Println("GOING TO CANCEL THE CONTEXT")
		s.cancel()
		return nil
	})

	return nil
}

func (s *Preprocessor) Stop() error {
	s.cancel()

	err := s.grp.Wait()
	if err != nil {
		return err
	}

	s.debugger.Print("\nALL PROCESSES HAVE STOPPED.\n")

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

	fmt.Printf("\n Time Taken: %d\n", s.metrics.ProcessEnd.Sub(s.metrics.ProcessStart).Milliseconds())

	fmt.Printf("\n AnalyzeUser: %d\n", auTimes.Load())
	fmt.Printf("\n CleanRev: %d\n", crTimes.Load())
	fmt.Printf("\n AnalyzeDiff: %d\n", adTimes.Load())
	fmt.Printf("\n Bots: %d\n", bots.Load())

	return nil
}

func (s *Preprocessor) startRevsFile() error {
	data, err := os.ReadFile(s.inpFilePath)
	if err != nil {
		return err
	}
	revisions := make([]*RevisionMeta, 0)
	if err = json.Unmarshal(data, &revisions); err != nil {
		return err
	}
	s.metrics.RevsParsed = len(revisions)
	if len(revisions) == 0 {
		return fmt.Errorf("empty file")
	}

	fmt.Println("Pushing Revisions for users")
outer:
	for i := len(revisions) - 1; i >= 0; i-- {
		select {
		case <-s.ctx.Done():
			break outer
		case s.fetchUsersChan <- revisions[i]:
		}
	}
	close(s.fetchUsersChan)

	fmt.Println("Pushing Revisions for processing")
exit:
	for i := len(revisions) - 1; i >= 0; i-- {
		select {
		case <-s.ctx.Done():
			break exit
		case s.processRevChan <- revisions[i]:
		}
	}
	close(s.processRevChan)

	return nil
}

// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

func (s *Preprocessor) prepareUsersCache() error {
	data, err := os.ReadFile(s.userCacheFPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, &s.userCache)
}

func (s *Preprocessor) saveUsersCache() error {
	data, err := json.MarshalIndent(s.userCache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.userCacheFPath, data, 0700)
}

func (s *Preprocessor) consumeForUsers() error {
	if err := s.prepareUsersCache(); err != nil {
		return err
	}
	s.debugger.Print("\nCURRENT USER CACHE LEN: %d\n", len(s.userCache))
	s.metrics.UsersCacheFound = len(s.userCache)

	urlChan := make(chan string, 1)
	s.grp.Go(func() error {
		return s.fetchUsersData(urlChan)
	})

	usersBatchLim := 49
	usersBatchMap := make(map[int]struct{})
	ususersParam := "ususerids"

	url := *s.USER_URL
	params := url.Query()
	getURL := func() string {
		var sb strings.Builder
		first := true
		for id := range usersBatchMap {
			if !first {
				sb.WriteByte('|')
			} else {
				first = false
			}
			sb.WriteString(strconv.Itoa(id))
		}
		params.Set(ususersParam, sb.String())
		url.RawQuery = params.Encode()

		return url.String()
	}

	// flushes the buffer, fetches the results and caches them
	flushUserBatch := func() {
		if len(usersBatchMap) == 0 {
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case urlChan <- getURL():
			clear(usersBatchMap)
		}
	}

outer:
	for {
		select {
		case <-s.ctx.Done():
			break outer
		case meta, ok := <-s.fetchUsersChan:
			if !ok {
				break outer
			}
			_, cached := s.userCache[meta.UserID]
			_, exists := usersBatchMap[meta.UserID]
			if !cached && !exists {
				usersBatchMap[meta.UserID] = struct{}{}
			}

			if len(usersBatchMap) == usersBatchLim {
				flushUserBatch()
			}
		}
	}

	if len(usersBatchMap) > 0 {
		flushUserBatch()
	}

	close(urlChan)

	if err := s.saveUsersCache(); err != nil {
		return err
	}

	return nil
}

func (s *Preprocessor) fetchUsersData(urlChan chan string) error {
	client := http.Client{}
	rate := 3                     // Max number of requests in 1 second
	timeBtn := int64(1000 / rate) // Minimum time gap between 2 requests to maintain rate
	lastReq := int64(0)           // UnixMilliseconds of last request

outer:
	for {
		select {
		case <-s.ctx.Done():
			break outer
		case url, ok := <-urlChan:
			if !ok {
				break outer
			}

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("User-Agent", history.USER_AGENT)

			diff := time.Now().UnixMilli() - lastReq
			if diff < timeBtn {
				time.Sleep(time.Duration(timeBtn-diff) * time.Millisecond)
			}

			lastReq = time.Now().UnixMilli()
			resp, err := client.Do(req)
			if err != nil {
				return err
			}

			d := fmt.Sprintf("Fetching user data: %s : %dms : %d\n", url, time.Now().UnixMilli()-lastReq, resp.StatusCode)
			s.debugger.Debug(d)

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
		}
	}

	return nil
}

// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

func (s *Preprocessor) consumeForProcess() error {
	revAnalyses := make([]*RevisionAnalysis, 0)

	parallel := func(revCtx *RevisionAnalysis) error {
		if err := s.analyzeRev(revCtx); err != nil {
			revCtx.Debug.Errors = append(revCtx.Debug.Errors, err)
		}
		return nil
	}

	grp := errgroup.Group{}
	grp.SetLimit(40)

	first := true

outer:
	for {
		select {
		case <-s.ctx.Done():
			break outer
		case meta, ok := <-s.processRevChan:
			if !ok {
				break outer
			}
			if first {
				s.metrics.ProcessStart = time.Now().UTC()
				first = false
			}

			userData, exists := s.userCache[meta.UserID]
			if !exists {
				return fmt.Errorf("user data cache wasn't found: %d", meta.UserID)
			}

			revCtx := &RevisionAnalysis{
				Process: &ProcessCtx{
					Meta: meta,
					User: userData,
				},
				Tags:       new(RevisionTags),
				Confidence: new(RevisionConfidence),
				Diffs:      new(RevisionDiffs),
				Debug:      new(RevisionDebug),
			}
			revAnalyses = append(revAnalyses, revCtx)

			grp.Go(func() error {
				return parallel(revCtx)
			})

			s.metrics.RevsProcessed += 1

			if s.metrics.RevsProcessed%100 == 0 {
				s.debugger.Print("\nRevs Processed: %d", s.metrics.RevsProcessed)
			}
		}
	}

	data, err := json.MarshalIndent(revAnalyses, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(s.analysisFName, data, 0700)
	if err != nil {
		return err
	}

	s.metrics.ProcessEnd = time.Now().UTC()

	return nil
}

var auTimes atomic.Int64
var crTimes atomic.Int64
var adTimes atomic.Int64

var bots atomic.Int64

func (s *Preprocessor) analyzeRev(r *RevisionAnalysis) error {
	var err error

	t1 := time.Now()
	err = s.User.analyzeUser(r)
	if err != nil {
		return err
	}
	s.metrics.RevUsersAnalysed++
	auTimes.Add(time.Since(t1).Milliseconds())

	t2 := time.Now()
	revClean, err := s.Cleaner.cleanRev(r)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return err
	}
	s.metrics.RevsCleaned++
	crTimes.Add(time.Since(t2).Milliseconds())

	t3 := time.Now()
	err = s.Differ.analyzeDiff(r, revClean)
	if err != nil {
		return err
	}
	s.metrics.RevsDiffed++
	adTimes.Add(time.Since(t3).Milliseconds())

	return nil
}
