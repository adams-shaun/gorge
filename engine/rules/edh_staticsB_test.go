// The edh-staticsB statics played through real games on the corpus cards:
// O-Naginata's CantAttach, Tamiyo, Collector of Tales' CantDiscard, Yurlok of
// Scorch Thrash's ManaBurn, Rasputin Dreamweaver's MaxCounter (CR 704.5r) and
// Immerwolf's CantTransform (CR 701.28f).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	edhbSmallSrc = "Name:EDHB Small\nManaCost:G\nTypes:Creature Elf\nPT:2/2\nOracle:x\n"
	edhbBruteSrc = "Name:EDHB Brute\nManaCost:3 R\nTypes:Creature Ogre\nPT:4/4\nOracle:x\n"
	// edhbMindRotSrc is an opponent's discard spell: the targeted player
	// chooses and discards two cards.
	// edhbDreamSrc puts one dream counter on target creature.
	edhbDreamSrc = "Name:EDHB Dream\nManaCost:W\nTypes:Instant\n" +
		"A:SP$ PutCounter | ValidTgts$ Creature | CounterType$ DREAM | CounterNum$ 1 | SpellDescription$ Put a dream counter on target creature.\nOracle:x\n"
	edhbMindRotSrc = "Name:EDHB Mind Rot\nManaCost:2 B\nTypes:Sorcery\n" +
		"A:SP$ Discard | NumCards$ 2 | Mode$ TgtChoose | ValidTgts$ Player | SpellDescription$ The targeted player discards two.\nOracle:x\n"
)

// edhbGame builds a two-seat game, seat 0 first: each seat's deck is its
// named corpus cards, its fixture sources, then Mountains.
func edhbGame(t *testing.T, seed uint64, names0, srcs0, names1, srcs1 []string) (*Engine, Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck := func(names, srcs []string) []*cards.Card {
		var d []*cards.Card
		for _, n := range names {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not found in the compiled corpus registry", n)
			}
			d = append(d, c)
		}
		for _, s := range srcs {
			d = append(d, card(t, s))
		}
		return append(d, mountainDeck(t, 40-len(d))...)
	}
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{deck(names0, srcs0), deck(names1, srcs1)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	return e, cfg
}

// edhbDrain resolves the stack, passing priority and answering any other
// ask with its first Min options.
func edhbDrain(t *testing.T, e *Engine) {
	t.Helper()
	for n := 0; n < 50 && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack (depth %d)", len(e.G.Stack))
		}
		var choices []int
		if d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					choices = []int{o.Index}
				}
			}
		} else {
			for i := 0; i < d.Min && i < len(d.Options); i++ {
				choices = append(choices, d.Options[i].Index)
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
			t.Fatalf("submit %v to %+v: %v", choices, d, err)
		}
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("stack did not drain (depth %d)", len(e.G.Stack))
	}
}

// O-Naginata "can be attached only to a creature with power 3 or greater":
// its equip aimed at a 2/2 resolves without attaching, while the same equip
// aimed at a 4/4 attaches.
func TestEDHStaticsBONaginataCantEquipSmallCreature(t *testing.T) {
	e, cfg := edhbGame(t, 901, []string{"O-Naginata"}, []string{edhbSmallSrc, edhbBruteSrc}, nil, nil)
	nag := moveToBattlefieldByName(t, e, 0, "O-Naginata")
	small := moveSeeded(t, e, 0, edhbSmallSrc, state.ZBattlefield)
	brute := moveSeeded(t, e, 0, edhbBruteSrc, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	if e.Power(small) != 2 || e.Power(brute) != 4 {
		t.Fatalf("precondition: powers %d/%d, want 2 and 4", e.Power(small), e.Power(brute))
	}

	addMana(t, e, 0, "CC")
	opt, ok := findAbilityOption(e, nag, 0)
	if !ok {
		t.Fatal("O-Naginata's equip not offered from a {2} pool")
	}
	submitChoices(t, e, opt.Index)
	targetObject(t, e, small)
	edhbDrain(t, e)
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: equip did not resolve paid (pool %d)", got)
	}
	if at := e.G.Obj(nag).AttachedTo; at == small {
		t.Fatal("O-Naginata attached to a 2-power creature")
	}

	addMana(t, e, 0, "CC")
	opt, ok = findAbilityOption(e, nag, 0)
	if !ok {
		t.Fatal("O-Naginata's equip not offered the second time")
	}
	submitChoices(t, e, opt.Index)
	targetObject(t, e, brute)
	edhbDrain(t, e)
	if at := e.G.Obj(nag).AttachedTo; at != brute {
		t.Fatalf("O-Naginata attached to %d, want the 4-power %d", at, brute)
	}
	replayCheck(t, e, cfg)
}

