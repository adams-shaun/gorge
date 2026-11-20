package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	optimusFront = "Optimus Prime, Hero"
	optimusBack  = "Optimus Prime, Autobot Leader"
)

func convertedOption(t *testing.T, e *Engine, id state.ObjID) *decision.Option {
	t.Helper()
	for _, o := range castOptions(t, e) {
		if o.Mode == "converted" && o.Obj == id {
			cp := o
			return &cp
		}
	}
	return nil
}

func mtmteOpts(opts []decision.Option) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, fmt.Sprintf("%s#%d[mode=%q alt=%d]", o.Label, o.Obj, o.Mode, o.AltCostIndex))
	}
	return out
}

func mtmtePoolTotal(e *Engine, p state.PlayerID) int32 {
	var n int32
	for _, v := range e.G.Players[p].Pool {
		n += v
	}
	return n
}

// CR 702.162a: More Than Meets the Eye is an alternative cost to cast the card
// converted. Both the front-face cast ({3}{U}{R}{W}) and the converted cast
// ({2}{U}{R}{W}) are offered; the converted cast charges the MTMtE cost and
// the permanent enters as the back face (CR 712.11a).
func TestOptimusPrimeCastsConvertedForMTMTECost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, optimusFront)
	id := putNamed(t, e, 0, optimusFront, state.ZHand)
	addMana(t, e, 0, "UURWGG")
	if got := mtmtePoolTotal(e, 0); got != 6 {
		t.Fatalf("precondition: pool holds %d mana, want 6", got)
	}
	var plain, conv int
	for _, o := range castOptions(t, e) {
		if o.Obj != id {
			continue
		}
		switch o.Mode {
		case "":
			plain++
		case "converted":
			conv++
		}
	}
	if plain != 1 || conv != 1 {
		t.Fatalf("offers for %s: %d plain, %d converted, want one of each: %+v", optimusFront, plain, conv, mtmteOpts(castOptions(t, e)))
	}
	opt := convertedOption(t, e, id)
	submitChoices(t, e, opt.Index)
	o := e.G.Obj(id)
	if o.Zone != state.ZStack || o.FaceIdx != 1 || o.Face().Name != optimusBack {
		t.Fatalf("after paying: zone %s face %d %q, want the back face on the stack", o.Zone, o.FaceIdx, o.Face().Name)
	}
	if got := mtmtePoolTotal(e, 0); got != 1 {
		t.Fatalf("converted cast left %d mana, want 1 (charged {2}{U}{R}{W}, not the {3}{U}{R}{W} mana cost)", got)
	}
	passUntilStackEmpty(t, e, 40)
	o = e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.Face().Name != optimusBack {
		t.Fatalf("resolved: zone %s face %q, want %s on the battlefield", o.Zone, o.Face().Name, optimusBack)
	}
	replayCheck(t, e, cfg)
}

// With only the MTMtE cost's five mana available the front cast is not
// offered but the converted cast is, and with none neither is.
func TestMTMTEOfferFollowsItsOwnCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := proliferateEngine(t, reg, optimusFront)
	id := putNamed(t, e, 0, optimusFront, state.ZHand)
	addMana(t, e, 0, "UURW")
	if convertedOption(t, e, id) != nil {
		t.Fatalf("converted cast offered with 4 mana: %+v", mtmteOpts(castOptions(t, e)))
	}
	addMana(t, e, 0, "G")
	if convertedOption(t, e, id) == nil {
		t.Fatalf("converted cast not offered with {2}{U}{R}{W} available: %+v", mtmteOpts(castOptions(t, e)))
	}
	for _, o := range castOptions(t, e) {
		if o.Obj == id && o.Mode == "" {
			t.Fatalf("front cast offered with only 5 of its 6 mana: %+v", o)
		}
	}
}

// CR 712.8c: a spell cast converted has the back face's characteristics but
// its mana value is calculated from the front face's mana cost.
func TestConvertedSpellManaValueIsFrontFace(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := proliferateEngine(t, reg, optimusFront)
	id := putNamed(t, e, 0, optimusFront, state.ZHand)
	addMana(t, e, 0, "UURWG")
	opt := convertedOption(t, e, id)
	if opt == nil {
		t.Fatalf("precondition: no converted cast offered: %+v", mtmteOpts(castOptions(t, e)))
	}
	submitChoices(t, e, opt.Index)
	o := e.G.Obj(id)
	if o.Zone != state.ZStack || o.Face().Name != optimusBack {
		t.Fatalf("precondition: zone %s face %q, want the back face on the stack", o.Zone, o.Face().Name)
	}
	if back := o.Card.Faces[1].ManaValue(); back != 0 {
		t.Fatalf("precondition: back face mana value %d, want 0 (no cost)", back)
	}
	if mv := state.ManaValue(e.G, o); mv != 6 {
		t.Fatalf("converted spell mana value = %d, want the front face's 6", mv)
	}
}

