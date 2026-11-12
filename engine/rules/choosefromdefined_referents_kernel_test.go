package rules

// The ChangeZone ChooseFromDefined$ referent pools -- what each
// real corpus leg offers, and that the answered pick is what moves --
// answered through the resolution kernel on a real engine.

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// chooseFromDefinedSA locates a corpus card's ChangeZone leg carrying the
// named ChooseFromDefined$ spelling (printed chains first, then SVars).
func chooseFromDefinedSA(t *testing.T, cardName, spelling string) *cards.SA {
	t.Helper()
	c := corpusCardNamed(t, cardName)
	for _, face := range c.Faces {
		for _, ab := range face.Abilities {
			for sub := ab; sub != nil; sub = sub.Sub {
				if sub.Params["ChooseFromDefined"] == spelling {
					return sub
				}
			}
		}
		for _, name := range sortedKeysOf(face.SVars) {
			if sa := cards.ResolveSVar(face.SVars, name); sa != nil && sa.Params["ChooseFromDefined"] == spelling {
				return sa
			}
		}
	}
	t.Fatalf("corpus pin moved: %q no longer carries ChooseFromDefined$ %s", cardName, spelling)
	return nil
}

// isolateSA copies sa with its SubAbility chain detached.
func isolateSA(sa *cards.SA) *cards.SA {
	cp := *sa
	cp.Sub = nil
	return &cp
}

// sortedKeysOf is m's keys in sorted order (a deterministic SVar walk).
func sortedKeysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestChangeZoneChooseFromDefinedRememberedColorKernel: Sanar's
// `Remembered.White` leg offers only the white remembered library card, and
// the answered pick moves to exile while the non-members stay.
func TestChangeZoneChooseFromDefinedRememberedColorKernel(t *testing.T) {
	t.Parallel()
	s := isolateSA(chooseFromDefinedSA(t, "Sanar, Innovative First-Year", "Remembered.White"))
	if s.API != "ChangeZone" || s.Params["Origin"] != "Library" || s.Params["Destination"] != "Exile" {
		t.Fatalf("corpus pin moved: leg is %+v", s)
	}
	e := fixtureEngine(t, 2)
	sanar := placeSrc(t, e, 0, "Name:Sanar\nTypes:Creature Goblin\nPT:2/4\nOracle:x\n", state.ZBattlefield)
	lib := setLibrarySrcs(t, e, 0,
		"Name:White Runt\nManaCost:W\nTypes:Creature\nPT:1/1\nOracle:x\n",
		"Name:Blue Drake\nManaCost:1 U\nTypes:Creature\nPT:2/2\nOracle:x\n",
		"Name:Test Plains\nTypes:Land\nOracle:x\n")
	white, blue, land := lib[0], lib[1], lib[2]
	d := runProbeSA(t, e, s, func() *effects.Ctx {
		return &effects.Ctx{Source: sanar, Controller: 0, Remembered: []state.Target{{Obj: white}, {Obj: blue}}}
	}, nil)
	if d == nil || d.ResumeKind != "search" || d.Player != 0 {
		t.Fatalf("no search ask for the Remembered.White leg: %+v", d)
	}
	requireOfferSet(t, d, white)
	answerAsk(t, e, optionIdxForObj(t, d, white))
	requireZone(t, e, white, state.ZExile)
	requireZone(t, e, blue, state.ZLibrary)
	requireZone(t, e, land, state.ZLibrary)
}

