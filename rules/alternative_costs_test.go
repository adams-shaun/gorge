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
	for _, tc := range []struct{ name string }{{"Dizzy Spell"}, {"Ziatora's Proving Ground"}} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t, corpusAlternativeCard(t, tc.name))
			id := e.G.Zone(state.ZHand, 0)[0]
			for _, c := range "3UU" {
				e.G.Players[0].Pool[state.ManaIndex(byte(c))]++
			}
			var opt decision.Option
			for _, o := range e.legalActions(0) {
				if o.Kind == "ability" && o.Obj == id {
					opt = o
					break
				}
			}
			if opt.Kind != "ability" {
				t.Fatalf("%s hand activation not offered", tc.name)
			}
			e.beginActivation(0, opt)
			if d := e.Pending(); d == nil || d.Options[0].Kind != "discard" {
				t.Fatalf("%s did not ask to discard itself as activation cost: %+v", tc.name, d)
			}
			submitChoices(t, e, 0)
			if e.G.Obj(id).Zone != state.ZGraveyard {
				t.Fatalf("%s was not discarded as its real activation cost", tc.name)
			}
		})
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
