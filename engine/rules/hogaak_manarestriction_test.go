package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Hogaak, Arisen Necropolis is the S:Mode$ ManaRestriction zero-spend shape
// (cards.ManaRestrictionZeroSpendShape): "You can't spend mana to cast this
// spell". Convoke's creature taps and Delve's graveyard exiles are not mana,
// so they alone pay the {5}{B/G}{B/G} -- the offer gate prices the cast
// against the empty pool (pay.Descriptor.NoMana), the announcement gates
// require the credits to cover the whole cost (one colour contribution per
// hybrid pip, the rest of the generic by taps and exiles), and no mana is
// ever charged. The mode's source-restricted carriers (Myr Superion,
// Imperiosaur) need per-source pool provenance state.Player does not track,
// so they stay unsupported (stat:ManaRestriction.OtherShape).

const (
	hogaakDroneB = "Name:Test Black Drone\nManaCost:B\nTypes:Creature\nPT:1/1\nOracle:x\n"
	hogaakDroneG = "Name:Test Green Drone\nManaCost:G\nTypes:Creature\nPT:1/1\nOracle:x\n"
	hogaakFodder = "Name:Test Fodder\nTypes:Sorcery\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ Life.\nOracle:x\n"
)

// hogaakBoard seeds seat 0 with Hogaak, black and green creature copies on
// the battlefield and fodder cards in the graveyard, and returns the engine,
// its replay config, Hogaak's id in hand and the convokable creature ids.
func hogaakBoard(t *testing.T, blacks, greens, fodder int) (*Engine, Config, state.ObjID, []state.ObjID) {
	t.Helper()
	hogaak := corpusCardNamed(t, "Hogaak, Arisen Necropolis")
	seat0 := []*cards.Card{hogaak}
	for range blacks {
		seat0 = append(seat0, card(t, hogaakDroneB))
	}
	for range greens {
		seat0 = append(seat0, card(t, hogaakDroneG))
	}
	for range fodder {
		seat0 = append(seat0, card(t, hogaakFodder))
	}
	e, cfg := tokenReplGame(t, 9701, seat0...)
	id := moveSeededCard(t, e, 0, hogaak, state.ZHand)
	creatures := make([]state.ObjID, 0, blacks+greens)
	for range blacks {
		creatures = append(creatures, putCreature(t, e, 0, hogaakDroneB))
	}
	for range greens {
		creatures = append(creatures, putCreature(t, e, 0, hogaakDroneG))
	}
	for range fodder {
		addToGraveyard(t, e, 0, hogaakFodder)
	}
	return e, cfg, id, creatures
}

// hogaakCastOption returns Hogaak's cast option at seat 0's re-posed priority
// decision, if offered.
func hogaakCastOption(t *testing.T, e *Engine, id state.ObjID) (decision.Option, bool) {
	t.Helper()
	ppPriority(t, e)
	for _, o := range castOptions(t, e) {
		if o.Obj == id {
			return o, true
		}
	}
	return decision.Option{}, false
}

// answerHogaakAsks drives the pending cast-flow asks: the convoke
// announcement (one colour contribution per hybrid pip colour, then
// genericWanted generic contributions, never the same creature twice), the
// pip faces (index 0, the first feasible face), and the delve exiles (the
// shortfall the Min forces). Stops when the pending decision is no longer a
// cast-flow choose.
func answerHogaakAsks(t *testing.T, e *Engine, genericWanted int) {
	t.Helper()
	answerHogaakAsksPlan(t, e, []byte{'B', 'G'}, genericWanted)
}

