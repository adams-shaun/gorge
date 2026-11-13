package rules

// Search behaviour: the fail-to-find pin of a nonmandatory shuffle, the targeted
// gate's pre-ask, Yuna's Decision's each-leg, and compound zone origins and
// selectors.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSearchShuffleTailFailToFindShuffles: a hidden-library search that
// finds nothing still shuffles (CR 701.24d), and ShuffleNonMandatory$ does
// not turn that shuffle into a question.
func TestSearchShuffleTailFailToFindShuffles(t *testing.T) {
	t.Parallel()
	spell := "Name:Fruitless Search\nManaCost:U\nTypes:Sorcery\n" +
		"A:SP$ ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Card.namedNothing | ShuffleNonMandatory$ True\nOracle:x\n"
	e, cfg := fixtureGame(t, 171, cardsOfSrcs(t, spell), nil)
	moveNamedTo(t, e, 0, "Fruitless Search", state.ZHand)
	hand := len(e.G.Zone(state.ZHand, 0))
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Fruitless Search", "U", -1)
	if d != nil && d.ResumeKind == "search" {
		if len(d.Options) != 0 {
			t.Fatalf("a namedNothing search offered %+v", d.Options)
		}
		d = answerChoices(t, e)
	}
	if d != nil {
		t.Fatalf("unexpected follow-up %+v, want the search to finish with its shuffle", d)
	}
	n := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("shuffles = %d, want 1", n)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand-1 {
		t.Fatalf("hand = %d, want %d (the search found nothing)", got, hand-1)
	}
	replayCheck(t, e, cfg)
}

