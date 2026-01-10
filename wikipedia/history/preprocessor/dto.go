package preprocessor

import "time"

type RevisionContentSlotsMain struct {
	ContentModel  string `json:"contentmodel"`
	ContentFormat string `json:"contentformat"`
	Content       string `json:"content"`
}

type RevisionContentSlots struct {
	Main RevisionContentSlotsMain `json:"main"`
}

type RevisionContent struct {
	RevID     int                  `json:"revid"`
	ParentID  int                  `json:"parentid"`
	TimeStamp time.Time            `json:"timestamp"`
	Slots     RevisionContentSlots `json:"slots"`
	User      string               `json:"user"`
	Comment   string               `json:"comment"`
}

//

type RevisionMeta struct {
	RevID     int       `json:"revid"`
	ParentID  int       `json:"parentid"`
	TimeStamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
	User      string    `json:"user"`
	UserID    int       `json:"userid"`
	Comment   string    `json:"comment"`
}

// User Data Response Batch

type UserData struct {
	UserID       int       `json:"userid"`
	Name         string    `json:"name"`
	EditCount    int       `json:"editcount"`
	Registration time.Time `json:"registration"`
	Groups       []string  `json:"groups"`
}

type UserDataQuery struct {
	Users []*UserData `json:"users"`
}

type UserDataBatch struct {
	BatchComplete bool           `json:"batchcomplete"`
	Query         *UserDataQuery `json:"query"`
}

// Internal Processed Data

type ProcessCtx struct {
	Meta *RevisionMeta `json:"meta"`
	User *UserData     `json:"user"`
}

type RevisionTags struct {
	IsMicroEdit        bool `json:"isMicroEdit"`
	IsStructural       bool `json:"isStructural"`
	IsContentExpansion bool `json:"isContentExpansion"`
	IsCitationOnly     bool `json:"isCitationOnly"`
	IsDefinitionChange bool `json:"isDefinitionChange"`
}

type RevisionConfidence struct {
	// Automation likelihood
	Automation int // 0–100
	// Maintenance / moderation behavior
	Maintenance int // 0–100
	// Structural / non-content edits
	Structural int // 0–100
	// Human content contribution confidence
	Human int // 0–100
}

type RevisionDebug struct {
	Errors   []error `json:"errors"`
	Warnings []error `json:"warnings"`
}

type RevisionDiffs struct {
	Inserted  int `json:"inserted"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`

	SymmetricScore      int `json:"symmetricScore"`
	EditDistanceScore   int `json:"editDistanceScore"`
	SemanticChangeScore int `json:"semanticChangeScore"`
	LogScaledScore      int `json:"logScaledScore"`
	FinalScore          int `json:"finalScore"`

	ChangeScore  int    `json:"changedScore"`
	BalanceScore int    `json:"balanceScore"`
	TypeOfEdit   string `json:"typeOfEdit"`
}

type RevisionCtx struct {
	Process    *ProcessCtx         `json:"process"`
	Tags       *RevisionTags       `json:"tags"`
	Confidence *RevisionConfidence `json:"confidence"`
	Diffs      *RevisionDiffs      `json:"diffs"`

	Debug *RevisionDebug `json:"debug"`
}

//

type RevisionClean struct {
	RevID         int       `json:"revid"`
	ParentID      int       `json:"parentid"`
	TimeStamp     time.Time `json:"timestamp"`
	ContentFormat string    `json:"contentformat"`
	Content       string    `json:"content"`
}
