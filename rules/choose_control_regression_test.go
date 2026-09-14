package rules

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// choiceCorpus is the corpus registry opened once for this file's tests:
// each open decodes the whole compiled corpus (~0.25s), and a registry is
// read-only after load.
var choiceCorpus struct {
	sync.Mutex
	reg *cards.Registry
}

func choiceCorpusRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	choiceCorpus.Lock()
	// Deferred so a Skip/Fatal inside CorpusRegistry (runtime.Goexit)
	// cannot leave the lock held for the next test.
	defer choiceCorpus.Unlock()
	if choiceCorpus.reg == nil {
		choiceCorpus.reg = testutil.CorpusRegistry(t)
	}
	return choiceCorpus.reg
}

func choiceCorpusCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	reg := choiceCorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("missing corpus card %q", name)
	}
	return c
}

func submitChoicePass(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("wanted priority, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "pass" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no pass option: %+v", d.Options)
}

// TestSowerOfDiscordETBChoicesResume is an end-to-end corpus regression for
// the Updated ETB replacement path. The source has left the stack before its
// replacement asks, so this catches a resume accidentally bound to stack id 0.
// TestPlanetaryAnnihilationEachPlayerChoosesOwnLand proves that Defined$ Player
// is a persisted per-chooser continuation and ControlledByPlayer$ Chooser
// filters the option list before it reaches each player.
func TestPlanetaryAnnihilationEachPlayerChoosesOwnLand(t *testing.T) {
	decks := make([][]*cards.Card, 4)
	for i := range decks {
		decks[i] = mountainDeck(t, 40)
	}
	e := New(Config{Seed: 712, Names: []string{"a", "b", "c", "d"}, Decks: decks})
	planet := e.G.AddObject(choiceCorpusCard(t, "Planetary Annihilation"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: planet.ID, From: state.ZLibrary, To: state.ZStack})
	lands := make([]state.ObjID, 4)
	for p := range lands {
		o := e.G.AddObject(card(t, "Name:Land\nTypes:Land\nOracle:x\n"), state.PlayerID(p))
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		lands[p] = o.ID
	}
	ctx := &effects.Ctx{Source: planet.ID, Controller: 0}
	effects.SetSVars(ctx, planet.Face().SVars)
	effects.Resolve(e, ctx, planet.Face().SpellAbility())
	for p := range lands {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Player != state.PlayerID(p) || len(d.Options) != 1 || d.Options[0].Obj != lands[p] {
			t.Fatalf("chooser %d saw %+v, want only land %d", p, d, lands[p])
		}
		submitChoices(t, e, d.Options[0].Index)
	}
}

// TestFlayerTemporaryControlExpiresAndZoneChangeResetsControl covers both
// GainControl's LoseControl$ EOT contract and CR 400.7's new-object control
// reset after a stolen permanent changes zones.
// TestCommandeerChangesTargetAfterAnsweredChoice drives ChangeTargets through a
// real suspended answer; hostless fallback must not be mistaken for a target
// rewrite.
func TestCommandeerChangesTargetAfterAnsweredChoice(t *testing.T) {
	decks := [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}
	e := New(Config{Seed: 714, Names: []string{"a", "b"}, Decks: decks})
	target := e.G.AddObject(card(t, "Name:Burn\nTypes:Instant\nManaCost:R\nA:SP$ DealDamage | ValidTgts$ Player | NumDmg$ 1\nOracle:x\n"), 1)
	commandeer := e.G.AddObject(choiceCorpusCard(t, "Commandeer"), 0)
	for _, id := range []state.ObjID{target.ID, commandeer.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZStack})
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: target.ID, Player: 1, Amount: 1})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: commandeer.ID, IDs: []state.ObjID{target.ID}})
	sa := cards.ResolveSVar(commandeer.Face().SVars, "DBChooseTargets")
	effects.Resolve(e, &effects.Ctx{Source: commandeer.ID, Controller: 0, Targets: commandeer.Targets, SVars: commandeer.Face().SVars}, sa)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) < 2 {
		t.Fatalf("ChangeTargets did not ask: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 0 {
			submitChoices(t, e, o.Index)
			break
		}
	}
	if got := target.Targets; len(got) != 1 || !got[0].IsPlayer || got[0].Player != 0 {
		t.Fatalf("Commandeer target = %+v, want player 0", got)
	}
}

