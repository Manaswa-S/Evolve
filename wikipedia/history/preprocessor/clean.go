package preprocessor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Cleans the wiki markup using simple rules
func (s *Preprocessor) cleanRev(rc *RevisionCtx) (*RevisionClean, error) {
	revFName := fmt.Sprintf("%d-%d.json", rc.Process.Meta.TimeStamp.Unix(), rc.Process.Meta.RevID)
	revPath := filepath.Join(s.rawRevsDumpDir, revFName)

	revData, err := os.ReadFile(revPath)
	if err != nil {
		return nil, err
	}
	revRaw := new(RevisionContent)
	err = json.Unmarshal(revData, revRaw)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"pandoc",
		"-f", "mediawiki",
		"-t", "plain",
		"--wrap=none",
		"--strip-comments",
		"--quiet",
	)

	cmd.Stdin = bytes.NewReader([]byte(revRaw.Slots.Main.Content))

	var out bytes.Buffer
	cmd.Stdout = &out

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pandoc failed: %w: %s", err, stderr.String())
	}

	revClean := &RevisionClean{
		RevID:         revRaw.RevID,
		ParentID:      revRaw.ParentID,
		TimeStamp:     revRaw.TimeStamp,
		ContentFormat: "plaintext",
		Content:       out.String(),
	}
	revCleanData, err := json.MarshalIndent(revClean, "", " ")
	if err != nil {
		return nil, err
	}

	outPath := filepath.Join(s.cleanDumpDir, revFName)
	err = os.MkdirAll(s.cleanDumpDir, 0700)
	if err != nil {
		return nil, err
	}
	cleanF, err := os.Create(outPath)
	if err != nil {
		return nil, err
	}

	_, err = cleanF.Write(revCleanData)
	if err != nil {
		return nil, err
	}

	s.metrics.RevsCleaned += 1

	return revClean, nil
}

// pass through pandoc for mediawiki to plain text
// then do another pass that normalizes '==' to [SECTION: ]
// then we can pass it to the diff.

// pandoc -f mediawiki -t plain --wrap=none --strip-comments --quiet ./dump/wikipedia/ML/revs/media.wiki -o output.txt
