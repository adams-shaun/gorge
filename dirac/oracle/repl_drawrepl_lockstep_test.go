package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ReplDrawProgram lockstep pool: the Draw replacements whose
// ReplaceWith$ body is a draw, a life change or a token (Blood Scrivener,
// Hullbreacher, Notion Thief) and the bodyless Prevent$ skip (Possessed
// Portal). Each pair puts the one replacement on seat 0 and plays a vanilla
// seat 1, so one draw never meets two replacements (the kernel stalls a
// competition instead of picking the CR 616.1 order).
var kdrawReplDecks = map[string][2][]cardset.Entry{
	"scrivener": {
		{{Name: "Blood Scrivener", Count: 1}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Divination", Count: 4}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 11}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}},
	},
	"thief": {
		{{Name: "Notion Thief", Count: 1}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Divination", Count: 4}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 11}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}},
	},
	"hullbreacher": {
		{{Name: "Hullbreacher", Count: 1}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 12}, {Name: "Forest", Count: 11}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}},
	},
	"portal": {
		{{Name: "Possessed Portal", Count: 1}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Island", Count: 12}, {Name: "Swamp", Count: 15}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}},
	},
}

func TestReplDrawReplLockstep(t *testing.T) {
	t.Parallel()
	for name, decks := range kdrawReplDecks {
		name, decks := name, decks
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lockstepSet(t, "draw-repl", decks, 400, "KDRAWREPL_"+name+"_SEEDS")
		})
	}
}

// TestReplDrawReplLockstepMutation: dropping every program body for a plain
// skip (the draw replaced with nothing) diverges a seed, so the pool reads the
// body, not just the match.
func TestReplDrawReplLockstepMutation(t *testing.T) {
	t.Parallel()
	skip := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				a := &tbl.Cards[i].Abilities[j]
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDrawProgram && a.Repl.C&kop.ReplDrawSkip == 0 {
					a.Repl.C |= kop.ReplDrawSkip
					a.Prog = nil
				}
			}
		}
	}
	if n := lockstepRun(t, "draw-repl", kdrawReplDecks["thief"], 200, false, skip); n == 0 {
		t.Fatal("dropping the draw program's body diverged no seed")
	}
}
