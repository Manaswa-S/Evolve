package preprocessor

import (
	"bytes"
	"context"
	"encoding/json"
	"evolve/debugger"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Cleaner struct {
	cleanDir string
	rawDir   string

	pandocServerCount int
	pandocRRURLChan   chan string
	pandocHTTPClient  *http.Client
	pandocCtxCancel   context.CancelFunc

	ctx      context.Context
	dumpDir  string
	metrics  *Metrics
	debugger *debugger.Debugger
}

func NewCleaner(commons *Commons, rawDir, cleanDir string) (*Cleaner, error) {
	c := &Cleaner{
		cleanDir: cleanDir,
		rawDir:   rawDir,

		ctx:      commons.ctx,
		dumpDir:  commons.dumpDir,
		metrics:  commons.metrics,
		debugger: commons.debugger,
	}

	err := os.MkdirAll(c.cleanDir, 0700)
	if err != nil {
		return nil, err
	}

	c.pandocServerCount = 3
	c.pandocRRURLChan = make(chan string, c.pandocServerCount)
	c.pandocHTTPClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	c.startPandocServers()

	return c, nil
}

func (s *Cleaner) startPandocServers() error {
	pandocCtx, cancel := context.WithCancel(s.ctx)
	s.pandocCtxCancel = cancel
	
	for i := range s.pandocServerCount {
		port := fmt.Sprintf("%d", 3030+i)

		cmd := exec.CommandContext(pandocCtx, "pandoc-server",
			"--port", port,
			"--timeout", "30",
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("pandoc-server start ERROR: %v", err)
		}

		go func() {
			cmd.Wait()
		}()

		url := fmt.Sprintf("http://localhost:%s", port)
		s.pandocRRURLChan <- url

		for {
			_, err := http.Get(url)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

type PandocReq struct {
	Text       string   `json:"text"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Extensions []string `json:"extensions"`
	Wrap       string   `json:"wrap"`
}

type PandocResp struct {
	Output string `json:"output"`
	Base64 bool   `json:"base64"`
	// Message []struct {
	// 	Verbosity string `json:"verbosity"`
	// 	Message   string `json:"message"`
	// } `json:"messages"`
}

func (s *Cleaner) cleanRev(rc *RevisionAnalysis) (*RevisionClean, error) {
	revFName := fmt.Sprintf("%d-%d.json", rc.Process.Meta.TimeStamp.Unix(), rc.Process.Meta.RevID)
	revPath := filepath.Join(s.rawDir, revFName)

	revData, err := os.ReadFile(revPath)
	if err != nil {
		return nil, err
	}
	revRaw := new(RevisionContent)
	err = json.Unmarshal(revData, revRaw)
	if err != nil {
		return nil, err
	}

	// Pandoc

	pandocReq := PandocReq{
		Text:       revRaw.Slots.Main.Content,
		From:       "mediawiki",
		To:         "plain",
		Extensions: []string{"strip-comments"},
		Wrap:       "none",
	}
	pandocReqBytes, err := json.Marshal(pandocReq)
	if err != nil {
		return nil, fmt.Errorf("pandoc body marshall err: %v", err)
	}

	url := <-s.pandocRRURLChan
	defer func() { s.pandocRRURLChan <- url }()

	req, err := http.NewRequest("POST", url, bytes.NewReader(pandocReqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := s.pandocHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pandoc returned non-200 ERROR")
	}

	pandocRespBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	pandocResp := new(PandocResp)
	err = json.Unmarshal(pandocRespBytes, pandocResp)
	if err != nil {
		return nil, fmt.Errorf("pandoc resp unmarshall err: %v", err)
	}

	//

	revClean := &RevisionClean{
		RevID:         revRaw.RevID,
		ParentID:      revRaw.ParentID,
		TimeStamp:     revRaw.TimeStamp,
		ContentFormat: "plaintext",
		Content:       pandocResp.Output,
	}
	revCleanBytes, err := json.MarshalIndent(revClean, "", " ")
	if err != nil {
		return nil, err
	}

	outPath := filepath.Join(s.cleanDir, revFName)
	err = os.WriteFile(outPath, revCleanBytes, 0700)
	if err != nil {
		return nil, err
	}

	return revClean, nil
}
