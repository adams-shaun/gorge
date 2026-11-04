package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Kai Winn, Faithless Leader lockstep: "as Kai Winn enters, choose a
// nonland card name; whenever an opponent casts a spell with the chosen name
// or activates an ability of a source with the chosen name, that player loses
// 3 life" (ETB NameCard, Mode$ SpellAbilityCast; kernel ExtChooseName and a
// SpellCast line with Tf4NamedCast, dirac/repl_choosename.go). Both decks
// hold Kai Winn, so each seat is the other's opponent. Spells with the
// chosen name come from Shock, Doom Blade, Divination, Reverberate and the
// creatures; abilities of a named source from Millstone, Prodigal Sorcerer
// and Vampire Neonate. Reverberate copies a named instant, and a copy is
// not cast. gorge's name ask is limited to the matchup's own cards, the list
// the kernel offers: NewMatchup sets Matchup.NameUniverse for a matchup holding
// a name chooser, so Matchup.Config and kfuzz's gorge host agree by default.

var kkaiwinnDecks = [2][]cardset.Entry{
	// B/R: Kai Winn, burn and removal, the spell copier, Millstone
	{
		{Name: "Kai Winn, Faithless Leader", Count: 4}, {Name: "Millstone", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Doom Blade", Count: 3}, {Name: "Reverberate", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 3},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 8},
	},
	// U/B: Kai Winn, card draw, creatures with activated abilities
	{
		{Name: "Kai Winn, Faithless Leader", Count: 4}, {Name: "Millstone", Count: 2},
		{Name: "Divination", Count: 4}, {Name: "Doom Blade", Count: 3},
		{Name: "Vampire Neonate", Count: 4}, {Name: "Prodigal Sorcerer", Count: 3}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Island", Count: 8},
	},
}

// kkaiwinnPolicy names a card of the opponent's deck whenever it can (so the
// named cards actually get cast and activated), casts and activates before
// anything else, and otherwise plays randomly.
func kkaiwinnPolicy(seed uint64, oppNames [2]map[uint16]bool) Policy {
	rnd := RandomPolicy(seed)
	nameKind := ActChoose + CanonKind(mustChooseCode("name"))
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch k := a.Kind(); {
			case k == nameKind:
				if oppNames[c.D.Player&1][uint16(a.Tgt())<<8|uint16(a.Mode())] {
					buf = append(buf, a)
				}
			case k == ActCast || k == ActCastFace || k == ActActivate || k == ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func mustChooseCode(kind string) uint8 {
	c, ok := canon.ChooseCode(kind)
	if !ok {
		panic("no canonical choose kind " + kind)
	}
	return c
}

type kkaiwinnRun struct {
	games, divs, stalls int
	names               int // games in which a Kai Winn named a card
}

func runKaiwinn(t *testing.T, seeds int, st *dirac.NamedCastStats) kkaiwinnRun {
	t.Helper()
	reg, set, decks, tbl := p3Setup(t, "kaiwinn", kkaiwinnDecks)
	if len(tbl.NameCards) == 0 {
		t.Fatal("precondition: the matchup table lists no nameable cards")
	}
	var oppNames [2]map[uint16]bool
	for s := range decks {
		oppNames[s] = map[uint16]bool{}
	}
	for s := range decks {
		for _, c := range decks[s] {
			if !c.Faces[0].IsLand() {
				oppNames[1-s][canon.NameValue(cardName(c))] = true
			}
		}
	}
	old := dirac.DebugNamedCastStats
	dirac.DebugNamedCastStats = st
	defer func() { dirac.DebugNamedCastStats = old }()
	var run kkaiwinnRun
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kaiwinn-a", "kaiwinn-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		if m.NameUniverse == nil { // the harness default: the matchup's own cards
			t.Fatal("precondition: NewMatchup left a name chooser's matchup on the registry universe")
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kkaiwinnPolicy(seed, oppNames), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		for _, o := range g.E.G.Objs {
			if o.ChosenName != "" {
				run.names++
				break
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			t.Logf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return run
}

func TestKaiWinnLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.NamedCastStats
	run := runKaiwinn(t, 256, &st)
	t.Logf("%d games, %d divergences, %d stalls, %d games where a name was chosen, %d spell-cast and %d activation firings",
		run.games, run.divs, run.stalls, run.names, st.SpellFires.Load(), st.ActFires.Load())
	if run.divs != 0 || run.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", run.divs, run.stalls)
	}
	if n := st.SpellFires.Load(); n < 20 {
		t.Errorf("only %d firings on a cast of the named spell (need 20)", n)
	}
	if n := st.ActFires.Load(); n < 10 {
		t.Errorf("only %d firings on an activation of a named source (need 10)", n)
	}
}

// TestKaiWinnMutation: with the activation half dropped, or with every
// opponent spell matching instead of the named one, the games diverge.
func TestKaiWinnMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mut := range []struct {
		name string
		flag *bool
	}{
		{"ignore activations", &dirac.DebugNamedCastNoAct},
		{"match any opponent spell", &dirac.DebugNamedCastAnySpell},
	} {
		*mut.flag = true
		run := runKaiwinn(t, 128, nil)
		*mut.flag = false
		if run.divs == 0 {
			t.Fatalf("%s: no divergence in %d games", mut.name, run.games)
		}
		t.Logf("%s: diverged in %d of %d games", mut.name, run.divs, run.games)
	}
}
