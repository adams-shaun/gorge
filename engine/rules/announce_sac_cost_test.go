package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// nahiriFixture puts Nahiri's Sacrifice in seat 0's hand with 4 red floating
// and the named permanents on the battlefields.
func nahiriFixture(t *testing.T, mine, theirs []string) (*Engine, state.ObjID, map[string][]state.ObjID, []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	seat0 := []*cards.Card{viviCard(t, reg, "Nahiri's Sacrifice")}
	for _, n := range mine {
		seat0 = append(seat0, viviCard(t, reg, n))
	}
	var seat1 []*cards.Card
	for _, n := range theirs {
		seat1 = append(seat1, viviCard(t, reg, n))
	}
	e, _ := viviEngine(t, reg, seat0, seat1)
	spell := moveByName(t, e, 0, "Nahiri's Sacrifice", state.ZHand)
	own := map[string][]state.ObjID{}
	for _, n := range mine {
		own[n] = append(own[n], moveByName(t, e, 0, n, state.ZBattlefield))
	}
	var opp []state.ObjID
	for _, n := range theirs {
		opp = append(opp, moveByName(t, e, 1, n, state.ZBattlefield))
	}
	e.G.Players[0].Pool[state.MR] = 4
	viviRefresh(t, e)
	return e, spell, own, opp
}

func nahiriCastOption(e *Engine, spell state.ObjID) *decision.Option {
	d := e.Pending()
	if d == nil {
		return nil
	}
	return viviOption(d, "cast", spell)
}

func xOptionAmounts(t *testing.T, d *decision.Decision) map[int]int {
	t.Helper()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
		t.Fatalf("expected the X ask, got %+v", d)
	}
	out := map[int]int{}
	for _, o := range d.Options {
		if o.Kind != "x" {
			t.Fatalf("non-X option in the X ask: %+v", d.Options)
		}
		out[o.Amount] = o.Index
	}
	return out
}

// finishNahiri answers targets (the opposing creature, when any) and then
// the sacrifice ask with fodder, returning the sacrifice ask's options.
func finishNahiri(t *testing.T, e *Engine, target state.ObjID, fodder state.ObjID) []decision.Option {
	t.Helper()
	var sacOpts []decision.Option
	for step := 0; step < 8; step++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			return sacOpts
		}
		if d.Kind == decision.KTarget {
			if target == 0 {
				submitChoices(t, e)
				continue
			}
			o := viviOption(d, "permanent", target)
			if o == nil {
				t.Fatalf("target not offered: %+v", d.Options)
			}
			submitChoices(t, e, o.Index)
			continue
		}
		if o := viviOption(d, "sacrifice", fodder); o != nil {
			sacOpts = d.Options
			submitChoices(t, e, o.Index)
			continue
		}
		t.Fatalf("unexpected decision %v %q: %+v", d.Kind, d.Prompt, d.Options)
	}
	t.Fatal("cast did not settle")
	return nil
}

func TestNahirisSacrificeAnnouncesXForSacrificeCost(t *testing.T) {
	t.Parallel()
	e, spell, own, opp := nahiriFixture(t, []string{"Grizzly Bears", "Ornithopter"}, []string{"Grizzly Bears"})
	bear, orni, theirs := own["Grizzly Bears"][0], own["Ornithopter"][0], opp[0]
	for _, id := range []state.ObjID{bear, orni, theirs} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("object %d is not on the battlefield", id)
		}
	}
	if e.G.Obj(bear).Face().ManaValue() == e.G.Obj(orni).Face().ManaValue() {
		t.Fatal("the fodder mana values must differ")
	}
	cast := nahiriCastOption(e, spell)
	if cast == nil {
		t.Fatalf("no cast option for Nahiri's Sacrifice: %+v", e.Pending().Options)
	}
	submitChoices(t, e, cast.Index)
	xs := xOptionAmounts(t, e.Pending())
	if len(xs) != 2 {
		t.Fatalf("X options %v, want exactly {0, 2}", xs)
	}
	two, ok := xs[2]
	if _, zero := xs[0]; !ok || !zero {
		t.Fatalf("X options %v, want exactly {0, 2}", xs)
	}
	submitChoices(t, e, two)
	sacOpts := finishNahiri(t, e, theirs, bear)
	if len(sacOpts) != 1 || sacOpts[0].Obj != bear {
		t.Fatalf("sacrifice candidates %+v, want exactly p0's Grizzly Bears", sacOpts)
	}
	viviPass(t, e)
	viviPass(t, e)
	if z := e.G.Obj(theirs).Zone; z != state.ZGraveyard {
		t.Fatalf("p1's bear zone %v, want graveyard (X = 2 damage)", z)
	}
	if z := e.G.Obj(bear).Zone; z != state.ZGraveyard {
		t.Fatalf("p0's bear zone %v, want graveyard (sacrificed)", z)
	}
	if z := e.G.Obj(orni).Zone; z != state.ZBattlefield {
		t.Fatalf("Ornithopter zone %v, want battlefield", z)
	}
}

func TestNahirisSacrificeWithheldWithoutArtifactOrCreature(t *testing.T) {
	t.Parallel()
	e, spell, _, _ := nahiriFixture(t, nil, nil)
	if n := len(e.G.Zone(state.ZBattlefield, 0)); n != 0 {
		t.Fatalf("precondition: seat 0 battlefield has %d permanents, want none", n)
	}
	if o := e.G.Obj(spell); o == nil || o.Zone != state.ZHand {
		t.Fatal("precondition: the spell is not in hand")
	}
	if e.G.Players[0].Pool[state.MR] != 4 {
		t.Fatal("precondition: red mana not floating")
	}
	if c := nahiriCastOption(e, spell); c != nil {
		t.Fatalf("cast offered without an artifact or creature to sacrifice: %+v", c)
	}
}

func TestNahirisSacrificeAnnouncesZeroForManaValueZeroFodder(t *testing.T) {
	t.Parallel()
	e, spell, own, _ := nahiriFixture(t, []string{"Ornithopter"}, nil)
	orni := own["Ornithopter"][0]
	if o := e.G.Obj(orni); o == nil || o.Zone != state.ZBattlefield || o.Face().ManaValue() != 0 {
		t.Fatal("precondition: Ornithopter is not a mana value 0 permanent on the battlefield")
	}
	cast := nahiriCastOption(e, spell)
	if cast == nil {
		t.Fatalf("no cast option: %+v", e.Pending().Options)
	}
	submitChoices(t, e, cast.Index)
	xs := xOptionAmounts(t, e.Pending())
	zero, ok := xs[0]
	if len(xs) != 1 || !ok {
		t.Fatalf("X options %v, want exactly {0}", xs)
	}
	submitChoices(t, e, zero)
	finishNahiri(t, e, 0, orni)
	viviPass(t, e)
	viviPass(t, e)
	if z := e.G.Obj(orni).Zone; z != state.ZGraveyard {
		t.Fatalf("Ornithopter zone %v, want graveyard (sacrificed)", z)
	}
}