// TestChangeZoneChooseFromDefinedRememberedBareSearchPoolKernel: The
// Celestial Toymaker's bare `Remembered` leg offers the whole remembered set
// (ChangeNum$ 3); the deeper library card is never offered.
func TestChangeZoneChooseFromDefinedRememberedBareSearchPoolKernel(t *testing.T) {
	t.Parallel()
	s := isolateSA(chooseFromDefinedSA(t, "The Celestial Toymaker", "Remembered"))
	if s.API != "ChangeZone" || s.Params["Origin"] != "Library" || s.Params["Destination"] != "Exile" || s.Params["ChangeNum"] != "3" {
		t.Fatalf("corpus pin moved: leg is %+v", s)
	}
	e := fixtureEngine(t, 2)
	toy := placeSrc(t, e, 0, "Name:Toymaker\nTypes:Creature Rogue\nPT:2/4\nOracle:x\n", state.ZBattlefield)
	lib := setLibrarySrcs(t, e, 0,
		"Name:Peeked One\nManaCost:1 W\nTypes:Creature\nPT:1/1\nOracle:x\n",
		"Name:Peeked Two\nManaCost:1 U\nTypes:Creature\nPT:1/1\nOracle:x\n",
		"Name:Peeked Three\nManaCost:1 B\nTypes:Creature\nPT:1/1\nOracle:x\n",
		"Name:Deep Distraction\nManaCost:2\nTypes:Creature\nPT:2/2\nOracle:x\n")
	p1, p2, p3, deep := lib[0], lib[1], lib[2], lib[3]
	d := runProbeSA(t, e, s, func() *effects.Ctx {
		return &effects.Ctx{Source: toy, Controller: 0, Remembered: []state.Target{{Obj: p1}, {Obj: p2}, {Obj: p3}}}
	}, nil)
	if d == nil {
		t.Fatal("no search ask for the bare Remembered leg")
	}
	requireOfferSet(t, d, p1, p2, p3)
	answerAsk(t, e, optionIdxForObj(t, d, p1), optionIdxForObj(t, d, p2), optionIdxForObj(t, d, p3))
	for _, id := range []state.ObjID{p1, p2, p3} {
		requireZone(t, e, id, state.ZExile)
	}
	requireZone(t, e, deep, state.ZLibrary)
}

// TestChangeZoneChooseFromDefinedTopThirdOfLibraryKernel: Assemble the
// Team's `TopThirdOfLibrary` offers the top third rounded up (3 of 7), never
// a deeper card; the answered pick goes to hand and nothing else moves.
func TestChangeZoneChooseFromDefinedTopThirdOfLibraryKernel(t *testing.T) {
	t.Parallel()
	s := chooseFromDefinedSA(t, "Assemble the Team", "TopThirdOfLibrary")
	if s.API != "ChangeZone" || s.Params["Origin"] != "Library" || s.Params["Destination"] != "Hand" || s.Params["Hidden"] == "True" {
		t.Fatalf("corpus pin moved: leg is %+v", s)
	}
	e := fixtureEngine(t, 2)
	src := placeSrc(t, e, 0, "Name:Team Source\nTypes:Sorcery\nOracle:x\n", state.ZBattlefield)
	ids := setLibrarySrcs(t, e, 0,
		"Name:Top One\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n",
		"Name:Top Two\nManaCost:2\nTypes:Creature\nPT:2/2\nOracle:x\n",
		"Name:Top Three\nManaCost:3\nTypes:Creature\nPT:3/3\nOracle:x\n",
		"Name:Deep One\nManaCost:4\nTypes:Creature\nPT:4/4\nOracle:x\n",
		"Name:Deep Two\nManaCost:5\nTypes:Creature\nPT:5/5\nOracle:x\n",
		"Name:Deep Three\nManaCost:6\nTypes:Creature\nPT:6/6\nOracle:x\n",
		"Name:Deep Four\nManaCost:7\nTypes:Creature\nPT:7/7\nOracle:x\n")
	d := runProbeSA(t, e, s, func() *effects.Ctx { return &effects.Ctx{Source: src, Controller: 0} }, nil)
	if d == nil || d.ResumeKind != "search" {
		t.Fatalf("no search ask for the TopThirdOfLibrary leg: %+v", d)
	}
	requireOfferSet(t, d, ids[:3]...)
	answerAsk(t, e, optionIdxForObj(t, d, ids[1]))
	requireZone(t, e, ids[1], state.ZHand)
	for i, id := range ids {
		if i != 1 {
			requireZone(t, e, id, state.ZLibrary)
		}
	}
}