// answerHogaakAsksPlan drives the pending cast-flow asks with an explicit
// announcement plan: one convoke_<c> contribution per entry of colors (from
// distinct creatures), then genericWanted generic contributions, then the
// pip faces (index 0) and the delve exiles the Min forces. Each pip ask's
// offered option kinds are appended to the returned slice, so a test can pin
// which faces the payment offered and which it refused (an empty-menu
// defensive arm offers the first alternative whatever it covers -- a menu
// the feasibility gate did not build). Stops when the pending decision is no
// longer a cast-flow choose.
func answerHogaakAsksPlan(t *testing.T, e *Engine, colors []byte, genericWanted int) [][]string {
	t.Helper()
	var pipMenus [][]string
	for i := 0; i < 16; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			return pipMenus
		}
		kind := ""
		if len(d.Options) > 0 {
			kind = d.Options[0].Kind
		}
		switch {
		case strings.HasPrefix(kind, "convoke_"), kind == "harmonize", kind == "improvise_generic":
			used := map[state.ObjID]bool{}
			var choices []int
			for _, c := range colors {
				picked := false
				for _, o := range d.Options {
					if o.Kind == "convoke_"+string(c) && !used[o.Obj] {
						used[o.Obj] = true
						choices = append(choices, o.Index)
						picked = true
						break
					}
				}
				if !picked {
					t.Fatalf("the convoke ask lacks a %c contribution: %+v", c, d.Options)
				}
			}
			// genericWanted generic contributions from the rest.
			for _, o := range d.Options {
				if len(choices) >= len(colors)+genericWanted {
					break
				}
				if o.Kind == "convoke_generic" && !used[o.Obj] {
					used[o.Obj] = true
					choices = append(choices, o.Index)
				}
			}
			if len(choices) < len(colors)+genericWanted {
				t.Fatalf("the convoke ask lacks a covering answer: %+v", d.Options)
			}
			submitChoices(t, e, choices...)
		case strings.HasPrefix(kind, "pay_"):
			// A pip face: the first feasible one (its colour contribution
			// covers it).
			kinds := make([]string, 0, len(d.Options))
			for _, o := range d.Options {
				kinds = append(kinds, o.Kind)
			}
			pipMenus = append(pipMenus, kinds)
			submitChoices(t, e, 0)
		case kind == "exile":
			// Delve: the shortfall the Min forces.
			choices := make([]int, 0, d.Min)
			for i := 0; i < d.Min; i++ {
				choices = append(choices, i)
			}
			submitChoices(t, e, choices...)
		default:
			return pipMenus
		}
	}
	t.Fatalf("the cast-flow asks did not settle in 16 answers")
	return nil
}

// hogaakWantMenus asserts the cast's pip asks offered exactly the wanted
// option kinds, in order.
func hogaakWantMenus(t *testing.T, got [][]string, want ...[]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d pip menus, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if strings.Join(got[i], ",") != strings.Join(want[i], ",") {
			t.Fatalf("pip %d offered %v, want %v", i, got[i], want[i])
		}
	}
}

