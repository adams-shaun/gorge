package rules

// The ability-cast copy trigger (abcopy1): "whenever you activate an ability
// ... copy that ability" fires on events.AbilityPush, whose Obj is the SOURCE
// PERMANENT -- the ability's stack wrapper is minted inside events.Apply and
// never travels on the event. Before the TriggerAbility role the trigger's
// Remembered named the battlefield permanent, so every Defined$
// TriggeredSpellAbility consumer resolved a non-stack object and
// effCopySpellAbility's stack zone guard no-oped silently: no copy, no Note.
//
// Carriers here, verbatim corpus lines (inline fixtures per the licensing
// rule):
//
//   - Unbound Flourishing's ability arm (rules/hasxmanacost_test.go's
//     unboundFlourishingSrc): DB$ CopySpellAbility, no cost -- the pin test.
//   - The Enigma Jewel / Locus of Enlightenment's T: line + TrigCopy SVar
//     (the_enigma_jewel_locus_of_enlightenment.txt): the second clean DB
//     carrier (the brief's other candidates need an Exhaust/EquippedBy
//     predicate that fights the fixture).
//   - Rings of Brighthearth's T: line + TrigCopySpell SVar
//     (rings_of_brighthearth.txt): the AB$ CopySpellAbility WITH Cost$ {2}
//     free-copy hazard -- routed through the pay/decline window, never free.
//   - A Verrak-shaped carrier whose Cost$ is the unpriceable PayLife<X>:
//     hard decline per the ParseUnlessCost convention (the ask is posed,
//     "pay" is not answerable); Verrak's own Condition$ LifePaid gate is
//     deliberately not carried -- the leaf under test is the unpriceable
//     cost, not that condition.
//
// Harness: glavaAnnounce / drainTriggerAsks / replayCheck from
// changex_test.go and hasxmanacost_test.go; the multi-activation leaf uses
// its own tapless X-cost ability so two activations of ONE permanent can be
// outstanding at once ({X}{T} would tap-block the second).

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// enigmaJewelAbilityCopySrc carries the_enigma_jewel_locus_of_enlightenment's
// T: line and TrigCopy SVar VERBATIM on a plain artifact (the fusion card's
// two faces are irrelevant to the ability arm).
const enigmaJewelAbilityCopySrc = "Name:Test Enigma Jewel\nManaCost:2\nTypes:Artifact\n" +
	"T:Mode$ AbilityCast | ValidActivatingPlayer$ You | ValidSA$ SpellAbility.!ManaAbility | TriggerZones$ Battlefield | Execute$ TrigCopy | TriggerDescription$ Whenever you activate an ability that isn't a mana ability, copy it. You may choose new targets for the copy.\n" +
	"SVar:TrigCopy:DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | MayChooseTarget$ True\n" +
	"Oracle:Whenever you activate an ability that isn't a mana ability, copy it. You may choose new targets for the copy.\n"

// ringsAbilityCopySrc carries rings_of_brighthearth's T: line and
// TrigCopySpell SVar VERBATIM -- the AB$ CopySpellAbility with Cost$ 2 free-
// copy hazard.
const ringsAbilityCopySrc = "Name:Test Rings of Brighthearth\nManaCost:3\nTypes:Artifact\n" +
	"T:Mode$ AbilityCast | ValidActivatingPlayer$ You | ValidSA$ SpellAbility.!ManaAbility | TriggerZones$ Battlefield | Execute$ TrigCopySpell | OptionalDecider$ You | TriggerDescription$ Whenever you activate an ability, if it isn't a mana ability, you may pay {2}. If you do, copy that ability. You may choose new targets for the copy.\n" +
	"SVar:TrigCopySpell:AB$ CopySpellAbility | Cost$ 2 | Defined$ TriggeredSpellAbility | MayChooseTarget$ True\n" +
	"Oracle:Whenever you activate an ability, if it isn't a mana ability, you may pay {2}. If you do, copy that ability. You may choose new targets for the copy.\n"

