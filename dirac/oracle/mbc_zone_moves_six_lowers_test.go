package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCZoneMoves6Lowers plays the MBC zone-moves-6 spelling family's cards
// in a lockstep fixture built on their dr-mbc pool halves, one game per
// spelling:
//
//   - kw:LimitSearchLibrary: Aven Mindcensor. gorge playerkeywords.go reads
//     only the Shroud and Hexproof player grants, so the static's grant is
//     inert and kgen lowers the static to nothing; the card still plays.
//   - other:library position N of Self from Graveyard: Bookwurm. Its
//     {2}{G} ability (ActivationZone$ Graveyard) puts CARDNAME from its own
//     graveyard into its library third from the top (OpToLibrary
//     kop.LibThird); both decks are Bookwurm and Forests, so blocked or
//     traded 7/7s reach the graveyard and the activation is offered.
//   - param:ChangeZone.AttachedTo: Boonweaver Giant. Its enters trigger
//     searches the library, graveyard and hand for an Aura and puts it onto
//     the battlefield attached to the Giant (AttachedTo$ Self, gorge
//     markChangeZoneAttach and the entry gate). Its Aura is Canopy Cover,
//     chosen because its SynthNameHash shares the Giant's low four bits, so
//     both land in one dr-mbc split under the 4-way and the 16-way cut.
//
// The cleave spelling param:ChangeZone.NonBasicSpell has no playable card of
// its own: Alchemist's Retrieval's cleave body lowers its NonBasicSpell$
// marker but the card stays refused by the spell-cost family (its cleave
// Cost$ differs from the printed mana cost, a separate family), so the
// fixture cannot deal it.
//
// Each deck's cards must lower: cardset's matchup refuses a deck with a
// refused card, so reverting the lowering fails these games at the deal.
func TestMBCZoneMoves6Lowers(t *testing.T) {
	aven := []cardset.Entry{
		{Name: "Aven Mindcensor", Count: 4}, {Name: "Plains", Count: 18}, {Name: "Island", Count: 18},
	}
	avenB := []cardset.Entry{
		{Name: "Aven Mindcensor", Count: 2}, {Name: "Plains", Count: 19}, {Name: "Forest", Count: 19},
	}
	bookwurm := []cardset.Entry{
		{Name: "Bookwurm", Count: 4}, {Name: "Forest", Count: 36},
	}
	bookwurmB := []cardset.Entry{
		{Name: "Bookwurm", Count: 3}, {Name: "Forest", Count: 37},
	}
	boonweaver := []cardset.Entry{
		{Name: "Boonweaver Giant", Count: 4}, {Name: "Canopy Cover", Count: 4}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 16},
	}
	boonweaverB := []cardset.Entry{
		{Name: "Boonweaver Giant", Count: 2}, {Name: "Canopy Cover", Count: 2}, {Name: "Plains", Count: 18}, {Name: "Forest", Count: 18},
	}
	t.Run("LimitSearchLibrary", func(t *testing.T) {
		n := mbcZM6Lockstep(t, "Aven Mindcensor", [2][]cardset.Entry{aven, avenB}, 16, func(g *GorgeSide, ev events.Event) bool {
			return ev.Kind == events.MoveZone && ev.To == state.ZBattlefield && mbcZM6Named(g, ev.Obj, "Aven Mindcensor")
		})
		if n == 0 {
			t.Errorf("no game put Aven Mindcensor onto the battlefield")
		}
	})
	t.Run("LibraryPosition2Graveyard", func(t *testing.T) {
		n := mbcZM6Lockstep(t, "Bookwurm", [2][]cardset.Entry{bookwurm, bookwurmB}, 16, func(g *GorgeSide, ev events.Event) bool {
			return ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZLibrary && mbcZM6Named(g, ev.Obj, "Bookwurm")
		})
		if n == 0 {
			t.Errorf("no game activated Bookwurm from the graveyard into the library")
		}
	})
	t.Run("SearchAttachedTo", func(t *testing.T) {
		giant, aura := cardset.SynthNameHash("Boonweaver Giant"), cardset.SynthNameHash("Canopy Cover")
		if giant&0xF != aura&0xF {
			t.Fatalf("Canopy Cover hash %#x and Boonweaver Giant hash %#x differ in the low four bits: pick an Aura that shares the Giant's split", aura, giant)
		}
		n := mbcZM6Lockstep(t, "Boonweaver Giant", [2][]cardset.Entry{boonweaver, boonweaverB}, 16, func(g *GorgeSide, ev events.Event) bool {
			return ev.Kind == events.Attach && len(ev.IDs) == 1 && mbcZM6Named(g, ev.IDs[0], "Boonweaver Giant")
		})
		if n == 0 {
			t.Errorf("no game attached a found Aura to Boonweaver Giant")
		}
	})
}

// mbcZM6Named reports whether object id's front face is name.
func mbcZM6Named(g *GorgeSide, id state.ObjID, name string) bool {
	o := g.E.G.Obj(id)
	return o != nil && o.Face() != nil && o.Face().Name == name
}

// mbcZM6Lockstep plays seeds lockstep games of decks on the dr-mbc-* split that
// holds carrier, gorge leading, failing on the first divergence, and returns how
// many of gorge's events across the games witness(g, ev) accepts: the proof
// the games reached the card's path, not only that the cards dealt.
func mbcZM6Lockstep(t *testing.T, carrier string, decks [2][]cardset.Entry, seeds uint64, witness func(g *GorgeSide, ev events.Event) bool) int {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, mbcZM6SplitFor(t, carrier, decks))
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	hits := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		finish := func(g *GorgeSide) {
			for _, ev := range g.E.L.Events {
				if witness(g, ev) {
					hits++
				}
			}
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Finish: finish})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
	}
	return hits
}

// mbcZM6SplitFor is the dr-mbc-* synthetic pool that holds the carrier card
// and deals both decks. The splits are hash-cut and sized by what lowers, so
// no pool is named here: the pools are listed, the one holding the carrier
// found, and every other card of the decks asserted to be in it and not
// refused. The test fails when no pool holds the carrier or a deck card is
// missing from it.
func mbcZM6SplitFor(t *testing.T, carrier string, decks [2][]cardset.Entry) string {
	t.Helper()
	var tried []string
	for _, name := range cardsettest.SynthNames(t) {
		if !strings.HasPrefix(name, "dr-mbc-") {
			continue
		}
		set := cardsettest.Synth(t, name)
		i, ok := set.Lookup(carrier)
		if !ok || set.Status(i) == cardset.StatusRefused {
			tried = append(tried, name)
			continue
		}
		for _, d := range decks {
			for _, e := range d {
				j, ok := set.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s holds %s but not %s", name, carrier, e.Name)
				}
				if set.Status(j) == cardset.StatusRefused {
					t.Fatalf("%s holds %s but %s is refused there", name, carrier, e.Name)
				}
			}
		}
		return name
	}
	t.Fatalf("no dr-mbc-* pool holds %s unrefused (pools tried: %v)", carrier, tried)
	return ""
}
