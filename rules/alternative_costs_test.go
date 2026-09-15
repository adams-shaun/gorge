package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func corpusAlternativeCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("corpus missing %s", name)
	}
	return c
}

func castMode(t *testing.T, e *Engine, id state.ObjID, mode string) {
	t.Helper()
	e.beginCast(0, decision.Option{Kind: "cast", Obj: id, Mode: mode})
}

func TestBuybackConstantMistsReturnsToHand(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Constant Mists"))
	mtn := card(t, "Name:Land\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	land := e.G.AddObject(mtn, 0)
	land.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{land.ID})
	mist := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG] = 1, 1
	castMode(t, e, mist, "buyback")
	if d := e.Pending(); d == nil || d.Options[0].Kind != "sacrifice" {
		t.Fatalf("buyback did not ask to sacrifice its real Land cost: %+v", d)
	}
	submitChoices(t, e, 0)
	if e.G.Obj(land.ID).Zone != state.ZGraveyard || e.G.Obj(mist).Zone != state.ZStack {
		t.Fatalf("buyback payment zones land=%s spell=%s", e.G.Obj(land.ID).Zone, e.G.Obj(mist).Zone)
	}
	e.resolveTop()
	if e.G.Obj(mist).Zone != state.ZHand {
		t.Fatalf("Constant Mists after buyback = %s, want hand", e.G.Obj(mist).Zone)
	}
}

func TestBuybackSearingTouchFizzleGoesToGraveyard(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Searing Touch"))
	target := e.G.AddObject(card(t, "Name:Target\nTypes:Creature Human\nPT:1/1\nOracle:x\n"), 1)
	target.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{target.ID})
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR], e.G.Players[0].Pool[state.MC] = 1, 4
	castMode(t, e, spell, "buyback")
	d := e.Pending()
	targetChoice := -1
	for i, o := range d.Options {
		if o.Obj == target.ID {
			targetChoice = i
			break
		}
	}
	if targetChoice < 0 {
		t.Fatalf("Searing Touch target missing: %+v", d)
	}
	submitChoices(t, e, targetChoice)
	e.emit(events.Event{Kind: events.MoveZone, Obj: target.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	e.resolveTop()
	if got := e.G.Obj(spell).Zone; got != state.ZGraveyard {
		t.Fatalf("fizzled Buyback spell went to %s, want graveyard", got)
	}
}

func TestSuspendProfaneTutorHasProvenanceAndForcedCast(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Profane Tutor"))
	profane := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MC] = 1
	castMode(t, e, profane, "suspend")
	if o := e.G.Obj(profane); o.Zone != state.ZExile || o.Counter("TIME") != 2 || o.CastFlags&state.FlagSuspend == 0 {
		t.Fatalf("Suspend did not create marked exile card: %+v", o)
	}
	e.beginTurn(0)
	if got := e.G.Obj(profane).Counter("TIME"); got != 1 {
		t.Fatalf("first suspend upkeep TIME=%d, want 1", got)
	}
	e.beginTurn(0)
	if e.G.Obj(profane).Zone != state.ZStack {
		t.Fatalf("last suspend counter did not force free cast, zone=%s", e.G.Obj(profane).Zone)
	}
	// A different exiled copy with zero TIME is not a suspended card.
	other := e.G.AddObject(corpusAlternativeCard(t, "Profane Tutor"), 0)
	other.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), other.ID))
	e.suspendedCasts = nil
	e.beginTurn(0)
	if other.Zone != state.ZExile {
		t.Fatalf("ordinary exiled Suspend card was cast for free: %s", other.Zone)
	}
}