// verrakUnpriceableCopySrc carries verrak_warped_sengir's TrigCopySpell SVar
// VERBATIM (Cost$ PayLife<X>) with the T: line minus its Condition$ LifePaid
// gate, so the leaf under test is the unpriceable cost alone.
const verrakUnpriceableCopySrc = "Name:Test Verrak\nManaCost:2 B\nTypes:Creature Vampire\nPT:3/3\n" +
	"T:Mode$ AbilityCast | ValidActivatingPlayer$ You | ValidSA$ SpellAbility.!ManaAbility | TriggerZones$ Battlefield | Execute$ TrigCopySpell | TriggerDescription$ Whenever you activate an ability that isn't a mana ability, you may pay that much life again. If you do, copy that ability.\n" +
	"SVar:TrigCopySpell:AB$ CopySpellAbility | Cost$ PayLife<X> | Defined$ TriggeredSpellAbility | MayChooseTarget$ True\n" +
	"DeckHas:Ability$Life\n" +
	"Oracle:x\n"

// taplessXSifterSrc is changeXSifterSrc without the {T}: two activations of
// ONE permanent can then be outstanding at once for the multi-activation leaf.
const taplessXSifterSrc = "Name:Tapless X Sifter\nTypes:Artifact\n" +
	"A:AB$ Draw | Cost$ X | NumCards$ X | SpellDescription$ Draw X cards.\n" +
	"Oracle:Draw X cards.\n"

// lifepaySifterSrc is a non-X ability paid with life, so Verrak's shape can
// observe an activation without an X ask in the way.
const lifepaySifterSrc = "Name:Lifepay Sifter\nTypes:Artifact\n" +
	"A:AB$ Draw | Cost$ PayLife<1> | NumCards$ 1 | SpellDescription$ Pay 1 life: Draw a card.\n" +
	"DeckHas:Ability$Life\n" +
	"Oracle:Pay 1 life: Draw a card.\n"

// announceAbility activates a permanent's first ability (no X ask in the way,
// the lifepay fixture's shape) and returns the ability wrapper's id.
func announceAbility(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	castFirst(t, e, "ability")
	for i := len(e.G.Stack) - 1; i >= 0; i-- {
		o := e.G.Obj(e.G.Stack[i])
		if o == nil || o.Card != nil || o.Ability == nil {
			continue
		}
		if src := e.G.Obj(o.Source); src != nil && src.Face() != nil && src.Face().Name == name {
			if _, isTrig := state.TriggerOf(e.G, o); !isTrig {
				return o.ID
			}
		}
	}
	t.Fatalf("ability object of %q not on the stack after announce: %v", name, e.G.Stack)
	return 0
}

// drainCopyPayAsks drives the stack, answering the optional-decider ask yes
// and the copy-cost pay ask per mode: "pay" answers trigger_cost_pay (option
// 0), "decline" answers the same ask with option 1, and "hard-decline"
// FAILS unless the pay ask offers no answerable pay option at all (the
// unpriceable-cost shape) before answering its lone decline.
func drainCopyPayAsks(t *testing.T, e *Engine, limit int, mode string) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack (stack depth %d)", len(e.G.Stack))
		}
		switch {
		case d.Kind == decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
						t.Fatalf("submit pass: %v", err)
					}
					goto next
				}
			}
			t.Fatalf("priority decision with no pass option: %+v", d.Options)
		case d.Kind == decision.KTriggerOrder:
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: idx}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
		case d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_pay":
			switch mode {
			case "pay":
				submitChoices(t, e, 0)
			case "decline":
				submitChoices(t, e, 1)
			case "hard-decline":
				if len(d.Options) != 1 || d.Options[0].Kind != "trigger_cost_decline" {
					t.Fatalf("unpriceable copy cost posed answerable options %v, want decline only", d.Options)
				}
				submitChoices(t, e, 0)
			default:
				t.Fatalf("unknown drain mode %q", mode)
			}
		default:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
				t.Fatalf("submit %v: %v", d.Kind, err)
			}
		}
	next:
	}
	if !e.G.Over && len(e.G.Stack) > 0 {
		t.Fatalf("stack never emptied (depth %d after %d passes)", len(e.G.Stack), limit)
	}
}

// copyCount counts StackCopy events naming id.
func copyCount(e *Engine, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.StackCopy && ev.Obj == id {
			n++
		}
	}
	return n
}

