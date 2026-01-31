package compressor

import "time"

type RevisionClean struct {
	RevID         int       `json:"revid"`
	ParentID      int       `json:"parentid"`
	TimeStamp     time.Time `json:"timestamp"`
	ContentFormat string    `json:"contentformat"`
	Content       string    `json:"content"`
}
