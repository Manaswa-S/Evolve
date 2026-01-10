package preprocessor

import (
	"math"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Empty if first Revision, will still compare
var localLastRevTxtCache = ""

func (s *Preprocessor) analyzeDiff(rc *RevisionCtx, r *RevisionClean) error {
	dmp := diffmatchpatch.New()

	diffs := dmp.DiffMain(localLastRevTxtCache, r.Content, false)

	// dmp.DiffCleanupSemantic(diffs)
	// dmp.DiffCleanupEfficiency(diffs)

	// >>>

	oldWords := strings.Fields(localLastRevTxtCache)
	newWords := strings.Fields(r.Content)

	oldJoined := strings.Join(oldWords, "\n")
	newJoined := strings.Join(newWords, "\n")

	diffs = dmp.DiffMain(oldJoined, newJoined, false)

	countWords := func(s string) int {
		if s == "" {
			return 0
		}
		return strings.Count(s, "\n") + 1
	}

	var inserted, deleted, unchanged float64
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			inserted += float64(countWords(d.Text))
		case diffmatchpatch.DiffDelete:
			deleted += float64(countWords(d.Text))
		case diffmatchpatch.DiffEqual:
			unchanged += float64(countWords(d.Text))
		}
	}

	rc.Diffs.Inserted = int(inserted)
	rc.Diffs.Deleted = int(deleted)
	rc.Diffs.Unchanged = int(unchanged)

	// set the last as cache
	localLastRevTxtCache = r.Content

	// >>>

	editScore := editDistanceScore(inserted, deleted, unchanged)
	semanticScore := semanticChangeScore(inserted, deleted, unchanged)

	rc.Diffs.EditDistanceScore = int(editScore * 100)
	rc.Diffs.SemanticChangeScore = int(semanticScore * 100)
	rc.Diffs.FinalScore = int(finalScore(semanticScore, editScore) * 100)

	// >>>

	editType := "unknown"
	change := changeScore(inserted, deleted, unchanged)
	balance := balanceScore(inserted, deleted, unchanged)

	switch {
	case change < 0.05:
		editType = "formatting"
	case change < 0.25 && balance > 0.7:
		if inserted > deleted {
			editType = "expansion"
		} else {
			editType = "cleanup"
		}
	case change >= 0.25 && balance < 0.4:
		editType = "rewrite"
	case change >= 0.15:
		editType = "mixed"
	default:
		editType = "minor"
	}

	rc.Diffs.ChangeScore = int(change * 100)
	rc.Diffs.BalanceScore = int(balance * 100)
	rc.Diffs.TypeOfEdit = editType

	return nil
}

func symmetricChangeScore(i, d, u float64) float64 {
	return (i + d) / (u + i + d)
}

// editDistanceScore
// Treats insertions and deletions equally
// Strongly rewards unchanged text
// Models: “How disruptive was this edit relative to what stayed the same?”
// large U suppresses score heavily
func editDistanceScore(i, d, u float64) float64 {
	return (i + d) / ((2 * u) + i + d)
}

// semanticChangeScore
// Insertions and deletions are NOT equal
// You decide what matters more
// Models: “How much new intent did this edit introduce?”
// “Adding knowledge is more meaningful than removing text.”
func semanticChangeScore(i, d, u float64) float64 {
	alpha := 1.0
	beta := 0.7
	return ((alpha * i) + (beta * d)) / (u + (alpha * i) + (beta * d))
}

func logScaledScore(i, d, u float64) float64 {
	factor := 1.0
	r := math.Log(float64(1+factor*(i+d))) / math.Log(float64(1+(factor*(u+d))))
	return r
}

func finalScore(semantic, edit float64) float64 {
	// NOTE: maintain wSemantic + wEdit = 1
	wSemantic := 0.7
	wEdit := 0.3
	return (wSemantic * semantic) + (wEdit * edit)
}

// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// Intuition:
// It measures how much the text changed overall, ignoring whether it was added or removed.
// Scale: 0 → nothing changed, 1 → everything changed.
// Example:
// 1000 words in the parent revision, you add 50 and delete 10 →
// changeRatio = (50+10)/(1000+50+10) ≈ 0.058 → tiny change → probably formatting/typo.
// If you add 400 and delete 300 →
// changeRatio = 700 / 1000+700 = 700 / 1700 ≈ 0.41 → major rewrite.
// So changeRatio = “how much of this page is different now?”
func changeScore(i, d, u float64) float64 {
	return (i + d) / (i + d + u)
}

// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// Intuition:
// It measures symmetry of the edit, i.e., is it mostly one-sided or balanced?
// Scale: 0 → perfectly balanced (insertions = deletions), 1 → completely one-sided.
// Example:
// Inserted 50, deleted 50 → balance = |50-50| / 100 = 0 → symmetric → you rewrote equal content → probably a “rewrite”.
// Inserted 100, deleted 10 → balance = |100-10| / 110 ≈ 0.82 → mostly additions → likely “expansion”.
// Inserted 10, deleted 100 → balance = 0.9 → mostly deletions → likely “cleanup”.
// So balance = “is this edit adding, removing, or mixing content?"
func balanceScore(i, d, u float64) float64 {
	if i+d == 0 {
		return 0
	} else {
		return math.Abs((i - d) / (i + d))
	}
}

// High changeRatio + low balance → lots of words changed, but roughly equal insert/delete → rewrite.
// Low changeRatio + any balance → small change → formatting/minor.
// High changeRatio + high balance → mostly one-sided → expansion or cleanup.
// Intermediate changeRatio → could be mixed edits → mixed.

// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// cache prev revision
// Batch in memory
// character level diff
// word level diffs, better than characters
// sentence level diffs, maybe

// changeRatio := float64(added+removed) / float64(len(oldText)+1)
// isMinor := changeRatio < 0.01
// isMajor := changeRatio > 0.1

// Detect type of edit (RULE GOLD)
// Based on diffs:
// Only small deletes/inserts → formatting
// Many inserts, few deletes → expansion
// Balanced inserts/deletes → rewrite
// Only deletes → pruning / cleanup

// editType := "unknown"

// switch {
// case added < 50 && removed < 50:
//     editType = "formatting"
// case added > 500 && removed < 100:
//     editType = "expansion"
// case added > 300 && removed > 300:
//     editType = "rewrite"
// case removed > 300 && added < 100:
//     editType = "cleanup"
// }

// var added, removed, unchanged int
// for _, d := range diffs {
// 	switch d.Type {
// 	case diffmatchpatch.DiffInsert:
// 		added += len(d.Text)
// 	case diffmatchpatch.DiffDelete:
// 		removed += len(d.Text)
// 	case diffmatchpatch.DiffEqual:
// 		unchanged += len(d.Text)
// 	}
// }