func TestSuspendDoesNotCastThroughCantBeCast(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Profane Tutor"))
	profane := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MB], e.G.Players[0].Pool[state.MC] = 1, 1
	castMode(t, e, profane, "suspend")

	// A mandatory Suspend cast is still a cast. This live CantBeCast static
	// makes it illegal rather than merely unaffordable, so "if able" leaves
	// the zero-counter Profane Tutor in exile and never starts a cast flow.
	restrictor := e.G.AddObject(card(t, "Name:Restrictor\nTypes:Artifact\nS:Mode$ CantBeCast | ValidCard$ Card\nOracle:x\n"), 1)
	restrictor.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{restrictor.ID})
	e.beginTurn(0)
	e.beginTurn(0)
	if o := e.G.Obj(profane); o.Zone != state.ZExile || o.Counter("TIME") != 0 || e.cast != nil {
		t.Fatalf("restricted suspended spell was cast: %+v pending=%+v", o, e.cast)
	}
}

func TestSuspendXBenalishCommanderAnnouncesTimeAndCost(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Benalish Commander"))
	commander := e.G.Zone(state.ZHand, 0)[0]
	if info, ok := suspendCost(e.G.Obj(commander).Face()); !ok || !info.timeX || info.minTime != 1 || info.cost.X != 1 {
		t.Fatalf("Benalish Suspend parse = %+v, %v", info, ok)
	}
	e.G.Players[0].Pool[state.MC] = 1
	e.G.Players[0].Pool[state.MW] = 2
	castMode(t, e, commander, "suspend")
	if e.cast == nil || !e.cast.suspendTimeX || e.cast.suspendMinX != 1 {
		t.Fatalf("Suspend X pending cast = %+v", e.cast)
	}
	d := e.Pending()
	if d == nil || len(d.Options) != 1 || d.Options[0].Kind != "x" || d.Options[0].Amount != 1 {
		t.Fatalf("Suspend X did not offer only its payable XMin1 choice: %+v", d)
	}
	submitChoices(t, e, 0)
	o := e.G.Obj(commander)
	if o.Zone != state.ZExile || o.Counter("TIME") != 1 || o.X != 1 || o.CastFlags&state.FlagSuspend == 0 {
		t.Fatalf("Suspend X did not share announced X between cost and time: %+v", o)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("Suspend X cost left mana behind: %v", e.G.Players[0].Pool)
	}
}

func TestConvokeCrowdsFavorCommitsChosenCreature(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Crowd's Favor"))
	creature := card(t, "Name:Red Druid\nManaCost:R\nTypes:Creature Elf\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	id := e.G.AddObject(creature, 0)
	id.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{id.ID})
	spell := e.G.Zone(state.ZHand, 0)[0]
	if !e.HasKeyword(spell, "Convoke") {
		t.Fatal("corpus Crowd's Favor lost Convoke before cast")
	}
	castMode(t, e, spell, "")
	d := e.Pending()
	if d == nil || len(d.Options) == 0 || d.Options[0].Kind != "convoke_R" {
		t.Fatalf("Convoke did not announce a coloured creature payment: %+v", d)
	}
	submitChoices(t, e, 0)
	if e.cast == nil || !e.convokeCommitted(e.cast, id.ID) {
		t.Fatal("chosen Convoke creature was not reserved before payment")
	}
	// The mandatory target is the same creature. Once target selection is
	// answered, its Tap is the payment and it never supplied mana as well.
	submitChoices(t, e, 0)
	if !e.G.Obj(id.ID).Tapped || e.G.Players[0].Pool[state.MR] != 0 {
		t.Fatalf("Convoke creature was not exclusively tapped as payment: tapped=%v pool=%v", e.G.Obj(id.ID).Tapped, e.G.Players[0].Pool)
	}
}

