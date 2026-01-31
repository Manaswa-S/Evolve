package scraper

import "time"

// >>>>>

type DebugWarningsMain struct {
	DebugWildcard string `json:"*"`
}
type DebugWarningsJSON struct {
	DebugWildcard string `json:"*"`
}
type DebugWarnings struct {
	JSON DebugWarningsJSON `json:"json"`
	Main DebugWarningsMain `json:"main"`
}
type DebugError struct {
	Code          string `json:"code"`
	Info          string `json:"info"`
	DebugWildcard string `json:"*"`
}

// Common Debug Info
type Debug struct {
	Warnings DebugWarnings `json:"warnings"`
	Error    *DebugError   `json:"error"`
	Servedby string        `json:"servedby"`
}

// >>>>>
type ResolveRedirect struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ResolvePage struct {
	PageID    int    `json:"pageid"`
	NameSpace int    `json:"ns"`
	Title     string `json:"title"`
}

type ResolveQuery struct {
	Redirects []*ResolveRedirect `json:"redirects"`
	Pages     []*ResolvePage     `json:"pages"`
}

// Resolve Title
type ResolveResponse struct {
	*Debug
	BatchComplete bool          `json:"batchcomplete"`
	Query         *ResolveQuery `json:"query"`
}

// >>>>>

type RevisionMeta struct {
	RevID     int       `json:"revid"`
	ParentID  int       `json:"parentid"`
	TimeStamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
	User      string    `json:"user"`
	UserID    int       `json:"userid"`
	Comment   string    `json:"comment"`
}

type RevisionIndexPage struct {
	PageID    int             `json:"pageid"`
	NameSpace int             `json:"ns"`
	Title     string          `json:"title"`
	Revisions []*RevisionMeta `json:"revisions"`
}

type RevisionIndexNormalized struct {
	FromEncoded bool   `json:"fromencoded"`
	From        string `json:"from"`
	To          string `json:"to"`
}

type RevisionIndexContinue struct {
	RvContinue string `json:"rvcontinue"`
	Continue   string `json:"continue"`
}

type RevisionIndexQuery struct {
	Normalized []*RevisionIndexNormalized `json:"normalized"`
	Pages      []*RevisionIndexPage       `json:"pages"`
}

// Revisions IDs' Page
type RevisionIndex struct {
	*Debug
	Continue *RevisionIndexContinue `json:"continue"`
	Query    *RevisionIndexQuery    `json:"query"`
}

// >>>>>

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

type RevisionsContentPage struct {
	PageID    int                `json:"pageid"`
	NameSpace int                `json:"ns"`
	Title     string             `json:"title"`
	Revisions []*RevisionContent `json:"revisions"`
}

type RevisionsContentQuery struct {
	Pages []*RevisionsContentPage `json:"pages"`
}

type RevisionsContentBatch struct {
	BatchComplete bool                   `json:"batchcomplete"`
	Query         *RevisionsContentQuery `json:"query"`
}
