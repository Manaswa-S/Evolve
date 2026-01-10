package preprocessor

import "fmt"

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

var GroupsMap = map[GroupTag]func(*RevisionCtx){
	TagBot: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
	},
	TagGlobalBot: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
	},
	TagFlood: func(rc *RevisionCtx) {
		c := 60
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
	},
	TagTemplateEditor: func(rc *RevisionCtx) {
		c := 60
		rc.Confidence.Automation = reinforce(rc.Confidence.Automation, c)
	},
	//
	TagUser: func(rc *RevisionCtx) {
		c := 85
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagExtendedConfirmed: func(rc *RevisionCtx) {
		c := 90
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagAutoConfirmed: func(rc *RevisionCtx) {
		c := 85
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagConfirmed: func(rc *RevisionCtx) {
		c := 85
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagReviewer: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagAutoReviewer: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagEditor: func(rc *RevisionCtx) {
		c := 70
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagResearcher: func(rc *RevisionCtx) {
		c := 60
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagAbuseFilter: func(rc *RevisionCtx) {
		c := 60
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagTemp: func(rc *RevisionCtx) {
		c := 60
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagTempAccViewer: func(rc *RevisionCtx) {
		c := 75
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	TagIPBlockExempty: func(rc *RevisionCtx) {
		c := 65
		rc.Confidence.Human = reinforce(rc.Confidence.Human, c)
	},
	//
	TagSysOp: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Structural = reinforce(rc.Confidence.Structural, c)
	},
	TagGlobalSysOp: func(rc *RevisionCtx) {
		c := 90
		rc.Confidence.Structural = reinforce(rc.Confidence.Structural, c)
	},
	//
	TagBureaucrat: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagCheckUser: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagRollBacker: func(rc *RevisionCtx) {
		c := 80
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagPatroller: func(rc *RevisionCtx) {
		c := 70
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagExtendedMover: func(rc *RevisionCtx) {
		c := 65
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
	TagFileMover: func(rc *RevisionCtx) {
		c := 50
		rc.Confidence.Maintenance = reinforce(rc.Confidence.Maintenance, c)
	},
}

var IgnoreGroupMap = map[GroupTag]bool{
	TagAsterisk: true,
}

func (s *Preprocessor) analyzeUser(r *RevisionCtx) error {
	for _, flag := range r.Process.User.Groups {
		if set, ok := GroupsMap[GroupTag(flag)]; ok {
			set(r)
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