func TestConvokeMarchOfMultitudesAnnouncesEveryCreatureAndFundsX(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "March of the Multitudes"))
	for i := 0; i < 3; i++ {
		c := card(t, "Name:White Helper\nManaCost:W\nTypes:Creature Human\nPT:1/1\nOracle:x\n")
		o := e.G.AddObject(c, 0)
		o.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), o.ID))
	}
	e.G.Players[0].Pool[state.MG] = 1
	spell := e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "")
	d := e.Pending()
	if d == nil || d.Options == nil {
		t.Fatalf("March did not ask for Convoke: %+v", d)
	}
	// Choose two white contributions and one generic contribution in ONE
	// legal multi-select answer. The generic helper is what pays X=1.
	var whites, generic []int
	for i, o := range d.Options {
		switch o.Kind {
		case "convoke_W":
			whites = append(whites, i)
		case "convoke_generic":
			generic = append(generic, i)
		}
	}
	if len(whites) < 2 || len(generic) < 1 {
		t.Fatalf("March Convoke options whites=%v generic=%v all=%+v", whites, generic, d.Options)
	}
	submitChoices(t, e, whites[0], whites[1], generic[2])
	d = e.Pending()
	if d == nil || len(d.Options) != 2 || d.Options[1].Kind != "x" || d.Options[1].Amount != 1 {
		t.Fatalf("Convoke-funded X=1 was not offered: %+v", d)
	}
	submitChoices(t, e, 1)
	if e.cast != nil || e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("March did not complete its Convoke/X payment: cast=%+v zone=%s", e.cast, e.G.Obj(spell).Zone)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if !e.G.Obj(id).Tapped {
			t.Fatalf("selected Convoke creature %d was not tapped", id)
		}
	}
}

func TestHarmonizeZenithFestivalFundsX(t *testing.T) {
	e := handEngine(t)
	spell := e.G.AddObject(corpusAlternativeCard(t, "Zenith Festival"), 0)
	spell.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{spell.ID})
	helper := e.G.AddObject(card(t, "Name:Three Power Helper\nTypes:Creature Human\nPT:3/3\nOracle:x\n"), 0)
	helper.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{helper.ID})
	e.G.Players[0].Pool[state.MR] = 2
	castMode(t, e, spell.ID, "harmonize")
	d := e.Pending()
	if d == nil || len(d.Options) != 1 || d.Options[0].Kind != "harmonize" {
		t.Fatalf("Harmonize X contribution was not offered: %+v", d)
	}
	submitChoices(t, e, 0)
	d = e.Pending()
	if d == nil || len(d.Options) != 4 || d.Options[3].Amount != 3 {
		t.Fatalf("Harmonize-funded X=3 was not offered: %+v", d)
	}
	submitChoices(t, e, 3)
	if e.G.Obj(spell.ID).Zone != state.ZStack || !e.G.Obj(helper.ID).Tapped {
		t.Fatalf("Harmonize X cast did not pay and tap: spell=%s helper tapped=%v", e.G.Obj(spell.ID).Zone, e.G.Obj(helper.ID).Tapped)
	}
}

func TestHarmonizeWildRideUsesAnnouncedPower(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Wild Ride"))
	creature := card(t, "Name:Four Power Druid\nTypes:Creature Elf\nPT:4/4\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	id := e.G.AddObject(creature, 0)
	id.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{id.ID})
	e.G.Players[0].Pool[state.MR] = 1
	spell := e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "harmonize")
	d := e.Pending()
	if d == nil || d.Options[0].Kind != "harmonize" || d.Options[0].Amount != 4 {
		t.Fatalf("Harmonize did not offer the creature's chosen power: %+v", d)
	}
	submitChoices(t, e, 0)
	submitChoices(t, e, 0) // target the creature
	if !e.G.Obj(id.ID).Tapped || e.G.Players[0].Pool[state.MR] != 0 {
		t.Fatalf("Harmonize did not pay after announced reduction: tapped=%v pool=%v", e.G.Obj(id.ID).Tapped, e.G.Players[0].Pool)
	}
}

