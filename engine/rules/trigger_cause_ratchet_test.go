package rules

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// causeQualifiers are the trigger parameters that name what CAUSED the event
// (CR 603.2). A trigger matcher that skips one fires on events the card text
// excludes, silently.
var causeQualifiers = []string{"ValidCause", "TargetRelativeToCause", "WhileKeyword", "SearchOwnLibrary"}

// causeQualifiersRead is every (Mode, qualifier) pair a trigmatch matcher
// reads. A corpus pair outside this table and the residue fails the ratchet,
// and so does an entry here that no corpus trigger carries.
var causeQualifiersRead = map[string]bool{
	"TurnFaceUp ValidCause":            true, // trigmatch.turnFaceUpMatches
	"DamageDone ValidCause":            true, // trigmatch.damageCauseMatches
	"DamageDone TargetRelativeToCause": true, // trigmatch.damageCauseMatches
	"Exiled WhileKeyword":              true, // trigmatch.whileKeywordAdmits
	"Sacrificed WhileKeyword":          true, // trigmatch.whileKeywordAdmits
	"SearchedLibrary SearchOwnLibrary": true, // trigmatch.searchedLibraryMatches
	"Discarded ValidCause":             true, // trigmatch.DiscardCauseAdmits
	"DiscardedAll ValidCause":          true, // trigmatch.DiscardCauseAdmits
	"Panharmonicon ValidCause":         true, // the Mode$ Panharmonicon static (statics_echo.go)
	"BecomesTargetOnce ValidCause":     true, // trigmatch.becomesTargetOnceMatches
	"LifeLost ValidCause":              true, // trigmatch.lifeLostMatches
}

// causeQualifiersResidue is the exact set of pairs no matcher reads yet, with
// the cards that carry them. Deleting an entry is how a follow-up proves it
// landed; an entry whose cards moved is stale.
var causeQualifiersResidue = map[string][]string{
	"ChangesZone ValidCause": {"Sacred Ground"},
	"ChangesZoneAll ValidCause": {"Deep Gnome Terramancer", "Hero of Bretagard",
		"Ranar the Ever-Watchful", "The Madcap Jester"},
	// Mode$ Countered has no matcher at all: these cards are loud-unsupported,
	// not over-triggering.
	"Countered ValidCause": {"Baral, Chief of Compliance", "Lullmage Mentor", "Summoning Trap"},
}

// TestTriggerCauseQualifiersAreRead walks every corpus trigger line (T: lines
// and Mode$ SVars) and requires each (Mode, cause qualifier) pair to be read
// by a matcher or named in the exact residue table.
func TestTriggerCauseQualifiersAreRead(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	got := map[string]map[string]bool{}
	note := func(mode string, params func(string) bool, card string) {
		for _, q := range causeQualifiers {
			if !params(q) {
				continue
			}
			k := mode + " " + q
			if got[k] == nil {
				got[k] = map[string]bool{}
			}
			got[k][card] = true
		}
	}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			name := strings.ReplaceAll(f.Name, "’", "'")
			for _, tr := range f.Triggers {
				note(tr.Mode, func(q string) bool { _, ok := tr.Params[q]; return ok }, name)
			}
			for _, body := range f.SVars {
				if !strings.HasPrefix(body, "Mode$ ") {
					continue
				}
				kv := map[string]string{}
				for part := range strings.SplitSeq(body, "|") {
					if k, v, ok := strings.Cut(strings.TrimSpace(part), "$"); ok {
						kv[k] = strings.TrimSpace(v)
					}
				}
				note(kv["Mode"], func(q string) bool { _, ok := kv[q]; return ok }, name)
			}
		}
	}
	if len(got) == 0 {
		t.Fatal("precondition: the corpus walk found no cause-qualifier trigger at all (is .cards linked?)")
	}
	cardsOf := func(pair string) []string {
		var out []string
		for c := range got[pair] {
			out = append(out, c)
		}
		sort.Strings(out)
		return out
	}
	for pair := range got {
		_, read := causeQualifiersRead[pair]
		want, resid := causeQualifiersResidue[pair]
		switch {
		case read && resid:
			t.Errorf("%q is both read and residue: delete the residue entry", pair)
		case !read && !resid:
			t.Errorf("%q (%s) is neither read by a trigmatch matcher nor named in the residue: "+
				"the trigger over-fires silently", pair, strings.Join(cardsOf(pair), "; "))
		case resid:
			wantSorted := slices.Sorted(slices.Values(want))
			if have := cardsOf(pair); !slices.Equal(have, wantSorted) {
				t.Errorf("stale residue %q: carriers are %v, table says %v", pair, have, wantSorted)
			}
		}
	}
	for pair := range causeQualifiersRead {
		if got[pair] == nil {
			t.Errorf("read table names %q but no corpus trigger carries it", pair)
		}
	}
	for pair := range causeQualifiersResidue {
		if got[pair] == nil {
			t.Errorf("stale residue %q: no corpus trigger carries it", pair)
		}
	}
}
