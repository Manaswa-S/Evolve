package scraper

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
	PagesFetched int `json:"Pages Fetched"`
	RevsFetched  int `json:"RevsFetched"`
}

type Scraper struct {
	ROOT_URL string
	BASE_URL string
	PAGE_URL *url.URL
	REVS_URL *url.URL

	pageID    int
	pageTitle string

	dumpDir        string
	revsDir        string
	configFilePath string
	metaFilePath   string
	idsFilePath    string

	ctx    context.Context
	cancel context.CancelFunc
	grpctx context.Context
	grp    *errgroup.Group

	idChan      chan int
	revsUrlChan chan string
	revsChan    chan *RevisionsContentBatch
	idsSaveChan chan *RevisionMeta

	metrics  *Metrics
	debugger *debugger.Debugger
}

func NewWikiScrape(title, rootDumpDir string, debugger *debugger.Debugger) (*Scraper, error) {
	s := &Scraper{
		ROOT_URL: history.ROOT_URL,
		metrics:  new(Metrics),
		debugger: debugger,
	}
	var err error

	s.BASE_URL, err = s.prepareRootURLs()
	if err != nil {
		return nil, err
	}

	resolve, err := s.resolveTitle(title)
	if err != nil {
		return nil, err
	}
	s.pageID = resolve.Query.Pages[0].PageID
	s.pageTitle = resolve.Query.Pages[0].Title

	s.PAGE_URL, err = s.preparePageURL()
	if err != nil {
		return nil, err
	}
	s.REVS_URL, err = s.prepareRevsURL()
	if err != nil {
		return nil, err
	}

	s.dumpDir = filepath.Join(rootDumpDir, title)
	err = os.MkdirAll(s.dumpDir, 0700)
	if err != nil {
		return nil, err
	}

	s.revsDir = filepath.Join(rootDumpDir, title, "revs")
	err = os.MkdirAll(s.revsDir, 0700)
	if err != nil {
		return nil, err
	}
	s.configFilePath = filepath.Join(s.dumpDir, "0config.txt")
	s.metaFilePath = filepath.Join(s.dumpDir, "0meta.txt")
	s.idsFilePath = filepath.Join(s.dumpDir, "0ids.json")
	err = s.saveMeta(resolve)
	if err != nil {
		return nil, err
	}

	s.idChan = make(chan int, 60)
	s.idsSaveChan = make(chan *RevisionMeta, 60)
	s.revsUrlChan = make(chan string, 5)
	s.revsChan = make(chan *RevisionsContentBatch, 10)

	return s, nil
}