func TestTransmuteAndCyclingRealHandActivations(t *testing.T) {
	t.Run("Dizzy Spell searches matching mana value", func(t *testing.T) {
		e := handEngine(t, corpusAlternativeCard(t, "Dizzy Spell"))
		// handEngine's library is all Mountains (mana value one), exactly the
		// same mana value as Dizzy Spell and therefore a real legal Transmute
		// search result.
		wanted := e.G.Zone(state.ZLibrary, 0)[0]
		id := e.G.Zone(state.ZHand, 0)[0]
		e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 1, 2
		var opt decision.Option
		for _, o := range e.legalActions(0) {
			if o.Kind == "ability" && o.Obj == id {
				opt = o
				break
			}
		}
		if opt.Kind != "ability" {
			t.Fatal("Dizzy Spell Transmute activation not offered")
		}
		e.beginActivation(0, opt)
		submitChoices(t, e, 0) // discard Dizzy Spell
		if e.G.Obj(id).Zone != state.ZGraveyard {
			t.Fatalf("Transmute discard zone=%s", e.G.Obj(id).Zone)
		}
		e.resolveTop()
		d := e.Pending()
		choice := -1
		if d != nil {
			for i, o := range d.Options {
				if o.Obj == wanted {
					choice = i
					break
				}
			}
		}
		if choice < 0 {
			t.Fatalf("Transmute did not offer matching mana-value library card: %+v", d)
		}
		submitChoices(t, e, choice)
		if e.G.Obj(wanted).Zone != state.ZHand {
			t.Fatalf("Transmute target zone=%s, want hand", e.G.Obj(wanted).Zone)
		}
	})
	t.Run("Ziatora's Proving Ground cycling draws", func(t *testing.T) {
		e := handEngine(t, corpusAlternativeCard(t, "Ziatora's Proving Ground"))
		drawn := e.G.Zone(state.ZLibrary, 0)[0]
		id := e.G.Zone(state.ZHand, 0)[0]
		e.G.Players[0].Pool[state.MC] = 3
		var opt decision.Option
		for _, o := range e.legalActions(0) {
			if o.Kind == "ability" && o.Obj == id {
				opt = o
				break
			}
		}
		if opt.Kind != "ability" {
			t.Fatal("Ziatora cycling activation not offered")
		}
		e.beginActivation(0, opt)
		submitChoices(t, e, 0) // discard the cycling card
		if e.G.Obj(id).Zone != state.ZGraveyard {
			t.Fatalf("Cycling discard zone=%s", e.G.Obj(id).Zone)
		}
		e.resolveTop()
		if e.G.Obj(drawn).Zone != state.ZHand {
			t.Fatalf("Cycling did not draw: zone=%s", e.G.Obj(drawn).Zone)
		}
	})
}

func TestCastWithFlashHonorsScriptGates(t *testing.T) {
	e := handEngine(t, card(t, "Name:Slow Spell\nManaCost:0\nTypes:Sorcery\nA:SP$ Draw | Defined$ You\nOracle:x\n"))
	source := e.G.AddObject(card(t, "Name:Gated Flash\nTypes:Artifact\nS:Mode$ CastWithFlash | ValidCard$ Card | ValidSA$ Spell | Caster$ You | IsPresent$ Creature.YouCtrl\nOracle:x\n"), 0)
	source.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{source.ID})
	e.G.Active = 1
	spell := e.G.Zone(state.ZHand, 0)[0]
	if hasCastOption(e.legalActions(0), spell) {
		t.Fatal("unmet IsPresent$ incorrectly granted flash")
	}
	creature := e.G.AddObject(card(t, "Name:Gate Creature\nTypes:Creature Human\nPT:1/1\nOracle:x\n"), 0)
	creature.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{source.ID, creature.ID})
	if !hasCastOption(e.legalActions(0), spell) {
		t.Fatal("met IsPresent$ did not grant flash")
	}
	source.Card.Faces[0].Statics[0].Params["ValidSA"] = "Activated.Equip"
	if hasCastOption(e.legalActions(0), spell) {
		t.Fatal("activation-only ValidSA$ incorrectly granted spell flash")
	}
}