// Tamiyo, Collector of Tales: "Spells and abilities your opponents control
// can't cause you to discard cards": the opponent's resolved discard spell
// leaves Tamiyo's controller's hand untouched.
func TestEDHStaticsBTamiyoBlocksOpponentDiscard(t *testing.T) {
	e, cfg := edhbGame(t, 902, []string{"Tamiyo, Collector of Tales"}, nil, nil, []string{edhbMindRotSrc})
	moveToBattlefieldByName(t, e, 0, "Tamiyo, Collector of Tales")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 2, 1, state.StepMain1)
	rot := moveSeeded(t, e, 1, edhbMindRotSrc, state.ZHand)
	addMana(t, e, 1, "BBB")
	hand := len(e.G.Zone(state.ZHand, 0))
	if hand < 2 {
		t.Fatalf("precondition: seat 0 holds %d cards, want at least 2", hand)
	}
	castFromPriority(t, e, rot)
	targetPlayer(t, e, 0)
	edhbDrain(t, e)
	if o := e.G.Obj(rot); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: the discard spell did not resolve (zone %v)", o.Zone)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand {
		t.Fatalf("seat 0's hand went %d -> %d under Tamiyo", hand, got)
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 0 {
		t.Fatalf("seat 0's graveyard holds %d cards, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// Yurlok of Scorch Thrash: "{1}, {T}: Each player adds {B}{R}{G}" and "A
// player losing unspent mana causes that player to lose that much life":
// when main phase 1 ends with that mana unspent, both players lose 3.
func TestEDHStaticsBYurlokManaBurnsAtStepEnd(t *testing.T) {
	e, cfg := edhbGame(t, 903, []string{"Yurlok of Scorch Thrash"}, nil, nil, nil)
	yurlok := moveToBattlefieldByName(t, e, 0, "Yurlok of Scorch Thrash")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	addMana(t, e, 0, "C")
	opt, ok := gainedManaOption(e, yurlok)
	if !ok {
		t.Fatalf("Yurlok's mana ability not offered: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)
	edhbDrain(t, e)
	if p0, p1 := e.G.Players[0].Pool.Total(), e.G.Players[1].Pool.Total(); p0 != 3 || p1 != 3 {
		t.Fatalf("precondition: pools %d/%d after Yurlok's ability, want 3 each", p0, p1)
	}
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	passThroughStep(t, e, state.StepMain1, 20)
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: seat 0's pool did not empty as main 1 ended")
	}
	if got := e.G.Players[0].Life; got != life0-3 {
		t.Fatalf("seat 0 life %d -> %d, want to lose 3 to mana burn", life0, got)
	}
	if got := e.G.Players[1].Life; got != life1-3 {
		t.Fatalf("seat 1 life %d -> %d, want to lose 3 to mana burn", life1, got)
	}
	replayCheck(t, e, cfg)
}

// Rasputin Dreamweaver "can't have more than seven dream counters on it" (CR
// 704.5r): a resolved spell puts an eighth on, and the SBA takes it off.
func TestEDHStaticsBRasputinKeepsSevenDreamCounters(t *testing.T) {
	e, cfg := edhbGame(t, 904, []string{"Rasputin Dreamweaver"}, []string{edhbDreamSrc}, nil, nil)
	ras := moveToBattlefieldByName(t, e, 0, "Rasputin Dreamweaver")
	if got := e.G.Obj(ras).Counter("DREAM"); got != 7 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: ras, Counter: "DREAM", Amount: 7 - got})
	}
	dream := moveSeeded(t, e, 0, edhbDreamSrc, state.ZHand)
	addMana(t, e, 0, "W")
	from := len(e.L.Events)
	castFromPriority(t, e, dream)
	targetObject(t, e, ras)
	edhbDrain(t, e)
	added := false
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.CounterChange && ev.Obj == ras && ev.Counter == "DREAM" && ev.Amount > 0 {
			added = true
		}
	}
	if !added {
		t.Fatal("precondition: the spell put no dream counter on Rasputin")
	}
	if got := e.G.Obj(ras).Counter("DREAM"); got != 7 {
		t.Fatalf("Rasputin holds %d dream counters, want 7", got)
	}
	replayCheck(t, e, cfg)
}

// Immerwolf: "Non-Human Werewolves you control can't transform": when it
// becomes day, its controller's nightbound Werewolf stays on its back face
// while the opponent's identical one transforms back.
func TestEDHStaticsBImmerwolfHoldsNightbound(t *testing.T) {
	e, cfg := edhbGame(t, 905, []string{"Immerwolf", "Village Watch"}, nil, []string{"Village Watch"}, nil)
	moveToBattlefieldByName(t, e, 0, "Immerwolf")
	mine := moveToBattlefieldByName(t, e, 0, "Village Watch")
	theirs := moveToBattlefieldByName(t, e, 1, "Village Watch")
	e.setDayNight(state.DayNightNight)
	e.syncDayNightFaces()
	if e.G.Obj(mine).FaceIdx != 1 || e.G.Obj(theirs).FaceIdx != 1 {
		t.Fatalf("precondition: night faces %d/%d, want both back faces (Village Reavers)",
			e.G.Obj(mine).FaceIdx, e.G.Obj(theirs).FaceIdx)
	}
	e.setDayNight(state.DayNightDay)
	e.syncDayNightFaces()
	if e.G.Obj(theirs).FaceIdx != 0 {
		t.Fatal("precondition: the opponent's nightbound Werewolf did not transform at day")
	}
	if e.G.Obj(mine).FaceIdx != 1 {
		t.Fatal("Immerwolf's controller's non-Human Werewolf transformed")
	}
	e.pending = nil
	e.Advance()
	replayCheck(t, e, cfg)
}
