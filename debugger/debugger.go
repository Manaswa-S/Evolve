package debugger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Debugger struct {
	fileName string
	f        *os.File
}

func NewDebugger() (*Debugger, error) {
	nano := time.Now().Unix()

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	fPath := filepath.Join(wd, "logs")
	fName := filepath.Join(wd, "logs", fmt.Sprintf("debug.%d.txt", nano))
	err = os.MkdirAll(fPath, 0700)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(fName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return &Debugger{
		fileName: fName,
		f:        f,
	}, nil
}

func (s *Debugger) Debug(str string) {
	fmt.Fprintf(s.f, "\nDEBUG : %s", str)
}