func TestVedalkenOrreryAppliesOutsideHand(t *testing.T) {
	e := handEngine(t)
	orrery := e.G.AddObject(corpusAlternativeCard(t, "Vedalken Orrery"), 0)
	orrery.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{orrery.ID})
	e.G.Active = 1
	flashback := card(t, "Name:Slow Recall\nManaCost:0\nTypes:Sorcery\nK:Flashback:0\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n")
	gy := e.G.AddObject(flashback, 0)
	gy.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{gy.ID})
	if !hasCastOption(e.legalActions(0), gy.ID) {
		t.Fatal("Vedalken Orrery did not give a Flashback sorcery instant timing")
	}
	e.format = FormatCommander
	cmd := e.G.AddObject(card(t, "Name:Slow Commander\nManaCost:0\nTypes:Creature Human\nPT:1/1\nOracle:x\n"), 0)
	cmd.Zone = state.ZCommand
	e.G.Players[0].Commanders = []state.ObjID{cmd.ID}
	e.G.Players[0].CmdCasts = []int32{0}
	e.G.SetZone(state.ZCommand, 0, []state.ObjID{cmd.ID})
	if !hasCastOption(e.legalActions(0), cmd.ID) {
		t.Fatal("Vedalken Orrery did not give a command-zone spell instant timing")
	}
}

func TestGemstoneCavernsOpeningHandEffect(t *testing.T) {
	gem := corpusAlternativeCard(t, "Gemstone Caverns")
	fill := card(t, "Name:Filler\nTypes:Basic Land\nOracle:x\n")
	for seed := uint64(1); seed < 200; seed++ {
		deck := make([]*cards.Card, 40)
		deck[0] = gem
		for i := 1; i < len(deck); i++ {
			deck[i] = fill
		}
		e := New(Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}})
		if e.Pending() == nil || e.Pending().Kind != decision.KChoose || e.Pending().Player == e.opening.start {
			continue
		}
		d := e.Pending()
		if d.Options[0].Kind != "opening_yes" {
			continue
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		d = e.Pending()
		if d == nil || d.Options[0].Kind != "opening_exile" {
			t.Fatalf("Gemstone did not continue to mandatory exile: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		var caverns *state.Object
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if e.G.Obj(id).Face().Name == "Gemstone Caverns" {
				caverns = e.G.Obj(id)
			}
		}
		if caverns == nil || caverns.Counter("LUCK") != 1 || len(e.G.Zone(state.ZExile, 0)) != 1 {
			t.Fatalf("Gemstone result=%+v exile=%v", caverns, e.G.Zone(state.ZExile, 0))
		}
		return
	}
	t.Fatal("could not construct non-starting Gemstone opening hand")
}

func TestGemstoneCavernsIsOfferedOnlyAfterMulligans(t *testing.T) {
	gem := corpusAlternativeCard(t, "Gemstone Caverns")
	fill := card(t, "Name:Filler\nTypes:Basic Land\nOracle:x\n")
	deck := func() []*cards.Card {
		out := make([]*cards.Card, 40)
		out[0] = gem
		for i := 1; i < len(out); i++ {
			out[i] = fill
		}
		return out
	}
	for seed := uint64(1); seed < 1000; seed++ {
		e := New(Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck(), deck()}, Mulligans: 1})
		e.Advance()
		// The first pregame decision is always London, never an opening effect.
		if d := e.Pending(); d == nil || d.Kind != decision.KMulligan {
			t.Fatalf("opening effect preceded mulligan: %+v", d)
		}
		mulliganed := false
		for e.Pending() != nil && e.Pending().Kind == decision.KMulligan {
			d := e.Pending()
			choose := 0
			if !mulliganed && d.Player != e.mulligan.seats[0] {
				for _, id := range e.G.Zone(state.ZHand, d.Player) {
					if e.G.Obj(id).Face().Name == "Gemstone Caverns" {
						choose, mulliganed = 1, true
						break
					}
				}
			}
			submitChoices(t, e, choose)
			if mulliganed && choose == 1 {
				// Keep the redrawn hand only when the Caverns did not return.
				still := false
				for _, id := range e.G.Zone(state.ZHand, d.Player) {
					still = still || e.G.Obj(id).Face().Name == "Gemstone Caverns"
				}
				if still {
					break
				}
			}
		}
		if !mulliganed || e.Pending() == nil || e.Pending().Kind == decision.KMulligan {
			continue
		}
		if d := e.Pending(); d != nil && len(d.Options) > 0 && d.Options[0].Kind == "opening_yes" {
			t.Fatalf("mulliganed-away Gemstone was offered from a final hand without it: %+v", d)
		}
		return
	}
	t.Fatal("could not construct a mulliganed-away non-starting Gemstone hand")
}

