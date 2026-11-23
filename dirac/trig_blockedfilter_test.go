package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBecomesBlockedByFilter: a become-blocked line with Filter2 (gorge
// Mode$ AttackerBlocked ValidBlocker$ <spec>, attackerBlockedCandidates)
// fires once for an attacker at least one of whose blockers matches the
// filter, however many match, and not at all when none does.
func TestBecomesBlockedByFilter(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact})
	artifact := kop.FilterID(len(tbl.Filters) - 1)
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	wolf, golem, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Wolf", Types: kop.TCreature, Power: 1, Toughness: 6,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Flags: kop.TfSelf, Filter2: artifact}, Prog: gain}}},
		kop.CardDef{Name: "Test Golem", Types: kop.TCreature | kop.TArtifact, Power: 0, Toughness: 6},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 0, Toughness: 6})
	run := func(blockers ...CardID) (pend int) {
		t.Helper()
		sc := newScenTable(t, tbl, Config{NoElide: true})
		w := sc.perm(wolf, 0)
		var blocks [][2]Obj
		for _, c := range blockers {
			blocks = append(blocks, [2]Obj{sc.perm(c, 1), w})
		}
		sc.passUntil(t, kop.StepAttackers)
		sc.do(t, AkAttack, TCode(w), scAny)
		sc.do(t, AkDone, scAny, scAny)
		sc.passUntil(t, kop.StepBlockers)
		for _, b := range blocks {
			sc.do(t, AkBlock, TCode(b[0]), TCode(b[1]))
		}
		n := int(sc.s.npend)
		sc.do(t, AkDone, scAny, scAny)
		if n == 0 {
			n = int(sc.s.npend)
		}
		return n
	}
	for _, c := range []struct {
		name     string
		blockers []CardID
		want     int
	}{
		{"an artifact blocks", []CardID{golem}, 1},
		{"an artifact among two blockers", []CardID{bear, golem}, 1},
		{"two artifacts block", []CardID{golem, golem}, 1},
		{"no artifact blocks", []CardID{bear}, 0},
	} {
		if got := run(c.blockers...); got != c.want {
			t.Errorf("%s: %d pending triggers, want %d", c.name, got, c.want)
		}
	}
}