func TestVialSmasherChosenPlayerTakesDamage(t *testing.T) {
	e := New(Config{Seed: 715, Names: []string{"a", "b", "c"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
	vial := e.G.AddObject(choiceCorpusCard(t, "Vial Smasher the Fierce"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: vial.ID, From: state.ZLibrary, To: state.ZBattlefield})
	sa := cards.ResolveSVar(vial.Face().SVars, "TrigChoose")
	if sa == nil || sa.Sub == nil {
		t.Fatalf("Vial Smasher chain not linked: %+v", sa)
	}
	svars := make(map[string]string, len(vial.Face().SVars))
	for k, v := range vial.Face().SVars {
		svars[k] = v
	}
	// TriggeredSpellAbility$CardManaCostLKI is a separate count-expression
	// gap; hold the real choice/damage chain constant at a known amount here.
	svars["X"] = "4"
	ctx := &effects.Ctx{Source: vial.ID, Controller: 0, SVars: svars}
	effects.Resolve(e, ctx, sa)
	if e.G.Players[1].Life != 16 || e.G.Players[2].Life != 20 {
		t.Fatalf("Vial Smasher life = [%d %d], want [16 20] (ctx chosen=%+v state chosen=%+v)", e.G.Players[1].Life, e.G.Players[2].Life, ctx.Chosen, vial.Chosen)
	}
}

func TestWishclawChosenPlayerGainsControl(t *testing.T) {
	e := New(Config{Seed: 716, Names: []string{"a", "b"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	wish := e.G.AddObject(choiceCorpusCard(t, "Wishclaw Talisman"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: wish.ID, From: state.ZLibrary, To: state.ZBattlefield})
	sa := cards.ResolveSVar(wish.Face().SVars, "DBChoose")
	e.emit(events.Event{Kind: events.AbilityPush, Obj: wish.ID, Player: 0, Amount: 0})
	effects.Resolve(e, &effects.Ctx{Source: wish.ID, Controller: 0, SVars: wish.Face().SVars}, sa)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 || d.Options[0].Player != 1 {
		t.Fatalf("Wishclaw player choice = %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	if wish.Controller != 1 {
		t.Fatalf("Wishclaw controller = %d, want 1", wish.Controller)
	}
}

func TestReboundTargetRestrictionOnlyOffersPlayers(t *testing.T) {
	e := New(Config{Seed: 717, Names: []string{"a", "b"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	spell := e.G.AddObject(card(t, "Name:Flexible\nTypes:Instant\nManaCost:R\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"), 1)
	rebound := e.G.AddObject(choiceCorpusCard(t, "Rebound"), 0)
	creature := e.G.AddObject(card(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	for _, id := range []state.ObjID{spell.ID, rebound.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZStack})
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: creature.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: spell.ID, Player: 0, Amount: 1})
	effects.Resolve(e, &effects.Ctx{Source: rebound.ID, Controller: 0, Targets: []state.Target{{Obj: spell.ID}}, SVars: rebound.Face().SVars}, rebound.Face().SpellAbility())
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("Rebound did not ask: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind != "player" {
			t.Fatalf("Rebound offered non-player target %+v (creature %d)", o, creature.ID)
		}
	}
}

func TestFlayerTemporaryControlExpiresAndZoneChangeResetsControl(t *testing.T) {
	decks := [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}
	e := New(Config{Seed: 713, Names: []string{"a", "b"}, Decks: decks})
	flayer := e.G.AddObject(choiceCorpusCard(t, "Flayer of Loyalties"), 0)
	target := e.G.AddObject(card(t, "Name:Target\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	for _, id := range []state.ObjID{flayer.ID, target.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	}
	sa := cards.ResolveSVar(flayer.Face().SVars, "TrigGainControl")
	effects.Resolve(e, &effects.Ctx{Source: flayer.ID, Controller: 0, Targets: []state.Target{{Obj: target.ID}}, SVars: flayer.Face().SVars}, sa)
	if target.Controller != 0 {
		t.Fatalf("Flayer did not gain control: %d", target.Controller)
	}
	e.EndOfTurnCleanup()
	if target.Controller != 1 {
		t.Fatalf("temporary control did not expire: %d", target.Controller)
	}
	e.emit(events.Event{Kind: events.ControlChange, Obj: target.ID, Player: 0})
	e.emit(events.Event{Kind: events.MoveZone, Obj: target.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: target.ID, From: state.ZGraveyard, To: state.ZBattlefield})
	if target.Controller != 1 {
		t.Fatalf("zone change retained stolen control: %d", target.Controller)
	}
}

func TestSowerOfDiscordETBChoicesResume(t *testing.T) {
	e, _, _ := etbConfig(t, 711, nil, nil)
	sower := e.G.AddObject(choiceCorpusCard(t, "Sower of Discord"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: sower.ID, From: state.ZLibrary, To: state.ZHand})
	addMana(t, e, 0, "BBBBBB")

	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == sower.ID {
			submitChoices(t, e, o.Index)
			break
		}
	}
	for e.Pending() != nil && e.Pending().Kind == decision.KPriority {
		submitChoicePass(t, e)
	}
	if d = e.Pending(); d == nil || d.Kind != decision.KChoose || d.Source != sower.ID {
		t.Fatalf("Sower ETB did not ask: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	if d = e.Pending(); d == nil || d.Kind != decision.KChoose || d.Source != sower.ID {
		t.Fatalf("Sower second ETB choice did not resume: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)

	o := e.G.Obj(sower.ID)
	if o.Zone != state.ZBattlefield || len(o.Remembered) != 1 || len(o.Chosen) != 1 ||
		o.Remembered[0].Player == o.Chosen[0].Player {
		t.Fatalf("Sower choices not persisted separately: %+v", o)
	}
}

// abilityIndex returns the index of the first face ability with api.
func abilityIndex(t *testing.T, c *cards.Card, api string) int {
	t.Helper()
	for i, sa := range c.Faces[0].Abilities {
		if sa.API == api {
			return i
		}
	}
	t.Fatalf("%s has no %s ability", c.Faces[0].Name, api)
	return -1
}

// TestValleymakerChoosePlayerOffersEveryPlayer is the no-Choices$,
// no-ValidTgts$ ChoosePlayer shape: "Choose a player" offers every living
// player, and the chosen player -- not the controller -- adds the mana.
func TestValleymakerChoosePlayerOffersEveryPlayer(t *testing.T) {
	e := New(Config{Seed: 718, Names: []string{"a", "b", "c"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
	vm := e.G.AddObject(choiceCorpusCard(t, "Valleymaker"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: vm.ID, From: state.ZLibrary, To: state.ZBattlefield})
	idx := abilityIndex(t, vm.Card, "ChoosePlayer")
	e.emit(events.Event{Kind: events.AbilityPush, Obj: vm.ID, Player: 0, Amount: int32(idx)})
	effects.Resolve(e, &effects.Ctx{Source: vm.ID, Controller: 0, SVars: vm.Face().SVars}, vm.Face().Abilities[idx])
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Player != 0 || d.Min != 1 || d.Max != 1 || len(d.Options) != 3 {
		t.Fatalf("Valleymaker choice = %+v, want controller choosing one of three players", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 2 {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("player 2 not offered: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	if got := e.G.Players[2].Pool[state.MG]; got != 3 {
		t.Fatalf("chosen player's green mana = %d, want 3", got)
	}
	if got := e.G.Players[0].Pool[state.MG]; got != 0 {
		t.Fatalf("controller's green mana = %d, want 0", got)
	}
}

// TestOnlyBloodRepeatEachResumesEachOpponentsDiscard drives a real RepeatEach
// whose iteration asks through the engine's resolution: each opponent in
// turn chooses two cards to discard. The first answer must resume that
// opponent's discard with the loop's player still bound (Defined$
// Player.IsRemembered), and then continue the loop to the second opponent
// instead of dropping it.
func TestOnlyBloodRepeatEachResumesEachOpponentsDiscard(t *testing.T) {
	e := New(Config{Seed: 719, Names: []string{"a", "b", "c"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
	scheme := e.G.AddObject(choiceCorpusCard(t, "Only Blood Ends Your Nightmares"), 0)
	hands := map[state.PlayerID][]state.ObjID{}
	graveyards := map[state.PlayerID]int{}
	for _, p := range []state.PlayerID{1, 2} {
		hands[p] = append([]state.ObjID(nil), e.G.Zone(state.ZHand, p)...)
		graveyards[p] = len(e.G.Zone(state.ZGraveyard, p))
		if len(hands[p]) < 3 {
			t.Fatalf("player %d opening hand = %d cards, want at least 3", p, len(hands[p]))
		}
	}
	e.emit(events.Event{Kind: events.TriggerPush, Obj: scheme.ID, Player: 0, Amount: 0})
	e.resolveTop()
	for _, p := range []state.PlayerID{1, 2} {
		d := e.Pending()
		if d == nil || d.Kind != decision.KModes || d.Player != p || d.Min != 2 || len(d.Options) != len(hands[p]) {
			t.Fatalf("iteration for player %d asked %+v", p, d)
		}
		for _, o := range d.Options {
			if o.Player != p || !containsObj(hands[p], o.Obj) {
				t.Fatalf("player %d offered a card outside their hand: %+v", p, o)
			}
		}
		submitChoices(t, e, 0, 1)
	}
	for _, p := range []state.PlayerID{1, 2} {
		if got, gy := len(e.G.Zone(state.ZHand, p)), len(e.G.Zone(state.ZGraveyard, p)); got != len(hands[p])-2 || gy != graveyards[p]+2 {
			t.Fatalf("player %d hand/graveyard = %d/%d, want %d/%d", p, got, gy, len(hands[p])-2, graveyards[p]+2)
		}
	}
	if d := e.Pending(); d != nil && (d.Kind == decision.KModes || d.Kind == decision.KChoose) {
		t.Fatalf("loop asked again after every opponent answered: %+v", d)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("resolved RepeatEach left the stack non-empty: %v", e.G.Stack)
	}
}

func containsObj(ids []state.ObjID, id state.ObjID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// stealWith resolves a real GainControl SA from src's face against target.
func stealWith(t *testing.T, e *Engine, src *state.Object, sa *cards.SA, controller state.PlayerID, target state.ObjID) {
	t.Helper()
	effects.Resolve(e, &effects.Ctx{Source: src.ID, Controller: controller, Targets: []state.Target{{Obj: target}}, SVars: src.Face().SVars}, sa)
}

func controlBoard(t *testing.T, seed uint64, thief string) (*Engine, *state.Object, *state.Object) {
	t.Helper()
	e := New(Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	src := e.G.AddObject(choiceCorpusCard(t, thief), 0)
	victim := e.G.AddObject(card(t, "Name:Victim\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	for _, id := range []state.ObjID{src.ID, victim.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	}
	return e, src, victim
}

// TestVedalkenShacklesControlEndsWhenSourceUntaps covers LoseControl$
// Untap,LeavesPlay: control lasts exactly as long as the source stays tapped,
// and CR 611.2b makes the steal do nothing if the source is already untapped.
func TestVedalkenShacklesControlEndsWhenSourceUntaps(t *testing.T) {
	e, shackles, victim := controlBoard(t, 720, "Vedalken Shackles")
	sa := shackles.Face().Abilities[abilityIndex(t, shackles.Card, "GainControl")]
	stealWith(t, e, shackles, sa, 0, victim.ID)
	if victim.Controller != 1 {
		t.Fatalf("untapped Shackles stole the creature (CR 611.2b): controller %d", victim.Controller)
	}
	e.emit(events.Event{Kind: events.Tap, Obj: shackles.ID})
	stealWith(t, e, shackles, sa, 0, victim.ID)
	if victim.Controller != 0 {
		t.Fatalf("tapped Shackles did not steal: controller %d", victim.Controller)
	}
	e.EndOfTurnCleanup()
	if victim.Controller != 0 {
		t.Fatalf("Shackles control ended at cleanup while still tapped: controller %d", victim.Controller)
	}
	e.emit(events.Event{Kind: events.Untap, Obj: shackles.ID})
	if victim.Controller != 1 {
		t.Fatalf("control survived Shackles untapping: controller %d", victim.Controller)
	}
}

// TestKelloggControlEndsWhenSourceLeavesOrChangesHands covers LoseControl$
// LeavesPlay,LoseControl ("for as long as you control Kellogg") and the
// layering of overlapping control effects: an earlier until-end-of-turn
// steal that ends while Kellogg's is live changes nothing, and Kellogg's end
// returns the creature to the controller it had before either effect.
func TestKelloggControlEndsWhenSourceLeavesOrChangesHands(t *testing.T) {
	e, kellogg, victim := controlBoard(t, 721, "Kellogg, Dangerous Mind")
	sa := kellogg.Face().Abilities[abilityIndex(t, kellogg.Card, "GainControl")]
	stealWith(t, e, kellogg, sa, 0, victim.ID)
	if victim.Controller != 0 {
		t.Fatalf("Kellogg did not steal: controller %d", victim.Controller)
	}
	e.EndOfTurnCleanup()
	if victim.Controller != 0 {
		t.Fatalf("Kellogg's control ended at cleanup: controller %d", victim.Controller)
	}
	e.emit(events.Event{Kind: events.ControlChange, Obj: kellogg.ID, Player: 1})
	if victim.Controller != 1 {
		t.Fatalf("control survived Kellogg changing hands: controller %d", victim.Controller)
	}

	e, kellogg, victim = controlBoard(t, 722, "Kellogg, Dangerous Mind")
	flayer := e.G.AddObject(choiceCorpusCard(t, "Flayer of Loyalties"), 0)
	stealWith(t, e, flayer, cards.ResolveSVar(flayer.Face().SVars, "TrigGainControl"), 0, victim.ID)
	stealWith(t, e, kellogg, sa, 0, victim.ID)
	e.EndOfTurnCleanup()
	if victim.Controller != 0 {
		t.Fatalf("earlier EOT steal expiring overrode Kellogg's later effect: controller %d", victim.Controller)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: kellogg.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	if victim.Controller != 1 {
		t.Fatalf("control survived Kellogg leaving play: controller %d", victim.Controller)
	}
}

// TestPowerOfPersuasionControlLastsThroughYourNextTurn covers LoseControl$
// UntilTheEndOfYourNextTurn: control survives this turn's cleanup and the
// opponent's turn, and ends at the cleanup of the controller's next turn.
func TestPowerOfPersuasionControlLastsThroughYourNextTurn(t *testing.T) {
	e := New(Config{Seed: 723, Names: []string{"a", "b"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	spell := e.G.AddObject(choiceCorpusCard(t, "Power of Persuasion"), 0)
	victim := e.G.AddObject(card(t, "Name:Victim\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: victim.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 1})
	stealWith(t, e, spell, cards.ResolveSVar(spell.Face().SVars, "DBControl"), 0, victim.ID)
	for turn, active := range []state.PlayerID{0, 1} {
		if turn > 0 {
			e.emit(events.Event{Kind: events.TurnChange, Player: active, Amount: int32(turn + 1)})
		}
		e.EndOfTurnCleanup()
		if victim.Controller != 0 {
			t.Fatalf("control ended at the cleanup of turn %d: controller %d", turn+1, victim.Controller)
		}
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 3})
	e.EndOfTurnCleanup()
	if victim.Controller != 1 {
		t.Fatalf("control survived the end of the controller's next turn: controller %d", victim.Controller)
	}
}

// TestOldManOfTheSeaStaticCommandCheck covers LoseControl$ StaticCommandCheck:
// the stolen creature returns once its power exceeds Old Man's power.
func TestOldManOfTheSeaStaticCommandCheck(t *testing.T) {
	e, oldMan, victim := controlBoard(t, 724, "Old Man of the Sea")
	sa := oldMan.Face().Abilities[abilityIndex(t, oldMan.Card, "GainControl")]
	e.emit(events.Event{Kind: events.Tap, Obj: oldMan.ID})
	stealWith(t, e, oldMan, sa, 0, victim.ID)
	if victim.Controller != 0 {
		t.Fatalf("Old Man did not steal a 1/1: controller %d", victim.Controller)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: victim.ID, Counter: "P1P1", Amount: 1})
	if victim.Controller != 0 {
		t.Fatalf("control ended while power 2 <= Old Man's 2: controller %d", victim.Controller)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: victim.ID, Counter: "P1P1", Amount: 1})
	if victim.Controller != 1 {
		t.Fatalf("control survived power 3 > Old Man's 2: controller %d", victim.Controller)
	}
}

// TestControlEndsAtEndOfCombatAndWhenAuraUnattaches covers the two remaining
// LoseControl$ terms with their real scripts. Their triggers' bindings are
// supplied directly, because the trigger modes themselves (AttackersDeclared
// with AttackingPlayer$, Attached) and Defined$ TriggeredAttackingPlayer /
// TriggeredTarget are not modelled: Tahngarth's effect is resolved as its
// attacking opponent controls it, and Eriette's with the Aura as the
// triggering source and the enchanted permanent as its target.
func TestControlEndsAtEndOfCombatAndWhenAuraUnattaches(t *testing.T) {
	e, tahngarth, _ := controlBoard(t, 725, "Tahngarth, First Mate")
	sa := cards.ResolveSVar(tahngarth.Face().SVars, "TrigGainControl")
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	effects.Resolve(e, &effects.Ctx{Source: tahngarth.ID, Controller: 1, SVars: tahngarth.Face().SVars}, sa)
	if tahngarth.Controller != 1 {
		t.Fatalf("Tahngarth not given to the attacking player: controller %d", tahngarth.Controller)
	}
	e.setStep(state.StepEndCombat)
	if tahngarth.Controller != 1 {
		t.Fatalf("control ended before the end of combat step ended: controller %d", tahngarth.Controller)
	}
	e.setStep(state.StepMain2)
	if tahngarth.Controller != 0 {
		t.Fatalf("control survived the end of combat: controller %d", tahngarth.Controller)
	}

	e, eriette, victim := controlBoard(t, 726, "Eriette, the Beguiler")
	aura := e.G.AddObject(card(t, "Name:Aura\nTypes:Enchantment Aura\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: aura.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Attach, Obj: aura.ID, IDs: []state.ObjID{victim.ID}})
	sa = cards.ResolveSVar(eriette.Face().SVars, "TrigGainControl")
	ctx := &effects.Ctx{Source: eriette.ID, Controller: 0, Targets: []state.Target{{Obj: victim.ID}}, SVars: eriette.Face().SVars}
	ctx.TriggerSource = aura.ID
	effects.Resolve(e, ctx, sa)
	if victim.Controller != 0 {
		t.Fatalf("Eriette did not gain control: controller %d", victim.Controller)
	}
	e.EndOfTurnCleanup()
	if victim.Controller != 0 {
		t.Fatalf("Aura-bound control ended at cleanup: controller %d", victim.Controller)
	}
	e.emit(events.Event{Kind: events.Attach, Obj: aura.ID})
	if victim.Controller != 1 {
		t.Fatalf("control survived the Aura becoming unattached: controller %d", victim.Controller)
	}
}

// TestChaosDefilerRemembersEveryAskedIteration drives a real RepeatEach whose
// iterations ask (each opponent's ChooseCard with RememberChosen$) and whose
// Sub reads what the loop remembered. Every iteration here completes on a
// resumed frame, so each choice reaches the post-loop random choice only if
// the resumed iteration hands its Remembered back to the loop frame. The
// seed is fixed so the random pick lands on the FIRST opponent's permanent,
// which is in the pool only through that hand-over.
func TestChaosDefilerRemembersEveryAskedIteration(t *testing.T) {
	e := New(Config{Seed: 702, Names: []string{"a", "b", "c"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
	defiler := e.G.AddObject(choiceCorpusCard(t, "Chaos Defiler"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: defiler.ID, From: state.ZLibrary, To: state.ZBattlefield})
	relics := map[state.PlayerID]state.ObjID{}
	for _, p := range []state.PlayerID{1, 2} {
		o := e.G.AddObject(card(t, "Name:Relic\nTypes:Artifact\nOracle:x\n"), p)
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		relics[p] = o.ID
	}
	e.emit(events.Event{Kind: events.TriggerPush, Obj: defiler.ID, Player: 0, Amount: 0})
	e.resolveTop()
	for _, p := range []state.PlayerID{1, 2} {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Player != 0 || len(d.Options) != 1 || d.Options[0].Obj != relics[p] {
			t.Fatalf("iteration for opponent %d asked %+v, want only relic %d", p, d, relics[p])
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if z1, z2 := e.G.Obj(relics[1]).Zone, e.G.Obj(relics[2]).Zone; z1 != state.ZGraveyard || z2 != state.ZBattlefield {
		t.Fatalf("relic zones = [%v %v], want opponent 1's destroyed and opponent 2's kept", z1, z2)
	}
}