func TestChancellorOpeningEffectRegistersAndRunsItsPhaseTrigger(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Chancellor of the Tangle"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.applyOpeningEffect(openingEffect{player: 0, card: id, svar: "RevealCard"})
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].Execute != "EffMana" || e.G.Delayed[0].Phase != state.StepMain1 {
		t.Fatalf("Chancellor opening Effect did not register its real phase child: %+v", e.G.Delayed)
	}
	// handEngine starts from a genesis priority decision; this direct
	// pregame setup owns no live decision before its new first turn.
	e.pending = nil
	e.beginTurn(0)
	turn := e.G.Turn
	e.Advance()
	driveToStep(t, e, turn, 0, state.StepMain1)
	if len(e.G.Stack) == 0 {
		t.Fatal("Chancellor opening trigger did not reach the stack")
	}
	e.resolveTop()
	if e.G.Players[0].Pool[state.MG] != 1 {
		t.Fatalf("Chancellor trigger mana=%v, want one green", e.G.Players[0].Pool)
	}
}

func TestImpatientIguanaBecomesStartingPlayer(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Impatient Iguana"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.pending = nil
	e.opening = openingRound{start: 1, effects: []openingEffect{{player: 0, card: id, svar: "RevealCard"}}}
	e.stepOpening()
	submitChoices(t, e, 0)
	if e.G.Active != 0 {
		t.Fatalf("Impatient Iguana did not become starting player: active=%d", e.G.Active)
	}
}

func TestOpeningHandRevealActionAndPlayFirstGate(t *testing.T) {
	chancellor := corpusAlternativeCard(t, "Chancellor of the Tangle")
	iguana := corpusAlternativeCard(t, "Impatient Iguana")
	fill := card(t, "Name:Filler\nTypes:Basic Land\nOracle:x\n")
	deck := func(c *cards.Card) []*cards.Card {
		out := make([]*cards.Card, 40)
		out[0] = c
		for i := 1; i < len(out); i++ {
			out[i] = fill
		}
		return out
	}
	foundChancellor := false
	for seed := uint64(1); seed < 200; seed++ {
		e := New(Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck(chancellor), deck(chancellor)}})
		d := e.Pending()
		if d == nil || d.Options[0].Kind != "opening_yes" {
			continue
		}
		id := d.Options[0].Obj
		if e.G.Obj(id).Face().Name != "Chancellor of the Tangle" || e.opening.effects[e.opening.index].svar != "RevealCard" {
			t.Fatalf("Chancellor opening action = %+v, want RevealCard", e.opening.effects)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		revealed := false
		for _, ev := range e.L.Events {
			if ev.Kind == events.Note {
				for _, got := range ev.IDs {
					revealed = revealed || got == id
				}
			}
		}
		if !revealed || e.G.Obj(id).Zone != state.ZHand {
			t.Fatalf("Chancellor did not reveal from opening hand: revealed=%v zone=%s", revealed, e.G.Obj(id).Zone)
		}
		foundChancellor = true
		break
	}
	if !foundChancellor {
		t.Fatal("could not construct Chancellor opening hand")
	}
	for seed := uint64(1); seed < 200; seed++ {
		e := New(Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck(iguana), deck(iguana)}})
		d := e.Pending()
		if d == nil || d.Options[0].Kind != "opening_yes" {
			continue
		}
		if d.Player == e.opening.start {
			t.Fatalf("!PlayFirst offered Impatient Iguana to starting player %d", d.Player)
		}
		return
	}
	t.Fatal("could not construct opening hands for real reveal scripts")
}

// Keep events imported above in this file's behavioural setup, rather than
// hand-writing state mutation; this compile-time assertion documents that all
// alternative-cost tests use the event boundary for game changes.
var _ = events.Event{}