// CR 702.161a: living metal makes the permanent an artifact creature during
// its controller's turn only.
func TestLivingMetalIsCreatureOnlyOnControllersTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := proliferateEngine(t, reg, optimusFront)
	id := putNamedOnBattlefield(t, e, optimusFront)
	e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1})
	toMain1(t, e)
	o := e.G.Obj(id)
	if o.Face().Name != optimusBack || e.G.Active != 0 {
		t.Fatalf("precondition: face %q, active seat %d; want %s on seat 0's turn", o.Face().Name, e.G.Active, optimusBack)
	}
	if !e.IsCreature(id) || !e.IsArtifact(id) {
		t.Fatalf("on its controller's turn the Vehicle is creature=%v artifact=%v, want both", e.IsCreature(id), e.IsArtifact(id))
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: e.G.Turn + 1})
	if e.G.Active != 1 {
		t.Fatalf("precondition: active seat %d after the turn change, want 1", e.G.Active)
	}
	if e.IsCreature(id) {
		t.Fatal("on the opponent's turn the Vehicle is still a creature")
	}
	if !e.IsArtifact(id) {
		t.Fatal("the Vehicle stopped being an artifact on the opponent's turn")
	}
}

// CR 702.162a, 118.9a: More Than Meets the Eye is an alternative cost, so it
// cannot combine with another one. Under Omniscience (cast spells from hand
// without paying their mana costs) the card is castable free, but the
// converted cast is never free: with nothing to pay it is not offered, and
// with the five mana of its cost it charges all of them.
func TestMTMTEIsAnAlternativeCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := proliferateEngine(t, reg, optimusFront, "Omniscience")
	putNamedOnBattlefield(t, e, "Omniscience")
	id := putNamed(t, e, 0, optimusFront, state.ZHand)
	addMana(t, e, 0, "G")
	var free int
	for _, o := range castOptions(t, e) {
		if o.Obj == id && o.Mode != "converted" {
			free++
		}
	}
	if free == 0 {
		t.Fatalf("precondition: Omniscience offers no free cast of %s: %+v", optimusFront, mtmteOpts(castOptions(t, e)))
	}
	if o := convertedOption(t, e, id); o != nil {
		t.Fatalf("converted cast offered for free by combining MTMtE with Omniscience: %+v", *o)
	}
	addMana(t, e, 0, "UURW")
	opt := convertedOption(t, e, id)
	if opt == nil {
		t.Fatalf("converted cast not offered with {2}{U}{R}{W} available: %+v", mtmteOpts(castOptions(t, e)))
	}
	if opt.AltCostIndex != 0 {
		t.Fatalf("converted cast rides alternative cost %d, want none", opt.AltCostIndex)
	}
	submitChoices(t, e, opt.Index)
	if o := e.G.Obj(id); o.Zone != state.ZStack || o.FaceIdx != 1 {
		t.Fatalf("converted cast: zone %s face %d, want the back face on the stack", o.Zone, o.FaceIdx)
	}
	if got := mtmtePoolTotal(e, 0); got != 0 {
		t.Fatalf("converted cast left %d mana, want 0 (the MTMtE cost is charged in full)", got)
	}
}

// The 15 Transformers double-faced cards are carried by kw:More Than Meets
// the Eye and kw:Living metal alone: none reports a missing feature.
func TestEveryTransformersCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	names := []string{
		"Arcee, Sharpshooter", "Blaster, Combat DJ", "Blitzwing, Cruel Tormentor",
		"Cyclonus, the Saboteur", "Flamewar, Brash Veteran", "Goldbug, Humanity's Ally",
		"Jetfire, Ingenious Scientist", "Megatron, Tyrant", "Optimus Prime, Hero",
		"Prowl, Stoic Strategist", "Ratchet, Field Medic", "Slicer, Hired Muscle",
		"Soundwave, Sonic Spy", "Starscream, Power Hungry", "Ultra Magnus, Tactician",
	}
	for _, name := range names {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Errorf("%s: not in the corpus", name)
			continue
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