// allStackCopies lists every StackCopy event's (Obj, Player) pair.
func allStackCopies(e *Engine) []state.ObjID {
	var out []state.ObjID
	for _, ev := range e.L.Events {
		if ev.Kind == events.StackCopy {
			out = append(out, ev.Obj)
		}
	}
	return out
}

// TestUFActivationCopyResolvesTheAbility is the pin: activate an X-cost
// ability under Unbound Flourishing, accept the trigger, and assert exactly
// one StackCopy OF THE ABILITY WRAPPER -- and that the copy actually
// RESOLVES (the copied Draw X draws its cards: hand +2, ability and copy one
// each). Before the fix this emitted 0 StackCopy events (the trigger's
// Remembered named the battlefield permanent; the copy's zone guard no-oped).
func TestUFActivationCopyResolvesTheAbility(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 140, unboundFlourishingSrc, changeXSifterSrc, plainSifterSrc)
	uf := moveSeeded(t, e, 0, unboundFlourishingSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, changeXSifterSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, plainSifterSrc, state.ZBattlefield)
	abil := glavaAnnounce(t, e, 1)
	if got := pushCount(e, uf); got != 1 {
		t.Fatalf("UF TriggerPush count after the X-cost activation = %d, want 1", got)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainTriggerAsks(t, e, 40)
	if got := copyCount(e, abil); got != 1 {
		t.Fatalf("StackCopy of the ability wrapper = %d, want exactly one (got %v)",
			got, allStackCopies(e))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+2 {
		t.Fatalf("hand %d, want %d (the ability drew 1 AND its copy drew 1 -- a copy that never resolves is not a fix)",
			got, handBefore+2)
	}
	replayCheck(t, e, cfg)
}

// TestEnigmaJewelActivationCopyResolves is the second clean DB carrier on its
// verbatim corpus lines: same shape, different card.
func TestEnigmaJewelActivationCopyResolves(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 141, enigmaJewelAbilityCopySrc, changeXSifterSrc, plainSifterSrc)
	moveSeeded(t, e, 0, enigmaJewelAbilityCopySrc, state.ZBattlefield)
	moveSeeded(t, e, 0, changeXSifterSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, plainSifterSrc, state.ZBattlefield)
	abil := glavaAnnounce(t, e, 1)
	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainTriggerAsks(t, e, 40)
	if got := copyCount(e, abil); got != 1 {
		t.Fatalf("StackCopy of the ability wrapper = %d, want exactly one (got %v)",
			got, allStackCopies(e))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+2 {
		t.Fatalf("hand %d, want %d (ability and copy each drew 1)", got, handBefore+2)
	}
	replayCheck(t, e, cfg)
}

// TestRingsCopyDeclineNeverCopies is the free-copy guard's decline leaf on
// Rings of Brighthearth's verbatim lines: activate, accept the trigger, then
// DECLINE the {2} pay ask -- no StackCopy, only the ability's own resolution
// (one draw).
func TestRingsCopyDeclineNeverCopies(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 142, ringsAbilityCopySrc, changeXSifterSrc, plainSifterSrc)
	moveSeeded(t, e, 0, ringsAbilityCopySrc, state.ZBattlefield)
	moveSeeded(t, e, 0, changeXSifterSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, plainSifterSrc, state.ZBattlefield)
	abil := glavaAnnounce(t, e, 1)
	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainCopyPayAsks(t, e, 40, "decline")
	if got := copyCount(e, abil); got != 0 {
		t.Fatalf("declined {2} pay still copied: %d StackCopy events %v", got, allStackCopies(e))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("hand %d, want %d (only the ability's own draw)", got, handBefore+1)
	}
	replayCheck(t, e, cfg)
}