// TestTargetedGateLeavesAPreAskSASubRunnable: a sub-ability with its own
// ValidTgts$ and ConditionDefined$ Targeted is not skipped as "no targets"
// before its target is chosen: the chosen creature makes the gate hold and
// the pump applies. A TargetMin$ 0 sub that elected zero targets is a real
// zero and is skipped.
func TestTargetedGateLeavesAPreAskSASubRunnable(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct {
		name, min string
		pick      bool
	}{{"chosen target runs", "", true}, {"elected zero skips", " | TargetMin$ 0", false}} {
		t.Run(tc.name, func(t *testing.T) {
			spell := "Name:Gated Pump\nManaCost:G\nTypes:Sorcery\n" +
				"A:SP$ GainLife | Defined$ You | LifeAmount$ 1 | SubAbility$ DBPump\n" +
				"SVar:DBPump:DB$ Pump | ValidTgts$ Creature" + tc.min + " | NumAtt$ +2 | ConditionDefined$ Targeted | ConditionPresent$ Card\nOracle:x\n"
			e, cfg := fixtureGame(t, uint64(181+i), cardsOfSrcs(t, spell, creatureLineSrc("Bear")), nil)
			moveNamedTo(t, e, 0, "Gated Pump", state.ZHand)
			bear := moveNamedTo(t, e, 0, "Bear", state.ZBattlefield)
			power := e.Power(bear)
			life := e.G.Players[0].Life
			e.pending = nil
			addMana(t, e, 0, "G")
			submitChoices(t, e, castOptionFor(t, e, findNamedInZone(e, 0, state.ZHand, "Gated Pump")).Index)
			d := e.Pending()
			if d == nil || d.Kind != decision.KTarget {
				t.Fatalf("decision = %+v, want the sub-ability's target ask", d)
			}
			var choice []int
			if tc.pick {
				choice = []int{objOptionIndex(t, d, bear)}
			}
			if d := answerChoices(t, e, choice...); d != nil {
				t.Fatalf("unexpected follow-up %+v", d)
			}
			if got := e.G.Players[0].Life; got != life+1 {
				t.Fatalf("life = %d, want %d", got, life+1)
			}
			want := power
			if tc.pick {
				want = power + 2
			}
			if got := e.Power(bear); got != want {
				t.Fatalf("Bear power = %d, want %d", got, want)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestYunasDecisionPilgrimageEachCreatureAndLand: Yuna's Decision's real
// Continue the Pilgrimage mode (sacrifice a creature, draw, then you may put
// a creature card and/or a land card from your hand onto the battlefield).
// Its Optional$ ChangeZone asks the hand_move_confirm first -- even over a
// hand with no eligible card -- and the accepted pick is a hand_move KChoose,
// Min 0, Max one per present EACH clause, each option grouped by its clause.
func TestYunasDecisionPilgrimageEachCreatureAndLand(t *testing.T) {
	t.Parallel()
	yuna := corpusAlternativeCard(t, "Yuna's Decision")
	ability := cards.ResolveSVar(yuna.Faces[0].SVars, "DBChangeZone")
	if ability == nil || ability.API != "ChangeZone" || ability.Params["ChangeType"] != "EACH Creature & Land" {
		t.Fatalf("corpus pin moved: DBChangeZone = %+v", ability)
	}
	land := "Name:Pilgrim Forest\nTypes:Land Forest\nOracle:x\n"
	for i, tc := range []struct {
		name                         string
		includeCreature, includeLand bool
		want                         int
	}{
		{"creature only", true, false, 1},
		{"land only", false, true, 1},
		{"both", true, true, 2},
		{"none", false, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filler := "Name:Pilgrim Filler\nManaCost:U\nTypes:Instant\nOracle:x\n"
			e, cfg := fixtureGame(t, uint64(191+i), append([]*cards.Card{yuna},
				cardsOfSrcs(t, creatureLineSrc("Pilgrim Bear"), land, creatureLineSrc("Sacrificed Bear"), filler)...), nil)
			emptyHand(t, e, 0)
			moveNamedTo(t, e, 0, "Yuna's Decision", state.ZHand)
			moveNamedTo(t, e, 0, "Sacrificed Bear", state.ZBattlefield)
			var creature, landID state.ObjID
			if tc.includeCreature {
				creature = moveNamedTo(t, e, 0, "Pilgrim Bear", state.ZHand)
			}
			if tc.includeLand {
				landID = moveNamedTo(t, e, 0, "Pilgrim Forest", state.ZHand)
			}
			// The mode's draw takes a nonland, noncreature card, so the
			// eligible set is exactly the cards the case put in hand.
			libraryTopOrder(t, e, 0, moveNamedTo(t, e, 0, "Pilgrim Filler", state.ZLibrary))
			d := castNamedSpell(t, e, "Yuna's Decision", "GGGG", -1) // mode 0: Continue the Pilgrimage
			for d != nil && d.ResumeKind != "hand_move_confirm" {
				// The sacrifice of the one creature, if posed.
				d = answerChoices(t, e, 0)
			}
			if d == nil {
				t.Fatal("no hand_move_confirm was posed (Optional$ True asks before the pick)")
			}
			d = answerChoices(t, e, optionKindOrFail(t, d, "yes"))
			if findNamedInZone(e, 0, state.ZHand, "Pilgrim Filler") == 0 {
				t.Fatal("precondition: the mode did not draw")
			}
			if tc.want == 0 {
				if d != nil {
					t.Fatalf("an empty eligible set posed %+v after its accepted confirmation", d)
				}
				return
			}
			if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "hand_move" || d.Min != 0 {
				t.Fatalf("decision = %+v, want a Min 0 hand_move KChoose", d)
			}
			groups := map[state.ObjID]string{}
			for _, o := range d.Options {
				groups[o.Obj] = o.Group
			}
			if tc.includeCreature && groups[creature] != "0" {
				t.Fatalf("creature option group = %q, want EACH clause 0: %+v", groups[creature], d.Options)
			}
			if tc.includeLand && groups[landID] != "1" {
				t.Fatalf("land option group = %q, want EACH clause 1: %+v", groups[landID], d.Options)
			}
			if d.Max != tc.want || len(d.Options) != tc.want {
				t.Fatalf("Max/options = %d/%d, want %d/%d (one per present clause)", d.Max, len(d.Options), tc.want, tc.want)
			}
			var all []int
			var chosen []state.ObjID
			for _, o := range d.Options {
				all = append(all, o.Index)
				chosen = append(chosen, o.Obj)
			}
			if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: all}); err != nil {
				t.Fatalf("a one-per-present-clause answer was rejected: %v", err)
			}
			if d := answerChoices(t, e, all...); d != nil {
				t.Fatalf("unexpected follow-up %+v", d)
			}
			for _, id := range chosen {
				if z := zoneOfObjID(e, id); z != state.ZBattlefield {
					t.Fatalf("chosen card %d is in %v, want the battlefield", id, z)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestVerdantCrescendoSearchesItsCompoundOrigin: Verdant Crescendo's real
// DBSearch (Origin$ Library,Graveyard, no selector) offers the union of both
// zones' Nissa, Nature's Artisan cards, and the chosen GRAVEYARD copy -- not
// the first library card -- goes to hand.
func TestVerdantCrescendoSearchesItsCompoundOrigin(t *testing.T) {
	t.Parallel()
	vc := corpusAlternativeCard(t, "Verdant Crescendo")
	nissa := "Name:Nissa, Nature's Artisan\nManaCost:4 G G\nTypes:Legendary Planeswalker Nissa\nLoyalty:5\nOracle:x\n"
	forest := "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"
	e, cfg := fixtureGame(t, 201, append([]*cards.Card{vc}, cardsOfSrcs(t, nissa, nissa, forest)...), nil)
	moveNamedTo(t, e, 0, "Verdant Crescendo", state.ZHand)
	graveNissa := moveNamedTo(t, e, 0, "Nissa, Nature's Artisan", state.ZGraveyard)
	libNissa := moveNamedTo(t, e, 0, "Nissa, Nature's Artisan", state.ZLibrary)
	if libNissa == graveNissa {
		t.Fatal("precondition: the two Nissas are the same object")
	}
	moveNamedTo(t, e, 0, "Forest", state.ZLibrary)
	d := castNamedSpell(t, e, "Verdant Crescendo", "GGGG", -1)
	// The first search (a basic land, onto the battlefield tapped).
	if d == nil || d.ResumeKind != "search" {
		t.Fatalf("decision = %+v, want the basic land search", d)
	}
	d = answerChoices(t, e, 0)
	for d != nil && d.ResumeKind != "search" {
		d = answerChoices(t, e, tapePick(d)...)
	}
	if d == nil || d.Kind != decision.KChoose {
		t.Fatal("the compound-origin search posed no chooser")
	}
	offered := optionObjIDs(d)
	has := func(id state.ObjID) bool {
		for _, o := range offered {
			if o == id {
				return true
			}
		}
		return false
	}
	if !has(libNissa) || !has(graveNissa) {
		t.Fatalf("options = %v, want both compound-origin candidates %d and %d", offered, libNissa, graveNissa)
	}
	d = answerChoices(t, e, objOptionIndex(t, d, graveNissa))
	for d != nil {
		d = answerChoices(t, e, tapePick(d)...)
	}
	if got := zoneOfObjID(e, graveNissa); got != state.ZHand {
		t.Fatalf("answered graveyard Nissa zone = %v, want hand", got)
	}
	if got := zoneOfObjID(e, libNissa); got != state.ZLibrary {
		t.Fatalf("unchosen library Nissa zone = %v, want library", got)
	}
	replayCheck(t, e, cfg)
}

// TestMemoryLeakCompoundFetchPlayerChoosesFromBothZones: Memory Leak's real
// fetch (Origin$ Hand,Graveyard | DefinedPlayer$ Targeted | Chooser$ You)
// offers the caster the targeted opponent's hand AND graveyard cards only;
// the chosen graveyard card is exiled and the hand card stays.
func TestMemoryLeakCompoundFetchPlayerChoosesFromBothZones(t *testing.T) {
	t.Parallel()
	ml := corpusAlternativeCard(t, "Memory Leak")
	e, cfg := fixtureGame(t, 211, append([]*cards.Card{ml}, cardsOfSrcs(t, creatureLineSrc("Other Hand Creature"))...),
		cardsOfSrcs(t, creatureLineSrc("Hand Creature"), creatureLineSrc("Grave Creature")))
	emptyHand(t, e, 0)
	moveNamedTo(t, e, 0, "Memory Leak", state.ZHand)
	other := moveNamedTo(t, e, 0, "Other Hand Creature", state.ZHand)
	emptyHand(t, e, 1)
	hand := moveNamedTo(t, e, 1, "Hand Creature", state.ZHand)
	grave := moveNamedTo(t, e, 1, "Grave Creature", state.ZGraveyard)
	d := castNamedSpell(t, e, "Memory Leak", "BBB", 1)
	for d != nil && d.ResumeKind != "search" {
		d = answerChoices(t, e, tapePick(d)...) // the reveal's own acknowledgement, if any
	}
	if d == nil || d.Player != 0 {
		t.Fatalf("decision = %+v, want the caster's compound search", d)
	}
	seen := map[state.ObjID]bool{}
	for _, o := range d.Options {
		seen[o.Obj] = true
		if o.Player != 1 {
			t.Fatalf("candidate %+v owned by %d, want the targeted player", o, o.Player)
		}
	}
	if !seen[hand] || !seen[grave] || seen[other] {
		t.Fatalf("options = %+v, want the targeted player's hand and graveyard only", d.Options)
	}
	d = answerChoices(t, e, objOptionIndex(t, d, grave))
	for d != nil {
		d = answerChoices(t, e, tapePick(d)...)
	}
	if got := zoneOfObjID(e, grave); got != state.ZExile {
		t.Fatalf("chosen graveyard card zone = %v, want exile", got)
	}
	if got := zoneOfObjID(e, hand); got != state.ZHand {
		t.Fatalf("unchosen hand card zone = %v, want hand", got)
	}
	replayCheck(t, e, cfg)
}
