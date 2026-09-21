package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// This file pins CR 702.140d's remaining halves: a mutated pile's UNDER-CARD
// activated abilities, mana abilities and statics are live exactly as the top
// card's are ("It has all abilities of the cards beneath it"). Before the
// pile view landed, every reader walked o.Face() -- always the pile's TOP
// card -- so an ability or static printed on a card beneath the top was
// invisible. The tests mutate a carrier UNDER a foreign top card so the
// ability under test is unambiguously the under-card's.

// --- CR 702.140d: an under-card ACTIVATED ability ---

// TestMutatedPilePorcuparrotUnderOffersAndResolvesActivatedAbility is the
// activated-ability half: Porcuparrot ({T}: this creature deals X damage to
// any target, X = times this creature has mutated) mutated UNDER a Bear. The
// pile's top card is the vanilla Bear, so the ability is only reachable
// through the under-card. The {T} ability must be OFFERED and must deal the
// under-card's own X=1 (Count$TimesMutated on Porcuparrot's face).
func TestMutatedPilePorcuparrotUnderOffersAndResolvesActivatedAbility(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	parrot := mustCorpusCard(t, reg, "Porcuparrot")
	e, cfg := tokenReplGame(t, 401, parrot)
	parrotID := moveSeededCard(t, e, 0, parrot, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "RRR") // the mutate cost {2}{R}

	// Place the Parrot UNDER the Bear: the pile's top card stays the vanilla
	// Bear, so the only activated ability the pile has is Porcuparrot's.
	mutateCastOnto(t, e, mutatedCastOption(t, e, parrotID), bear, false)
	mutateDrain(t, e, 40)

	pile := e.G.Obj(bear)
	if pile == nil || pile.Face() == nil || pile.Face().Name != "Mutate Bear" {
		t.Fatalf("pile top card = %+v, want the Bear on top", pile)
	}
	if pile.TimesMutated != 1 {
		t.Fatalf("TimesMutated = %d, want 1", pile.TimesMutated)
	}

	// The pile's {T} cost needs a non-summoning-sick source (CR 302.6): the
	// Bear entered THIS turn, so advance past it (two turns keeps seat 0
	// active, the boast_test convention) before the ability is offered.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 2})

	// The under-card ability is a flat pile index past the (empty) top-face
	// ability list, so it is a real "ability" option on the pile object.
	addMana(t, e, 0, "") // fresh priority decision
	opt := abilityOption(t, e, bear, 0)

	lifeBefore := e.G.Players[1].Life
	submitChoices(t, e, opt.Index)
	// The ability's target ask (ValidTgts$ Any) -- answer the opponent's face.
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Porcuparrot's ability did not ask for its target: %+v", d)
	}
	opp := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			opp = o.Index
		}
	}
	if opp < 0 {
		t.Fatalf("target ask offers no opponent face: %+v", d.Options)
	}
	submitChoices(t, e, opp)
	passUntilStackEmpty(t, e, 40)

	if got := lifeBefore - e.G.Players[1].Life; got != 1 {
		t.Fatalf("under-card ability dealt %d damage, want X = TimesMutated = 1", got)
	}
	replayCheck(t, e, cfg)
}

// --- CR 702.140d: an under-card MANA ability ---

// mutateManaDorkSrc is an authored fixture mutate creature carrying a mana
// ability (never a corpus .txt, per the licensing rule). No corpus mutate
// card prints a mana ability, so a fixture is the only way to pin the
// under-card mana path; it is a real compiled keyword + AB$ Mana, so the
// engine's own mana-ability collector and payment path are what run.
const mutateManaDorkSrc = "Name:Mana Mutant\nManaCost:1 G\nTypes:Creature Elf Druid\nPT:1/1\n" +
	"K:Mutate:1 G\n" +
	"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.\n" +
	"Oracle:x\n"