// TestRingsCopyPaidCopiesOnce is the paid leaf: the same shape, the pay ask
// answered "pay" with the floating mana -- exactly one StackCopy and BOTH
// resolutions' draws (ability 1 + copy 1).
func TestRingsCopyPaidCopiesOnce(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 143, ringsAbilityCopySrc, changeXSifterSrc, plainSifterSrc)
	moveSeeded(t, e, 0, ringsAbilityCopySrc, state.ZBattlefield)
	moveSeeded(t, e, 0, changeXSifterSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, plainSifterSrc, state.ZBattlefield)
	abil := glavaAnnounce(t, e, 1) // {X}{T} at X=1 leaves 1 floating of the 2 added
	addMana(t, e, 0, "CC")         // the {2} is now payable from the pool
	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainCopyPayAsks(t, e, 40, "pay")
	if got := copyCount(e, abil); got != 1 {
		t.Fatalf("paid {2} copy = %d StackCopy events, want exactly one", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+2 {
		t.Fatalf("hand %d, want %d (ability and paid copy each drew 1)", got, handBefore+2)
	}
	replayCheck(t, e, cfg)
}

// TestUnpriceableCopyCostIsHardDecline is the Verrak-shaped leaf: a
// Cost$ PayLife<X> copy execute (verbatim SVar) poses its pay ask but offers
// NO answerable pay option -- one decline-only decision, no copy.
func TestUnpriceableCopyCostIsHardDecline(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 144, verrakUnpriceableCopySrc, lifepaySifterSrc, plainSifterSrc)
	moveSeeded(t, e, 0, verrakUnpriceableCopySrc, state.ZBattlefield)
	moveSeeded(t, e, 0, lifepaySifterSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, plainSifterSrc, state.ZBattlefield)
	abil := func() state.ObjID {
		addMana(t, e, 0, "") // drive to Main1 and the first priority ask
		return announceAbility(t, e, "Lifepay Sifter")
	}()
	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainCopyPayAsks(t, e, 40, "hard-decline")
	if got := copyCount(e, abil); got != 0 {
		t.Fatalf("unpriceable PayLife<X> copy still copied: %d events %v", got, allStackCopies(e))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("hand %d, want %d (only the ability's own draw)", got, handBefore+1)
	}
	replayCheck(t, e, cfg)
}

// TestEachTriggerCopiesItsOwnActivation is the multi-activation exactness
// leaf: two activations of the SAME permanent outstanding, each trigger's
// copy names ITS OWN activation -- the changex1 topmost-wrapper scan got
// exactly this wrong; the fire-time role fixes it. Asserted through the
// copies' {X}: the X=1 activation's copy draws 1, the X=2 activation's copy
// draws 2 (hand +6 total: 1+1+2+2; a topmost-binding defect would copy the
// newer wrapper twice and draw 1+1+2+2's counterpart 7, and the pre-fix
// engine copies nothing at all).
func TestEachTriggerCopiesItsOwnActivation(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 145, unboundFlourishingSrc, taplessXSifterSrc, plainSifterSrc)
	moveSeeded(t, e, 0, unboundFlourishingSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, taplessXSifterSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, plainSifterSrc, state.ZBattlefield)
	addMana(t, e, 0, "CCCCC")
	castFirst(t, e, "ability")
	if d := e.Pending(); d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" {
		t.Fatalf("first X decision %+v", d)
	}
	submitChoices(t, e, 1)
	act1 := glavaAbilityObject(t, e, func() state.ObjID {
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Tapless X Sifter" {
				return id
			}
		}
		t.Fatalf("Tapless X Sifter permanent not on the battlefield")
		return 0
	}())
	castFirst(t, e, "ability")
	if d := e.Pending(); d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" {
		t.Fatalf("second X decision %+v", d)
	}
	submitChoices(t, e, 2)
	act2 := glavaAbilityObject(t, e, func() state.ObjID {
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Tapless X Sifter" {
				return id
			}
		}
		t.Fatalf("Tapless X Sifter permanent not on the battlefield")
		return 0
	}())
	if act1 == act2 {
		t.Fatalf("both activations resolved to one stack object %d", act1)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainTriggerAsks(t, e, 60)
	copies := allStackCopies(e)
	if len(copies) != 2 || copies[0] == copies[1] {
		t.Fatalf("StackCopy events %v, want exactly two distinct ability wrappers", copies)
	}
	if (copies[0] != act1 && copies[0] != act2) || (copies[1] != act1 && copies[1] != act2) {
		t.Fatalf("StackCopy targets %v, want {%d, %d}", copies, act1, act2)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+6 {
		t.Fatalf("hand %d, want %d (each activation and ITS OWN copy drew its own X: 1+1+2+2)",
			got, handBefore+6)
	}
	replayCheck(t, e, cfg)
}