// TestChangeZoneChooseFromDefinedTriggeredSourcesKernel: Zurgo and Ojutai's
// `TriggeredSources` offers only the trigger's source Dragon; the pick
// returns it to hand.
func TestChangeZoneChooseFromDefinedTriggeredSourcesKernel(t *testing.T) {
	t.Parallel()
	s := chooseFromDefinedSA(t, "Zurgo and Ojutai", "TriggeredSources")
	if s.API != "ChangeZone" || s.Params["Origin"] != "Battlefield" || s.Params["Destination"] != "Hand" {
		t.Fatalf("corpus pin moved: leg is %+v", s)
	}
	e := fixtureEngine(t, 2)
	zurgo := placeSrc(t, e, 0, "Name:Zurgo and Ojutai\nTypes:Creature Orc Dragon\nPT:4/4\nOracle:x\n", state.ZBattlefield)
	dragon := placeSrc(t, e, 0, "Name:Dmg Dragon\nTypes:Creature Dragon\nPT:4/4\nOracle:x\n", state.ZBattlefield)
	bear := placeSrc(t, e, 0, "Name:Other Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	d := runProbeSA(t, e, s, func() *effects.Ctx {
		return &effects.Ctx{Source: zurgo, Controller: 0, TriggerContext: effects.TriggerContext{TriggerSource: dragon}}
	}, nil)
	if d == nil || d.ResumeKind != "hidden_pick" {
		t.Fatalf("no hidden-pick ask for the TriggeredSources leg: %+v", d)
	}
	requireOfferSet(t, d, dragon)
	answerAsk(t, e, optionIdxForObj(t, d, dragon))
	requireZone(t, e, dragon, state.ZHand)
	requireZone(t, e, bear, state.ZBattlefield)
}

// TestChangeZoneChooseFromDefinedTargetedCmcLE4Kernel: Back for Seconds'
// `Targeted.cmcLE4` first poses its Optional$ confirm, then offers only the
// targeted graveyard creature with mana value <= 4.
func TestChangeZoneChooseFromDefinedTargetedCmcLE4Kernel(t *testing.T) {
	t.Parallel()
	s := isolateSA(chooseFromDefinedSA(t, "Back for Seconds", "Targeted.cmcLE4"))
	// The corpus leg also has Condition$ Bargain, which this engine cannot
	// evaluate and now correctly fails closed. This kernel isolates the
	// ChooseFromDefined/Optional ask contract, so remove that unrelated gate.
	delete(s.Params, "Condition")
	if s.API != "ChangeZone" || s.Params["Origin"] != "Graveyard" || s.Params["Destination"] != "Battlefield" {
		t.Fatalf("corpus pin moved: leg is %+v", s)
	}
	e := fixtureEngine(t, 2)
	spell := placeSrc(t, e, 0, "Name:Back for Seconds\nManaCost:2 B\nTypes:Sorcery\nOracle:x\n", state.ZHand)
	cheap := placeSrc(t, e, 0, "Name:Cheep Target\nManaCost:2 B\nTypes:Creature\nPT:3/2\nOracle:x\n", state.ZGraveyard)
	pricey := placeSrc(t, e, 0, "Name:Pricey Target\nManaCost:5 B B\nTypes:Creature\nPT:7/7\nOracle:x\n", state.ZGraveyard)
	bear := placeSrc(t, e, 0, "Name:Untargeted Bear\nManaCost:1\nTypes:Creature\nPT:2/2\nOracle:x\n", state.ZGraveyard)
	d := runProbeSA(t, e, s, func() *effects.Ctx {
		return &effects.Ctx{Source: spell, Controller: 0, Targets: []state.Target{{Obj: cheap}, {Obj: pricey}}}
	}, nil)
	if d == nil || d.ResumeKind != "hidden_pick_confirm" {
		t.Fatalf("first ask = %+v, want the hidden_pick_confirm gate", d)
	}
	d = answerAsk(t, e, optionKindIndex(t, d, "yes"))
	if d == nil || d.ResumeKind != "hidden_pick" {
		t.Fatalf("no hidden-pick ask for the Targeted.cmcLE4 leg: %+v", d)
	}
	requireOfferSet(t, d, cheap)
	answerAsk(t, e, optionIdxForObj(t, d, cheap))
	requireZone(t, e, cheap, state.ZBattlefield)
	requireZone(t, e, pricey, state.ZGraveyard)
	requireZone(t, e, bear, state.ZGraveyard)
}