func TestHogaakCastOnlyByConvokeAndDelve(t *testing.T) {
	t.Parallel()

	// Four black and three green creatures plus two delve cards: the convoke
	// announcement takes one colour contribution per pip (B, G) and three
	// generic, Delve exiles two more, and the whole {5}{B/G}{B/G} is paid
	// without a unit of mana.
	e, cfg, id, creatures := hogaakBoard(t, 4, 3, 2)
	toMain1(t, e)
	opt, ok := hogaakCastOption(t, e, id)
	if !ok {
		t.Fatalf("Hogaak not offered with convoke and delve credits: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	// One black and one green contribution: each pip has a completing face
	// (the other pip takes the other credit), so pip 0 offers both, and after
	// pip 0 announced black only the green face of pip 1 completes -- the
	// uncovered black is refused, not silently offered by the empty-menu
	// defensive arm.
	menus := answerHogaakAsksPlan(t, e, []byte{'B', 'G'}, 3)
	hogaakWantMenus(t, menus, []string{"pay_B", "pay_G"}, []string{"pay_G"})
	drainStack(t, e, 24)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Hogaak not on the battlefield after the cast: %+v", o)
	}
	tapped := 0
	for _, cid := range creatures {
		if e.G.Obj(cid).Tapped {
			tapped++
		}
	}
	if tapped != 5 {
		t.Fatalf("%d convoked creatures tapped, want 5", tapped)
	}
	if n := len(e.G.Zone(state.ZGraveyard, 0)); n != 0 { // Hogaak entered from hand; the two fodder were exiled
		t.Fatalf("graveyard holds %d cards after the delve exiles, want 0", n)
	}
	if p := e.G.Players[0].Pool.Total(); p != 0 {
		t.Fatalf("pool after the cast = %v, want empty (no mana may be spent)", e.G.Players[0].Pool)
	}
	replayCheck(t, e, cfg)

	// One unit short, covered by floating mana: NOT offered. A cast whose
	// credits cannot pay the whole cost must never appear -- an offer the pay
	// step then refuses is a rewind.
	e, cfg, id, _ = hogaakBoard(t, 3, 2, 1)
	toMain1(t, e)
	addMana(t, e, 0, "G")
	if _, ok := hogaakCastOption(t, e, id); ok {
		t.Fatalf("Hogaak offered one unit short with floating mana: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)

	// Fully covered with floating mana beside: offered, and the floating mana
	// is never spent.
	e, cfg, id, _ = hogaakBoard(t, 4, 3, 2)
	toMain1(t, e)
	addMana(t, e, 0, "G")
	opt, ok = hogaakCastOption(t, e, id)
	if !ok {
		t.Fatalf("Hogaak not offered with floating mana beside full credits: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	answerHogaakAsks(t, e, 3)
	drainStack(t, e, 24)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Hogaak not on the battlefield after the cast: %+v", o)
	}
	if p := e.G.Players[0].Pool; p.Total() != 1 || p[state.MG] != 1 {
		t.Fatalf("pool after the cast = %v, want the floating {G} untouched", p)
	}
	replayCheck(t, e, cfg)

	// Both pips announced at one colour face. Two green contributions cover
	// both {B/G} pips (CR 702.51a's pip absorption: an unresolved hybrid pip
	// is exactly one unit a convoke creature of its colour pays), so every
	// pip menu offers green and only green -- the announceFeasible
	// composition must fold the credits into the unresolved pips the same way
	// the Submit gate did. A composition that drops a credit whose colour
	// unit is still unresolved leaves both faces unpayable, the menu falls to
	// the defensive arm's uncovered alternative, and the cast aborts at
	// payCast -- an offered payment whose completion is refused.
	e, cfg, id, _ = hogaakBoard(t, 3, 2, 2)
	toMain1(t, e)
	opt, ok = hogaakCastOption(t, e, id)
	if !ok {
		t.Fatalf("Hogaak not offered with two green contributions: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	menus = answerHogaakAsksPlan(t, e, []byte{'G', 'G'}, 3)
	hogaakWantMenus(t, menus, []string{"pay_G"}, []string{"pay_G"})
	drainStack(t, e, 24)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Hogaak not on the battlefield after the all-green cast: %+v", o)
	}
	if p := e.G.Players[0].Pool; p.Total() != 0 {
		t.Fatalf("pool after the all-green cast = %v, want empty", p)
	}
	replayCheck(t, e, cfg)

	// The same at black: two black contributions cover both pips at their
	// black faces, and no green face is ever offered.
	e, cfg, id, _ = hogaakBoard(t, 2, 3, 2)
	toMain1(t, e)
	opt, ok = hogaakCastOption(t, e, id)
	if !ok {
		t.Fatalf("Hogaak not offered with two black contributions: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	menus = answerHogaakAsksPlan(t, e, []byte{'B', 'B'}, 3)
	hogaakWantMenus(t, menus, []string{"pay_B"}, []string{"pay_B"})
	drainStack(t, e, 24)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Hogaak not on the battlefield after the all-black cast: %+v", o)
	}
	if p := e.G.Players[0].Pool; p.Total() != 0 {
		t.Fatalf("pool after the all-black cast = %v, want empty", p)
	}
	replayCheck(t, e, cfg)

	// The mode's source-restricted carriers stay unsupported: registering the
	// bare key would mark them playable while inert, and their shape needs
	// per-source pool provenance state.Player does not track.
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{"Myr Superion", "Imperiosaur"} {
		c, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("corpus missing %s", name)
		}
		m := r.Unsupported(c, sup)
		if len(m) == 0 {
			t.Fatalf("%s counted fully supported", name)
		}
		found := false
		for _, k := range m {
			if k == "stat:ManaRestriction.OtherShape" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s unsupported for the wrong reason: %v", name, m)
		}
	}
	h, ok2 := r.Lookup("Hogaak, Arisen Necropolis")
	if !ok2 {
		t.Fatal("corpus missing Hogaak, Arisen Necropolis")
	}
	if m := r.Unsupported(h, sup); len(m) != 0 {
		t.Fatalf("Hogaak unsupported: %v", m)
	}
}
