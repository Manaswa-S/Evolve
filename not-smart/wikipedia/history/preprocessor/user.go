package preprocessor

import (
	"context"
	"evolve/debugger"
	"fmt"
)

/*
"*" : anon
user : registered user
autoconfirmed : old trustworthy account
confirmed : manually confirmed, rare, still trustworthy
bot : official bot flag
flood : temporary, mass edits, bot-like
bot-global : global bot
rollbacker : edits are reactive, can revert vandalism
patroller : quality controller
autoreviewer : their reviews are auto marked as reviewed
abusefilter : power user, maintains abuse filters
editor : trusted editor
templateeditor : can edit protected templates, mostly structural
sysop : admin, mostly moderation
bureaucrat : assigns user rights
checkuser : investigates sockpuppets
global-sysop : admin across wikis'
researcher : limited access for analytics
*/
type GroupTag string

const (
	// Automation
	TagBot            GroupTag = "bot"
	TagGlobalBot      GroupTag = "global-bot"
	TagFlood          GroupTag = "flood"
	TagTemplateEditor GroupTag = "templateeditor"

	// Human Content
	TagUser              GroupTag = "user"
	TagExtendedConfirmed GroupTag = "extendedconfirmed"
	TagAutoConfirmed     GroupTag = "autoconfirmed"
	TagConfirmed         GroupTag = "confirmed"
	TagReviewer          GroupTag = "reviewer"
	TagAutoReviewer      GroupTag = "autoreviewer"
	TagAbuseFilter       GroupTag = "abusefilter"
	TagEditor            GroupTag = "editor"
	TagResearcher        GroupTag = "researcher"
	TagTemp              GroupTag = "temp"
	TagTempAccViewer     GroupTag = "temporary-account-viewer"
	TagIPBlockExempty    GroupTag = "ipblock-exempt"

	// Structural
	TagSysOp       GroupTag = "sysop"
	TagGlobalSysOp GroupTag = "global-sysop"

	// Maintenance
	TagBureaucrat    GroupTag = "bureaucrat"
	TagCheckUser     GroupTag = "checkuser"
	TagRollBacker    GroupTag = "rollbacker"
	TagPatroller     GroupTag = "patroller"
	TagExtendedMover GroupTag = "extendedmover"
	TagFileMover     GroupTag = "filemover"

	TagAsterisk GroupTag = "*"
)

func reinforce(current, weight int) int {
	return current + int((100-current)*weight/100)
}

var GroupsMap = map[GroupTag]func(*RevisionAnalysis){
	TagBot: func(rc *RevisionAnalysis) {
		c := 100
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
		rc.Tags.IsBot = true
	},
	TagGlobalBot: func(rc *RevisionAnalysis) {
		c := 80
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
	},
	TagFlood: func(rc *RevisionAnalysis) {
		c := 60
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
	},
	TagTemplateEditor: func(rc *RevisionAnalysis) {
		c := 60
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
	},
	//
	TagUser: func(rc *RevisionAnalysis) {
		c := 85
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagExtendedConfirmed: func(rc *RevisionAnalysis) {
		c := 90
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagAutoConfirmed: func(rc *RevisionAnalysis) {
		c := 85
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagConfirmed: func(rc *RevisionAnalysis) {
		c := 85
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagReviewer: func(rc *RevisionAnalysis) {
		c := 80
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagAutoReviewer: func(rc *RevisionAnalysis) {
		c := 80
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagEditor: func(rc *RevisionAnalysis) {
		c := 70
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagResearcher: func(rc *RevisionAnalysis) {
		c := 60
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagAbuseFilter: func(rc *RevisionAnalysis) {
		c := 60
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagTemp: func(rc *RevisionAnalysis) {
		c := 60
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagTempAccViewer: func(rc *RevisionAnalysis) {
		c := 75
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagIPBlockExempty: func(rc *RevisionAnalysis) {
		c := 65
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	//
	TagSysOp: func(rc *RevisionAnalysis) {
		c := 80
		rc.Confidence.Structural = reinforce(rc.Confidence.Structural, c)
	},
	TagGlobalSysOp: func(rc *RevisionAnalysis) {
		c := 90
		rc.Confidence.Structural = reinforce(rc.Confidence.Structural, c)
	},
	//
	TagBureaucrat: func(rc *RevisionAnalysis) {
		c := 80
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagCheckUser: func(rc *RevisionAnalysis) {
		c := 80
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagRollBacker: func(rc *RevisionAnalysis) {
		c := 80
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagPatroller: func(rc *RevisionAnalysis) {
		c := 70
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagExtendedMover: func(rc *RevisionAnalysis) {
		c := 65
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagFileMover: func(rc *RevisionAnalysis) {
		c := 50
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
}

var IgnoreGroupMap = map[GroupTag]bool{
	TagAsterisk: true,
}

type UserAnalyzer struct {
	// In       chan *RevisionMeta
	// Out      chan error
	ctx      context.Context
	metrics  *Metrics
	debugger *debugger.Debugger
}

func NewUserAnalyzer(commons *Commons) *UserAnalyzer {
	// func NewUserAnalyzer(commons *Commons, in chan *RevisionMeta, out chan error) *UserAnalyzer {
	return &UserAnalyzer{
		// In:       in,
		// Out:      out,
		ctx:      commons.ctx,
		metrics:  commons.metrics,
		debugger: commons.debugger,
	}
}

func (s *UserAnalyzer) analyzeUser(ra *RevisionAnalysis) error {
	for _, flag := range ra.Process.User.Groups {
		if set, ok := GroupsMap[GroupTag(flag)]; ok {
			set(ra)
		} else {
			if exempt, ok := IgnoreGroupMap[GroupTag(flag)]; ok {
				if !exempt {
					s.debugger.Debug(fmt.Sprintf("'%s' not in groups map and unexempted", flag))
				}
			} else {
				s.debugger.Debug(fmt.Sprintf("'%s' not in groups map and ignore map", flag))
			}
		}
	}

	return nil
}