// TestChangeZoneChooseFromDefinedExiledWithCreatureKernel: The Grim
// Captain's `ExiledWith.Creature` offers only the creature exiled with the
// source (not its exiled land, not another exiler's creature).
func TestChangeZoneChooseFromDefinedExiledWithCreatureKernel(t *testing.T) {
	t.Parallel()
	s := chooseFromDefinedSA(t, "The Grim Captain", "ExiledWith.Creature")
	if s.API != "ChangeZone" || s.Params["Origin"] != "Exile" || s.Params["Destination"] != "Battlefield" {
		t.Fatalf("corpus pin moved: leg is %+v", s)
	}
	e := fixtureEngine(t, 2)
	captain := placeSrc(t, e, 0, "Name:The Grim Captain\nTypes:Creature Nightmare Pirate\nPT:7/7\nOracle:x\n", state.ZBattlefield)
	creature := placeSrc(t, e, 0, "Name:Exiled Creature\nManaCost:3 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n", state.ZExile)
	land := placeSrc(t, e, 0, "Name:Exiled Land\nTypes:Land\nOracle:x\n", state.ZExile)
	other := placeSrc(t, e, 0, "Name:Other Exiler\nTypes:Creature\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	snake := placeSrc(t, e, 0, "Name:Exiled Snake\nManaCost:1 G\nTypes:Creature Snake\nPT:1/1\nOracle:x\n", state.ZExile)
	e.G.Obj(creature).ExiledWith = captain
	e.G.Obj(land).ExiledWith = captain
	e.G.Obj(snake).ExiledWith = other
	d := runProbeSA(t, e, s, func() *effects.Ctx { return &effects.Ctx{Source: captain, Controller: 0} }, nil)
	if d == nil || d.ResumeKind != "hidden_pick" {
		t.Fatalf("no hidden-pick ask for the ExiledWith.Creature leg: %+v", d)
	}
	requireOfferSet(t, d, creature)
	answerAsk(t, e, optionIdxForObj(t, d, creature))
	requireZone(t, e, creature, state.ZBattlefield)
	requireZone(t, e, land, state.ZExile)
	requireZone(t, e, snake, state.ZExile)
}

// TestChangeZoneChooseFromDefinedReplacedCardsKernel: Averna's
// `ReplacedCards.Land` offers only the bound batch's lands (never a
// nonmember land in the same zone) and the answered land moves; with no
// binding the pool is empty -- no ask, nothing moved.
func TestChangeZoneChooseFromDefinedReplacedCardsKernel(t *testing.T) {
	t.Parallel()
	s := isolateSA(chooseFromDefinedSA(t, "Averna, the Chaos Bloom", "ReplacedCards.Land"))
	if s.API != "ChangeZone" || s.Params["Origin"] != "Exile" || s.Params["Destination"] != "Battlefield" {
		t.Fatalf("corpus pin moved: leg is %+v", s)
	}
	e := fixtureEngine(t, 2)
	aver := placeSrc(t, e, 0, "Name:Averna\nTypes:Creature Elemental\nPT:4/2\nOracle:x\n", state.ZBattlefield)
	land := placeSrc(t, e, 0, "Name:Batch Land\nTypes:Land\nOracle:x\n", state.ZExile)
	beast := placeSrc(t, e, 0, "Name:Exiled Beast\nManaCost:2 G\nTypes:Creature Beast\nPT:3/3\nOracle:x\n", state.ZExile)
	other := placeSrc(t, e, 0, "Name:Nonmember Land\nTypes:Land\nOracle:x\n", state.ZExile)
	d := runProbeSA(t, e, s, func() *effects.Ctx {
		return &effects.Ctx{Source: aver, Controller: 0, Repl: effects.ReplacementInputs{Cards: []state.ObjID{land, beast}}}
	}, nil)
	if d == nil || d.ResumeKind != "hidden_pick" {
		t.Fatalf("no hidden-pick ask for the ReplacedCards.Land leg: %+v", d)
	}
	requireOfferSet(t, d, land)
	answerAsk(t, e, optionIdxForObj(t, d, land))
	requireZone(t, e, land, state.ZBattlefield)
	requireZone(t, e, other, state.ZExile)
	requireZone(t, e, beast, state.ZExile)

	e2 := fixtureEngine(t, 2)
	aver2 := placeSrc(t, e2, 0, "Name:Averna\nTypes:Creature Elemental\nPT:4/2\nOracle:x\n", state.ZBattlefield)
	land2 := placeSrc(t, e2, 0, "Name:Exiled Land\nTypes:Land\nOracle:x\n", state.ZExile)
	if d := runProbeSA(t, e2, s, func() *effects.Ctx { return &effects.Ctx{Source: aver2, Controller: 0} }, nil); d != nil {
		t.Fatalf("a pick was offered over an unbound ReplacedCards pool: %+v", d)
	}
	requireZone(t, e2, land2, state.ZExile)
}
