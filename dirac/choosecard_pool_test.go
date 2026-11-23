package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// chooseCardScen builds a resolving spell of seat 0 whose program is one
// ChooseCard over the creature cards of zone z matching ctrl, asked of the
// chooser ref, with C the instruction's mode (1 is "up to one").
func chooseCardScen(t *testing.T, z kop.Zone, ctrl kop.CtrlSel, chooser kop.Ref, c int16) (*scen, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 16}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, cd := range da {
		id[tbl.Cards[cd].Name] = cd
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: ctrl, Zone: z})
	f := kop.FilterID(len(tbl.Filters) - 1)
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Choose Pool", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: []kop.Instr{
			{Op: kop.OpChooseCard, Obj: chooser, F: f, C: c},
		}}}})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	srcObj := sc.perm(src, 0)
	e := stackEntry{kind: seTrigger, obj: srcObj, srcVer: s.objs[srcObj].ver, card: src, ab: 0, ctrl: 0, trigObj: NoObj}
	s.sp = 1
	s.stack[0] = e
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	return sc, id["Savannah Lions"]
}

// chooseOptions steps the scenario to its ChooseCard ask and returns the
// asked seat and the offered options.
func chooseOptions(t *testing.T, sc *scen) (uint8, []Action) {
	t.Helper()
	sc.s.runProg(sc.k)
	if sc.s.res.ask != raGenChooseCard {
		t.Fatalf("ask %d, want the ChooseCard ask %d", sc.s.res.ask, raGenChooseCard)
	}
	asked := sc.s.res.askP
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenChooseCard {
		t.Fatalf("decision kind %d sub %d, want the ChooseCard choice", d.Kind, d.Sub)
	}
	return asked, append([]Action(nil), d.Options[:d.N]...)
}

// TestChooseCardOptionalNone: an optional ChooseCard (C=1) over the cards of
// a zone other than the battlefield offers each matching card and a Done
// (gorge choiceBounds: no Mandatory$ is Min 0, Max 1); Done records no
// chosen card, a pick records exactly that card, and a single candidate is
// still asked (the pick is the player's, never forced).
func TestChooseCardOptionalNone(t *testing.T) {
	for _, z := range []kop.Zone{kop.ZoneHand, kop.ZoneGraveyard} {
		put := func(sc *scen, lions CardID, n int) []Obj {
			var out []Obj
			for i := 0; i < n; i++ {
				if z == kop.ZoneHand {
					out = append(out, sc.hand(lions, 0))
				} else {
					out = append(out, sc.grave(lions, 0, 1)...)
				}
			}
			return out
		}
		for _, n := range []int{1, 2} {
			sc, lions := chooseCardScen(t, z, kop.CtrlYou, kop.RefController, 1)
			cards := put(sc, lions, n)
			asked, opts := chooseOptions(t, sc)
			picks, dones := 0, 0
			for _, a := range opts {
				switch a.Kind() {
				case AkChoose:
					picks++
				case AkDone:
					dones++
				}
			}
			if asked != 0 || picks != n || dones != 1 {
				t.Fatalf("zone %d, %d cards: asked %d, %d picks, %d dones, want seat 0, %d picks, one done", z, n, asked, picks, dones, n)
			}
			// Done chooses none: the precondition is that a card was on offer
			sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
			if sc.s.g.nchosen != 0 {
				t.Fatalf("zone %d, %d cards: Done left %d chosen", z, n, sc.s.g.nchosen)
			}

			// the same ask, answered with the first card, chooses that card
			sc, lions = chooseCardScen(t, z, kop.CtrlYou, kop.RefController, 1)
			cards = put(sc, lions, n)
			_, opts = chooseOptions(t, sc)
			var pick Action
			for _, a := range opts {
				if a.Kind() == AkChoose && Obj(a.Source()) == cards[0] {
					pick = a
				}
			}
			if pick == 0 {
				t.Fatalf("zone %d: no option names card %d (%v)", z, cards[0], opts)
			}
			sc.s.Apply(pick)
			if sc.s.g.nchosen != 1 || sc.s.g.chosen[0] != cards[0] {
				t.Fatalf("zone %d: chose %d (first %d), want exactly card %d", z, sc.s.g.nchosen, sc.s.g.chosen[0], cards[0])
			}
		}
	}

	// nothing in the pool: nothing is asked
	sc, _ := chooseCardScen(t, kop.ZoneHand, kop.CtrlYou, kop.RefController, 1)
	sc.s.runProg(sc.k)
	if sc.s.res.ask == raGenChooseCard {
		t.Fatalf("an empty pool asked")
	}
}

// TestChooseCardPoolBaseOpponent: the chosen-opponent chooser (RefEachOpponent)
// answers from its own seat, and its pool is matched from the resolving
// controller (gorge cardChoices' activator perspective), so OppCtrl is the
// opponent's own cards (ControlledByPlayer$ Chooser, kgen's OppCtrl) and
// YouCtrl is the controller's, asked of the opponent (Demonic Hordes).
func TestChooseCardPoolBaseOpponent(t *testing.T) {
	cases := []struct {
		ctrl     kop.CtrlSel
		own, opp int // seat 0's and seat 1's graveyard cards
		wantSeat uint8
	}{
		{kop.CtrlOpp, 1, 2, 1}, // the opponent's cards
		{kop.CtrlYou, 2, 1, 0}, // the controller's cards, asked of the opponent
	}
	for _, c := range cases {
		sc, lions := chooseCardScen(t, kop.ZoneGraveyard, c.ctrl, kop.RefEachOpponent, 0)
		sc.grave(lions, 0, c.own)
		sc.grave(lions, 1, c.opp)
		asked, opts := chooseOptions(t, sc)
		if asked != 1 {
			t.Fatalf("ctrl %d: asked seat %d, want the opponent's seat 1", c.ctrl, asked)
		}
		want := c.own
		if c.wantSeat == 1 {
			want = c.opp
		}
		if len(opts) != want {
			t.Fatalf("ctrl %d: %d options, want %d (%v)", c.ctrl, len(opts), want, opts)
		}
		for _, a := range opts {
			if got := sc.s.objs[Obj(a.Source())].owner(); got != c.wantSeat {
				t.Fatalf("ctrl %d: option names a card of seat %d, want seat %d", c.ctrl, got, c.wantSeat)
			}
		}
	}
}