// Prepare
func (s *Scraper) prepareRootURLs() (string, error) {
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

func (s *Scraper) preparePageURL() (*url.URL, error) {
	u, err := url.Parse(s.BASE_URL)
	if err != nil {
		return nil, err
	}

	pageQueries := map[string]string{
		"pageids":   strconv.Itoa(s.pageID),
		"prop":      "revisions",
		"rvslots":   "main",
		"rvprop":    "ids|timestamp|size|user|userid|comment",
		"rvlimit":   "500",
		"redirects": "1",
	}
	params := u.Query()
	for key, val := range pageQueries {
		params.Set(key, val)
	}
	u.RawQuery = params.Encode()

	return u, nil
}

func (s *Scraper) prepareRevsURL() (*url.URL, error) {
	u, err := url.Parse(s.BASE_URL)
	if err != nil {
		return nil, err
	}

	revQueries := map[string]string{
		"revids":  "1",
		"prop":    "revisions",
		"rvslots": "main",
		"rvprop":  "ids|timestamp|content|user|comment",
	}
	params := u.Query()
	for key, val := range revQueries {
		params.Set(key, val)
	}
	u.RawQuery = params.Encode()

	return u, nil
}

// Resolve
func (s *Scraper) resolveTitle(title string) (*ResolveResponse, error) {
	u, err := url.Parse(s.BASE_URL)
	if err != nil {
		return nil, err
	}
	queries := map[string]string{
		"titles":    title,
		"redirects": "1",
	}
	params := u.Query()
	for key, val := range queries {
		params.Set(key, val)
	}
	u.RawQuery = params.Encode()

	client := http.Client{}
	url := u.String()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", history.USER_AGENT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resolve := new(ResolveResponse)
	err = json.Unmarshal(body, resolve)
	if err != nil {
		return nil, err
	}

	if resolve.Query == nil || resolve.Query.Pages == nil || len(resolve.Query.Pages) == 0 {
		return nil, fmt.Errorf("resolve is empty")
	}

	return resolve, nil
}

func (s *Scraper) saveMeta(resolve *ResolveResponse) error {
	metaF, err := os.OpenFile(s.metaFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer metaF.Close()

	resolveMarshaled, err := json.MarshalIndent(resolve, "", " ")
	if err != nil {
		return err
	}

	_, err = metaF.Write(resolveMarshaled)
	if err != nil {
		return err
	}

	return nil
}

func (s *Scraper) Run() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.grp, s.grpctx = errgroup.WithContext(s.ctx)

	stopped := func(str string) {
		fmt.Printf("\n%s STOPPED.\n", str)

	}

	s.grp.Go(func() error {
		err := s.saveRevs()
		stopped("saveRevs")
		return err
	})

	s.grp.Go(func() error {
		err := s.saveIds()
		stopped("saveIds")
		return err
	})

	s.grp.Go(func() error {
		err := s.fetchRevs()
		stopped("fetchRevs")
		return err
	})

	s.grp.Go(func() error {
		err := s.prepareRevParam()
		stopped("prepareRevParam")
		return err
	})

	s.grp.Go(func() error {
		err := s.fetchIds()
		stopped("fetchIds")
		return err
	})

	return nil
}

func (s *Scraper) Stop() error {
	s.cancel()

	err := s.grp.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (s *Scraper) PrintMetrics() error {
	fmt.Printf("\n\n")
	fmt.Printf("Metrics:\n")

	data, err := json.MarshalIndent(s.metrics, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", string(data))

	return nil
}

// Fetch IDs'
func (s *Scraper) fetchIds() error {
	client := http.Client{}

	continuationVal := ""
	continuationParam := "rvcontinue"

	url := *s.PAGE_URL
	params := url.Query()

outer:
	for {
		select {
		case <-s.ctx.Done():
			break outer
		default:
			// Add continuation param and prepare URL and Request
			if continuationVal != "" {
				params.Set(continuationParam, continuationVal)
				url.RawQuery = params.Encode()
			}
			pageurl := url.String()
			req, err := http.NewRequest("GET", pageurl, nil)
			if err != nil {
				return err
			}
			req.Header.Set("User-Agent", history.USER_AGENT)

			// Fetch the page
			fmt.Printf("Fetching page: %s: ", pageurl)
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			fmt.Printf("%d\n", resp.StatusCode)

			// Read the response, status
			pageBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			page := new(RevisionIndex)
			err = json.Unmarshal(pageBody, page)
			if err != nil {
				return fmt.Errorf("pageBody unmarshall error: %v", err)
			}
			if page.Query == nil || page.Query.Pages == nil {
				return fmt.Errorf("Page is nil")
			}
			firstPage := page.Query.Pages[0]

			// Push all IDs to the channel
			for _, revMeta := range firstPage.Revisions {
				s.idChan <- revMeta.RevID
				s.idsSaveChan <- revMeta
			}
			s.metrics.PagesFetched += 1

			if page.Continue != nil {
				continuationVal = page.Continue.RvContinue
			} else {
				break outer
			}
		}
	}

	close(s.idChan)
	close(s.idsSaveChan)

	return nil
}

// Prepare Rev params
func (s *Scraper) prepareRevParam() error {
	batch := make([]int, 0)
	batchLen := 20
	revIdsParam := "revids"

	url := *s.REVS_URL
	params := url.Query()

	process := func() string {
		// Concatenate IDs and return the URL string
		var sb strings.Builder
		for i := 0; i < len(batch); i++ {
			if i > 0 {
				sb.WriteByte('|')
			}
			sb.WriteString(strconv.Itoa(batch[i]))
		}
		params.Set(revIdsParam, sb.String())
		url.RawQuery = params.Encode()

		return url.String()
	}

	for id := range s.idChan {
		// Batch the ID
		batch = append(batch, id)

		// Process batch if limit hit
		if len(batch) == batchLen {
			urlStr := process()
			s.revsUrlChan <- urlStr
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		urlStr := process()
		s.revsUrlChan <- urlStr
	}

	close(s.revsUrlChan)

	return nil
}

// Fetch Revs
func (s *Scraper) fetchRevs() error {
	client := http.Client{}
	rate := 3                     // Max number of requests in 1 second
	timeBtn := int64(1000 / rate) // Minimum time gap between 2 requests to maintain rate
	lastReq := int64(0)           // UnixMilliseconds of last request

	for url := range s.revsUrlChan {
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

		fmt.Printf("Fetching revision: %s : ", url)

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
		revBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		allRevs := new(RevisionsContentBatch)
		err = json.Unmarshal(revBody, allRevs)
		if err != nil {
			return err
		}

		// Send the Revisions to the worker for async saving to disk
		s.revsChan <- allRevs
	}

	close(s.revsChan)

	return nil
}

// Save Revs
func (s *Scraper) saveRevs() error {
	configF, err := os.OpenFile(s.configFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer configF.Close()

	for revs := range s.revsChan {
		if revs.Query == nil || revs.Query.Pages == nil || len(revs.Query.Pages) == 0 {
			return fmt.Errorf("revs are nil")
		}
		revisions := revs.Query.Pages[0].Revisions

		for _, singleRev := range revisions {

			fileName := fmt.Sprintf("%d-%d.json", singleRev.TimeStamp.Unix(), singleRev.RevID)
			filePath := filepath.Join(s.revsDir, fileName)
			f, err := os.Create(filePath)
			if err != nil {
				return err
			}
			revMarshal, err := json.MarshalIndent(singleRev, "", "  ")
			if err != nil {
				return fmt.Errorf("revMarshall error: %v", err)
			}
			_, err = f.Write(revMarshal)
			if err != nil {
				return err
			}
			f.Close()

			s.metrics.RevsFetched += 1
		}

		configF.Seek(0, 0)
		configF.Truncate(0)
		lastRev := revisions[len(revisions)-1]
		fmt.Fprintf(configF, "%d-%d", lastRev.RevID, lastRev.ParentID)
	}

	return nil
}

// Save the Index
func (s *Scraper) saveIds() error {
	idsF, err := os.OpenFile(s.idsFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer idsF.Close()

	_, err = idsF.Write([]byte("[\n"))
	if err != nil {
		return err
	}
	defer func() {
		idsF.Write([]byte("\n]"))
		idsF.Close()
	}()

	first := true

	for rev := range s.idsSaveChan {
		marshal, err := json.MarshalIndent(rev, "", " ")
		if err != nil {
			return err
		}
		if first {
			first = false
		} else {
			idsF.Write([]byte(",\n"))
		}
		_, err = idsF.Write(marshal)
		if err != nil {
			return err
		}
	}

	return nil
}
