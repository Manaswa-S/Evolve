package compressor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Compressor struct {
	rootDir string
}

func NewCompressor(rootDir string) *Compressor {
	return &Compressor{
		rootDir: rootDir,
	}
}

func (s *Compressor) Run() error {
	cleanDir := filepath.Join(s.rootDir, "clean")
	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		return err
	}

	outFile := filepath.Join(s.rootDir, "compress.txt")

	outF, err := os.Create(outFile)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fName := entry.Name()
		fPath := filepath.Join(cleanDir, fName)

		data, err := os.ReadFile(fPath)
		if err != nil {
			return err
		}
		clean := new(RevisionClean)
		if err = json.Unmarshal(data, clean); err != nil {
			return err
		}

		_, err = outF.Write([]byte("\n"))
		if err != nil {
			return err
		}
		_, err = outF.WriteString(clean.Content)
		if err != nil {
			return err
		}
	}

	outF.Close()
	fmt.Println("COMPRESS DONE")

	return nil
}
