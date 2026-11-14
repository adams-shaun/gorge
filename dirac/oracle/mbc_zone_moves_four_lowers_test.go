package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The MBC zone-moves-4 lowering: The Muckslinger Gang's "Creature spells you
// control and creature cards you own in any zone other than the battlefield or
// the stack get +3/+3" (S:Mode$ Continuous | AffectedZone$ All) lowers to
// kop.StatOffZonePT, a marker the kernel's off-battlefield P/T readers stall
// on while it is in force (dirac/static_offzonept.go). Two pools:
//
//   - mbcmuckgang: the Gang with no off-battlefield P/T reader beside it. Its
//     battlefield anthem (S[0], not attacking or blocking) plays against burn
//     and blockers in lockstep, and the marker never fires.
//   - mbcmuckgwenna: Gwenna, Eyes of Gaea's "creature spell with power 5 or
//     greater" cast trigger beside the Gang. gorge reads the boosted power of
//     the spell on the stack (a Grizzly Bears spell is a 5/5 there), so with
//     the marker in force some game stalls offzonept and none diverges; with
//     the marker dropped from the table some game diverges (the pool reaches
//     the read the marker guards).
func TestMBCZoneMoves4Lowers(t *testing.T) {
	t.Parallel()
	_ = corpusDirOrSkip(t)
	gangDecks := [2][]cardset.Entry{
		{
			{Name: "The Muckslinger Gang", Count: 10}, {Name: "Grizzly Bears", Count: 10},
			{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Raging Goblin", Count: 12}, {Name: "Shock", Count: 6}, {Name: "Mountain", Count: 22},
		},
	}
	gwennaDecks := [2][]cardset.Entry{
		{
			{Name: "The Muckslinger Gang", Count: 8}, {Name: "Gwenna, Eyes of Gaea", Count: 6}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Raging Goblin", Count: 12}, {Name: "Shock", Count: 6}, {Name: "Mountain", Count: 22},
		},
	}
	t.Run("mbcmuckgang", func(t *testing.T) {
		t.Parallel()
		tbl := mbcZoneMoves4Table(t, "mbcmuckgang", gangDecks)
		if mbcOffZonePT(tbl) == nil {
			t.Fatal("The Muckslinger Gang has no StatOffZonePT static: the AffectedZone$ All grant did not lower")
		}
		if !mbcCardIs(tbl, "Grizzly Bears", kop.TCreature) {
			t.Fatal("Grizzly Bears is not a creature in the table")
		}
		lockstepSet(t, "mbcmuckgang", gangDecks, 200, "KMBCZONEMOVES4_SEEDS")
	})
	t.Run("mbcmuckgwenna", func(t *testing.T) {
		t.Parallel()
		tbl := mbcZoneMoves4Table(t, "mbcmuckgwenna", gwennaDecks)
		if mbcOffZonePT(tbl) == nil {
			t.Fatal("The Muckslinger Gang has no StatOffZonePT static: the AffectedZone$ All grant did not lower")
		}
		if mbcCard(tbl, "Gwenna, Eyes of Gaea") == nil {
			t.Fatal("Gwenna, Eyes of Gaea is not in the mbcmuckgwenna table: the pool holds no off-battlefield P/T reader")
		}
		reg, set, gd := lockstepPool(t, "mbcmuckgwenna", gwennaDecks)
		var stalled, div int
		for seed := uint64(1); seed <= 120; seed++ {
			res := mbcZoneMoves4Game(t, reg, set, gd, tbl, seed)
			if res.Div != nil {
				div++
				t.Errorf("seed %d: diverged at decision %d with the marker in force: %s %s", seed, res.Div.At, res.Div.What, res.Div.Detail)
			}
			if res.Stall == KernelStallPrefix+dirac.StallName(dirac.StallOffZonePT) {
				stalled++
			}
		}
		if stalled == 0 {
			t.Fatal("no seed stalled offzonept: the pool never reads a boosted spell's power")
		}
		t.Logf("mbcmuckgwenna: %d of 120 seeds stalled offzonept, %d diverged", stalled, div)
		// the marker dropped: the kernel reads the printed power where gorge
		// reads the boosted one
		mut, _, _, err := set.Matchup(gwennaDecks[0], gwennaDecks[1])
		if err != nil {
			t.Fatal(err)
		}
		mbcOffZonePT(mut).Static.Kind = kop.StatNone
		for seed := uint64(1); seed <= 120; seed++ {
			if res := mbcZoneMoves4Game(t, reg, set, gd, mut, seed); res.Div != nil {
				return
			}
		}
		t.Fatal("no seed diverged with the StatOffZonePT marker dropped: the pool never reaches the read it guards")
	})
}

func mbcZoneMoves4Table(t *testing.T, pool string, decks [2][]cardset.Entry) *kop.Table {
	t.Helper()
	set, err := cardset.RawByName(pool)
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skip("no kernel corpus table: " + err.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

// mbcZoneMoves4Game plays one lockstep game of the pool on table tbl.
func mbcZoneMoves4Game(t *testing.T, reg *cards.Registry, set cardset.Set, gd [2][]*cards.Card, tbl *kop.Table, seed uint64) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"mbcmuckgwenna-a", "mbcmuckgwenna-b"}, gd, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := castFirstPolicy(seed)
	if seed%8 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
	CloseSide(k)
	return res
}

// mbcOffZonePT is The Muckslinger Gang's StatOffZonePT static.
func mbcOffZonePT(tbl *kop.Table) *kop.Ability {
	cd := mbcCard(tbl, "The Muckslinger Gang")
	if cd == nil {
		return nil
	}
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOffZonePT {
			return a
		}
	}
	return nil
}
