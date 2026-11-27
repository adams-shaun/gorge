package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ReplDrawProgram lockstep pool: the Draw replacements whose
// ReplaceWith$ body is a draw, a life change, a token or a win (Blood Scrivener,
// Hullbreacher, Notion Thief, Laboratory Maniac, Jace), Out of the Tombs'
// graveyard return with its Remembered-gated LosesGame, and the Optional$ True
// skip (Obstinate Familiar, whose election both engines pose at each draw). The bodyless Prevent$ skip (Possessed Portal)
// has no deck here: no pair reached its skip (the mutation never diverged in
// 200 seeds), so a pair would be vacuous evidence. Each pair puts the one replacement on
// seat 0 and plays a vanilla seat 1, so one draw never meets two replacements (the kernel stalls a
// competition instead of picking the CR 616.1 order).
var kdrawReplDecks = map[string][2][]cardset.Entry{
	"scrivener": {
		// A hand-emptying spell reaches Hellbent, where the body applies; one
		// copy in a deck that draws cards never did. A second Scrivener would
		// also match the body's draws, a competition the kernel stalls.
		{{Name: "Blood Scrivener", Count: 1}, {Name: "One with Nothing", Count: 4}, {Name: "Dark Ritual", Count: 4},
			{Name: "Swamp", Count: 31}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}},
	},
	"thief": {
		{{Name: "Notion Thief", Count: 1}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Divination", Count: 4}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 11}},
		// Revitalize's Draw is its second instruction: the replacement body
		// runs after the spell's GainLife, so the outer restore keeps the
		// spell from repeating (or skipping) that instruction.
		{{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Revitalize", Count: 4},
			{Name: "Island", Count: 8}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 4}},
	},
	"familiar": {
		// Obstinate Familiar's Optional$ True skip: every draw its controller
		// makes once it resolves (the turn draw and each Divination card)
		// poses the apply / decline election (dirac/repl_drawmay.go). One copy:
		// two Familiars would be a competition the kernel stalls.
		{{Name: "Obstinate Familiar", Count: 1}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Divination", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 7}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}},
	},
	// "You win the game instead" while the library is empty. Seat 0's deck
	// draws its library down (Divination; Jace's +1 mills two and draws one);
	// seat 1 is all lands and attacks no one, so the only way a game ends
	// before a turn cap is a library running out: the win when the carrier is
	// on the battlefield, the CR 704.5b loss when it is not. One carrier per
	// deck: two would be a competition the kernel poses as an order ask.
	"maniac": {
		{{Name: "Laboratory Maniac", Count: 1}, {Name: "Divination", Count: 8}, {Name: "Island", Count: 31}},
		{{Name: "Plains", Count: 40}},
	},
	"jace": {
		{{Name: "Jace, Wielder of Mysteries", Count: 1}, {Name: "Divination", Count: 8}, {Name: "Island", Count: 31}},
		{{Name: "Plains", Count: 40}},
	},
	"hullbreacher": {
		{{Name: "Hullbreacher", Count: 1}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 12}, {Name: "Forest", Count: 11}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}},
	},
	// Out of the Tombs' empty-library return. The upkeep trigger mills its
	// own controller, so a game that resolves the enchantment reaches an
	// empty library without another mill card; the two pairs separate the
	// branches by whether a creature card can have been milled.
	"tombs_returns": {
		{{Name: "Out of the Tombs", Count: 1}, {Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Swamp", Count: 19}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8}, {Name: "Swamp", Count: 20}},
	},
	"tombs_loses": {
		{{Name: "Out of the Tombs", Count: 1}, {Name: "Swamp", Count: 39}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8}, {Name: "Swamp", Count: 20}},
	},
	// Divination resolving into an empty library poses the return's hidden
	// pick mid-resolution (the queued akDrawPick): gorge keeps the spell on
	// the stack with its second draw pending while the pick is open, so this
	// pair compares the pick's pose and the outer Draw's continuation.
	"tombs_divination": {
		{{Name: "Out of the Tombs", Count: 1}, {Name: "Grizzly Bears", Count: 12},
			{Name: "Divination", Count: 8}, {Name: "Swamp", Count: 10}, {Name: "Island", Count: 9}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8}, {Name: "Swamp", Count: 20}},
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

// TestReplDrawReplLockstepMutation: each program body the pool carries is
// actually applied in the pool's games: dropping it for a plain skip (the draw
// replaced with nothing) diverges a seed. A deck whose replacement never
// applies would pass the untouched pool vacuously.
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
	for _, name := range []string{"scrivener", "thief", "hullbreacher", "maniac", "jace", "tombs_returns", "tombs_loses"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if n := lockstepRun(t, "draw-repl", kdrawReplDecks[name], 200, false, skip); n == 0 {
				t.Fatalf("%s: dropping the draw program's body diverged no seed (the deck never applies it)", name)
			}
		})
	}
}

// TestReplDrawReplBranches is the draw-repl pool's "both branches reached"
// check: the Out of the Tombs pairs must resolve the replacement into a
// battlefield move (tombs_returns, tombs_divination) and into the
// Remembered-gated loss (tombs_loses), not merely play beside the
// enchantment. An untouched pair
// passes while its branch never fires (the in-resolution pick hid a
// divergence exactly that way before tombs_divination existed), so the
// kernel counts each outcome (repl_drawpick.go's DrawPickStats) and the
// pair whose count is zero fails. It runs sequentially on purpose: the
// counters are process-wide, and Go resumes the parallel top-level tests
// only after the sequential ones have finished.
func TestReplDrawReplBranches(t *testing.T) {
	for _, tc := range []struct {
		name  string
		moved bool
		lost  bool
	}{
		{"tombs_returns", true, false},
		{"tombs_loses", false, true},
		{"tombs_divination", true, false},
	} {
		dirac.DrawPickStats.Moved.Store(0)
		dirac.DrawPickStats.Lost.Store(0)
		if n := lockstepRun(t, "draw-repl", kdrawReplDecks[tc.name], 200, false, nil); n != 0 {
			t.Fatalf("%s: %d of 200 seeds diverge", tc.name, n)
		}
		moved, lost := dirac.DrawPickStats.Moved.Load(), dirac.DrawPickStats.Lost.Load()
		if tc.moved && moved == 0 {
			t.Errorf("%s: the replacement moved no card to the battlefield in 200 seeds", tc.name)
		}
		if tc.lost && lost == 0 {
			t.Errorf("%s: the Remembered-gated LosesGame ran in none of 200 seeds", tc.name)
		}
		if !tc.moved && moved != 0 {
			// seat 0's deck holds no creature card (Out of the Tombs and
			// Swamps), so a move here means the pick's filter matched what it
			// must not.
			t.Errorf("%s: the replacement moved %d cards although seat 0 owns no creature", tc.name, moved)
		}
	}
}