// TestMutatedPileUnderCardManaAbilityIsOfferedAndProduces pins the mana-ability half: a
// mana dork mutated UNDER a Bear. The pile's top card is the Bear, so the
// {T}: add {G} ability is only the under-card's. The tap-for-mana option
// ("activate") must be offered for the pile and add green mana to the pool.
func TestMutatedPileUnderCardManaAbilityIsOfferedAndProduces(t *testing.T) {
	dork := card(t, mutateManaDorkSrc)
	e, cfg := tokenReplGame(t, 402, dork)
	dorkID := moveSeededCard(t, e, 0, dork, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "GG") // the mutate cost {1}{G}

	mutateCastOnto(t, e, mutatedCastOption(t, e, dorkID), bear, false)
	mutateDrain(t, e, 40)

	pile := e.G.Obj(bear)
	if pile == nil || pile.Face() == nil || pile.Face().Name != "Mutate Bear" {
		t.Fatalf("pile top card = %+v, want the Bear on top", pile)
	}

	// The pile's {T} tap cost needs a non-summoning-sick source (CR 302.6):
	// the Bear entered THIS turn, so advance past it (two turns keeps seat 0
	// active, the boast_test convention) before the tap is offered.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 2})

	// The under-card mana source must be offered as an "activate" option.
	addMana(t, e, 0, "") // fresh priority decision
	before := poolTotal(e.G.Players[0].Pool)
	idx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "activate" && o.Obj == bear {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("under-card mana ability not offered on the pile: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)

	after := poolTotal(e.G.Players[0].Pool)
	if after-before != 1 {
		t.Fatalf("under-card mana ability added %d mana, want 1", after-before)
	}
	if e.G.Players[0].Pool[state.MG] != 1 {
		t.Fatalf("pool green = %d, want 1 (the under-card {G} ability)", e.G.Players[0].Pool[state.MG])
	}
	replayCheck(t, e, cfg)
}

// --- CR 702.140d: an under-card STATIC ---

// mutateLordSrc is an authored fixture mutate creature whose static pumps
// every OTHER creature you control by its own X, where X = times it has
// mutated (never a corpus .txt). The SVar-indirect amount is deliberate: the
// static runs through staticAmount and must read the UNDER-CARD's own SVar
// table (threaded on ContinuousEffect.SVars), not the pile top card's.
const mutateLordSrc = "Name:Mutant Lord\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\n" +
	"K:Mutate:1 G\n" +
	"S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddPower$ X | AddToughness$ X | Description$ Other creatures you control get +X/+X.\n" +
	"SVar:X:Count$TimesMutated\n" +
	"Oracle:x\n"

// TestMutatedPileUnderCardStaticLordPumpsWithItsOwnSVar pins the static half end to end:
// the Mutant Lord mutated UNDER a Bear, so the pile's top card is the Bear.
// Its "other creatures you control get +X/+X, X = times this mutated" static
// must be live and must resolve X against the UNDER-CARD's own SVar table --
// X = TimesMutated = 1, so a third creature you control is 2/2 (base) + 1/+1.
func TestMutatedPileUnderCardStaticLordPumpsWithItsOwnSVar(t *testing.T) {
	lord := card(t, mutateLordSrc)
	e, cfg := tokenReplGame(t, 403, lord)
	lordID := moveSeededCard(t, e, 0, lord, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	other := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "GG") // the mutate cost {1}{G}

	mutateCastOnto(t, e, mutatedCastOption(t, e, lordID), bear, false)
	mutateDrain(t, e, 40)

	pile := e.G.Obj(bear)
	if pile == nil || pile.Face() == nil || pile.Face().Name != "Mutate Bear" {
		t.Fatalf("pile top card = %+v, want the Bear on top", pile)
	}
	if pile.TimesMutated != 1 {
		t.Fatalf("TimesMutated = %d, want 1", pile.TimesMutated)
	}
	// The other Bear takes +1/+1 from the under-card lord, X read from the
	// under-card's own Count$TimesMutated.
	if p, tt := e.Power(other), e.Toughness(other); p != 3 || tt != 3 {
		t.Fatalf("other creature = %d/%d, want 3/3 (the under-card lord's own X = TimesMutated 1)", p, tt)
	}
	replayCheck(t, e, cfg)
}

// --- CR 903.3d / 702.140a: Brokkos, Apex of Forever ---

// mutateCastOntoPips is mutateCastOnto for a cost carrying a hybrid/Phyrexian
// pip (Brokkos' {2}{U/B}{G}{G}): CR 601.2b announces such a pip through its
// own KChoose (the manaAsk machinery), which the shared helper does not know
// about -- each intervening pip ask is answered with its first option (the
// deterministic single-candidate payment; the test's pool carries no B, so
// pay_U is the only face).
func mutateCastOntoPips(t *testing.T, e *Engine, opt decision.Option, bearer state.ObjID, onTop bool) {
	t.Helper()
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("mutate placement ask: %+v", d)
	}
	place := -1
	for _, o := range d.Options {
		if o.Kind == "mutate_place" && ((onTop && o.Label == "On top") || (!onTop && o.Label == "Under")) {
			place = o.Index
		}
	}
	if place < 0 {
		t.Fatalf("no placement option: %+v", d.Options)
	}
	submitChoices(t, e, place)
	// Answer every intervening mana-pip ask (CR 601.2b) before the target ask.
	for {
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			break
		}
		pip := false
		for _, o := range d.Options {
			if strings.HasPrefix(o.Kind, "pay_") {
				pip = true
				break
			}
		}
		if !pip {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("mutate target ask: %+v", d)
	}
	idx := indexOfObjOption(d, bearer)
	if idx < 0 {
		t.Fatalf("mutate target ask does not offer %d: %+v", bearer, d.Options)
	}
	submitChoices(t, e, idx)
}

// TestBrokkosMayPlayMutateFromTheGraveyard pins the may-play mute half on the
// real carrier: Brokkos, Apex of Forever's graveyard self-grant is
// `ValidSA$ Spell.Mutate`, so from the graveyard it offers ONLY the mutate
// cast (paying the mutate cost) and never a plain cast. A non-Human creature
// you own is required as the merge target (CR 702.140a).
func TestBrokkosMayPlayMutateFromTheGraveyard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	brokkos := mustCorpusCard(t, reg, "Brokkos, Apex of Forever")
	e, cfg := tokenReplGame(t, 404, brokkos)
	brokkosID := moveSeededCard(t, e, 0, brokkos, state.ZGraveyard)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	// The mutate cost {2}{U/B}{G}{G}: the pool carries U but no B, so the
	// hybrid {U/B} pip has a single payment candidate and the payment never
	// asks (the strict-supersets no-ask convention); generic and {G}{G} pay
	// from the greens and the spare blue.
	addMana(t, e, 0, "GGGGUU")

	opts := castOptions(t, e)
	var mutateOpt decision.Option
	sawMutate, sawPlain := false, false
	for _, o := range opts {
		if o.Obj != brokkosID {
			continue
		}
		switch o.Mode {
		case "mutated":
			sawMutate = true
			mutateOpt = o
		case "mayplay", "":
			sawPlain = true
		}
	}
	if !sawMutate {
		t.Fatalf("Brokkos' ValidSA$ Spell.Mutate grant did not offer the mutate cast from the graveyard: %+v", opts)
	}
	if sawPlain {
		t.Fatalf("Brokkos' mutate-only grant wrongly offered a plain may-play cast: %+v", opts)
	}

	mutateCastOntoPips(t, e, mutateOpt, bear, true)
	mutateDrain(t, e, 40)

	pile := e.G.Obj(bear)
	if pile == nil || pile.Zone != state.ZBattlefield || pile.Face() == nil ||
		pile.Face().Name != "Brokkos, Apex of Forever" {
		t.Fatalf("pile = %+v, want Brokkos on top of the bear", pile)
	}
	if o := e.G.Obj(brokkosID); o != nil && o.Zone == state.ZBattlefield {
		t.Fatal("Brokkos is still a separate battlefield permanent")
	}
	replayCheck(t, e, cfg)
}
