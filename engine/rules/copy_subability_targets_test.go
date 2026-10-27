package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// nonElectingCopierSrc is Mirrorpool's copy ability without MayChooseTarget$:
// the copy keeps every target of the original (CR 707.10), the chain's too.
const nonElectingCopierSrc = "Name:Test Plain Copier\nTypes:Artifact\n" +
	"A:AB$ CopySpellAbility | Cost$ 2 T Sac<1/CARDNAME> | ValidTgts$ Instant.YouCtrl,Sorcery.YouCtrl | TargetType$ Spell | SpellDescription$ Copy target instant or sorcery spell you control.\n" +
	"Oracle:x\n"

// copyChainBoard is the cr-g206 board: seat 0 holds the real Vibrant Outburst
// ("deals 3 damage to any target. Tap up to one target creature.", a
// SubAbility$ link with its own ValidTgts$) and a copier on the battlefield;
// seat 1 has two Grizzly Bears. The spell is cast at the opponent tapping
// bearA, then the copier's ability targets it.
type copyChainBoard struct {
	eng          *Engine
	cfg          Config
	spell        state.ObjID
	bearA, bearB state.ObjID
}

func newCopyChainBoard(t *testing.T, copierSrc string, copierIsMirrorpool bool) *copyChainBoard {
	t.Helper()
	reg := searchTestRegistry(t)
	vo := searchCorpusCard(t, reg, "Vibrant Outburst")
	bears := searchCorpusCard(t, reg, "Grizzly Bears")
	mountain := searchCorpusCard(t, reg, "Mountain")
	var copier *cards.Card
	if copierIsMirrorpool {
		copier = searchCorpusCard(t, reg, "Mirrorpool")
	} else {
		copier = card(t, copierSrc)
	}
	fill := func(cs ...*cards.Card) []*cards.Card {
		out := slices.Clone(cs)
		for len(out) < 40 {
			out = append(out, mountain)
		}
		return out
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{fill(vo, copier), fill(bears, bears)},
		Tokens: reg.Tokens}
	eng := New(cfg)
	eng.Advance()
	toMain1(t, eng)
	b := &copyChainBoard{eng: eng, cfg: cfg}
	b.bearA = miscMoveByName(t, eng, 1, "Grizzly Bears", state.ZBattlefield)
	b.bearB = miscMoveByName(t, eng, 1, "Grizzly Bears", state.ZBattlefield)
	copierID := miscMoveByName(t, eng, 0, copier.Faces[0].Name, state.ZBattlefield)
	b.spell = miscMoveByName(t, eng, 0, "Vibrant Outburst", state.ZHand)
	// Mirrorpool enters tapped (its own ETB replacement); clear the tap.
	eng.emit(events.Event{Kind: events.Untap, Obj: copierID})
	eng.priorityRound()
	addMana(t, eng, 0, "URCCC")

	// PRECONDITION: the spell really has a targeted SubAbility$ link, so a
	// root-only copy declaration cannot cover it.
	root := vo.Faces[0].SpellAbility()
	if root == nil || root.Sub == nil || root.Sub.Params["ValidTgts"] == "" {
		t.Fatalf("Vibrant Outburst carries no targeted SubAbility$ link: %+v", root)
	}

	submitChoices(t, eng, miscCastOption(t, eng, b.spell))
	d := eng.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the root target ask, got %+v", d)
	}
	submitChoices(t, eng, chainOptIndex(t, d, func(o decision.Option) bool { return o.Kind == "player" && o.Player == 1 }))
	d = eng.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "cast_sub" {
		t.Fatalf("expected the cast-time ask for the chained tap target, got %+v", d)
	}
	submitChoices(t, eng, chainOptIndex(t, d, func(o decision.Option) bool { return o.Obj == b.bearA }))
	so := eng.G.Obj(b.spell)
	if so == nil || so.Zone != state.ZStack || len(so.SubTargets) != 1 || so.SubTargets[0].Obj != b.bearA {
		t.Fatalf("the cast did not record bearA as the chained target: %+v", so)
	}
	return b
}

func chainOptIndex(t *testing.T, d *decision.Decision, pred func(decision.Option) bool) int {
	t.Helper()
	for _, o := range d.Options {
		if pred(o) {
			return o.Index
		}
	}
	t.Fatalf("no matching option in %+v", d.Options)
	return -1
}

// activateCopier targets the spell with the copier's (only copy) ability.
func (b *copyChainBoard) activateCopier(t *testing.T, copierID state.ObjID, abilityIdx int) {
	t.Helper()
	ab := abilityOption(t, b.eng, copierID, abilityIdx)
	submitChoices(t, b.eng, ab.Index)
	d := b.eng.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the copy ability's target ask, got %+v", d)
	}
	submitChoices(t, b.eng, chainOptIndex(t, d, func(o decision.Option) bool { return o.Obj == b.spell }))
}

func (b *copyChainBoard) boardObj(t *testing.T, name string) state.ObjID {
	t.Helper()
	return miscBoardObj(t, b.eng, 0, name)
}

// passNoLateAsk passes priority until stop() holds, failing on ANY non-priority
// decision: a copy must never ask its chained target mid-resolution.
func passNoLateAsk(t *testing.T, e *Engine, stop func() bool) {
	t.Helper()
	for i := 0; i < 40 && !e.G.Over && !stop(); i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision while resolving (a late chained-target ask?): %+v", d)
		}
		submitChoices(t, e, chainOptIndex(t, d, func(o decision.Option) bool { return o.Kind == "pass" }))
	}
	if !stop() {
		t.Fatalf("stack never reached the expected state: %v", e.G.Stack)
	}
}

func subTargetEventsOn(e *Engine, obj state.ObjID) [][]state.ObjID {
	var out [][]state.ObjID
	for _, ev := range e.L.Events {
		if ev.Kind == events.TargetsChosen && ev.Obj == obj && ev.Text == events.SubTargetNotice {
			out = append(out, ev.IDs)
		}
	}
	return out
}

// mirrorpoolCopy activates Mirrorpool's copy ability on the spell and returns
// the copy's first copy_targets ask (the second and third results are unused).
func (b *copyChainBoard) mirrorpoolCopy(t *testing.T) (first, unused *decision.Decision, copyID state.ObjID) {
	t.Helper()
	pool := b.boardObj(t, "Mirrorpool")
	b.activateCopier(t, pool, 1)
	d := passUntilNonPriority(t, b.eng, 30)
	if d.Kind != decision.KTarget || d.ResumeKind != "copy_targets" {
		t.Fatalf("expected the copy's first copy_targets ask, got %+v", d)
	}
	copyID = d.Source
	if co := b.eng.G.Obj(copyID); co == nil || !co.IsCopy || co.Zone != state.ZStack || !co.CopyMayChooseTarget {
		t.Fatalf("ask source %d is not an electing stack copy: %+v", copyID, co)
	}
	return d, nil, copyID
}

// TestCopyAsksChainedTargetAtCreation: CR 707.10c -- a copy with "you may
// choose new targets" lets its controller change ANY target of the copied
// spell when the copy is created, the SubAbility$ chain's included. Two
// copy_targets asks (any target, then the tap target), each pre-filled with the
// original's choice, and nothing is asked when the copy resolves.
func TestCopyAsksChainedTargetAtCreation(t *testing.T) {
	t.Parallel()
	b := newCopyChainBoard(t, "", true)
	eng := b.eng
	d0, _, copyID := b.mirrorpoolCopy(t)

	// Stage 0: the root's "any target" declaration, the opponent pre-filled.
	if d0.Options[0].Kind != "player" || d0.Options[0].Player != 1 {
		t.Fatalf("stage 0 did not pre-fill the original's player target: %+v", d0.Options)
	}
	if !slices.ContainsFunc(d0.Options, func(o decision.Option) bool { return o.Kind == "player" && o.Player == 0 }) {
		t.Fatalf("stage 0 is not the any-target declaration (no own-player option): %+v", d0.Options)
	}
	submitChoices(t, eng, 0)

	// Stage 1: the chained tap declaration -- creatures only, bearA pre-filled.
	d1 := eng.Pending()
	if d1 == nil || d1.Kind != decision.KTarget || d1.ResumeKind != "copy_targets" || d1.Source != copyID {
		t.Fatalf("expected the copy's chained copy_targets ask, got %+v", d1)
	}
	for _, o := range d1.Options {
		if o.Kind == "player" {
			t.Fatalf("stage 1 offered a player to a creature-only declaration: %+v", d1.Options)
		}
	}
	if d1.Options[0].Obj != b.bearA {
		t.Fatalf("stage 1 did not pre-fill the original's chained target bearA: %+v", d1.Options)
	}
	submitChoices(t, eng, chainOptIndex(t, d1, func(o decision.Option) bool { return o.Obj == b.bearB }))

	// The copy resolved on the spot with the new chained target: bearB tapped,
	// bearA (the original's, which has not resolved yet) untouched, and the
	// choice was announced as a real targeting on the copy.
	if eng.G.Obj(b.bearB).Tapped != true || eng.G.Obj(b.bearA).Tapped {
		t.Fatalf("copy did not tap the NEW chained target: bearA tapped=%v bearB tapped=%v",
			eng.G.Obj(b.bearA).Tapped, eng.G.Obj(b.bearB).Tapped)
	}
	if got := subTargetEventsOn(eng, copyID); len(got) != 1 || !slices.Equal(got[0], []state.ObjID{b.bearB}) {
		t.Fatalf("copy's chained choice was not announced as a sub-target: %v", got)
	}
	passNoLateAsk(t, eng, func() bool { return len(eng.G.Stack) == 0 })
	if !eng.G.Obj(b.bearA).Tapped {
		t.Fatalf("the original no longer taps its own chained target")
	}
	replayCheck(t, eng, b.cfg)
}

// TestCopyChainedTargetDefaultsToOriginal: answering each stage with the
// pre-filled original keeps the original's chained target for the copy.
func TestCopyChainedTargetDefaultsToOriginal(t *testing.T) {
	t.Parallel()
	b := newCopyChainBoard(t, "", true)
	eng := b.eng
	_, _, copyID := b.mirrorpoolCopy(t)
	submitChoices(t, eng, 0)
	d1 := eng.Pending()
	if d1 == nil || d1.Kind != decision.KTarget || d1.ResumeKind != "copy_targets" {
		t.Fatalf("expected the chained copy_targets ask, got %+v", d1)
	}
	if d1.Options[0].Obj != b.bearA || !slices.ContainsFunc(d1.Options, func(o decision.Option) bool { return o.Obj == b.bearB }) {
		t.Fatalf("chained ask is not pre-filled with bearA among creatures: %+v", d1.Options)
	}
	submitChoices(t, eng, 0)
	if !eng.G.Obj(b.bearA).Tapped || eng.G.Obj(b.bearB).Tapped {
		t.Fatalf("copy did not keep bearA: bearA tapped=%v bearB tapped=%v",
			eng.G.Obj(b.bearA).Tapped, eng.G.Obj(b.bearB).Tapped)
	}
	if got := subTargetEventsOn(eng, copyID); len(got) != 1 || !slices.Equal(got[0], []state.ObjID{b.bearA}) {
		t.Fatalf("copy's kept chained target was not announced: %v", got)
	}
	passNoLateAsk(t, eng, func() bool { return len(eng.G.Stack) == 0 })
	replayCheck(t, eng, b.cfg)
}

// TestNonElectingCopyInheritsChainedTarget: a CopySpellAbility without
// MayChooseTarget$ asks nothing and the copy taps the ORIGINAL's chained
// creature, recorded on the copy while it is on the stack.
func TestNonElectingCopyInheritsChainedTarget(t *testing.T) {
	t.Parallel()
	b := newCopyChainBoard(t, nonElectingCopierSrc, false)
	eng := b.eng
	copier := b.boardObj(t, "Test Plain Copier")
	if ab := card(t, nonElectingCopierSrc).Faces[0].Abilities; len(ab) != 1 || ab[0].Params["MayChooseTarget"] != "" {
		t.Fatalf("fixture copier is not a non-electing CopySpellAbility: %+v", ab)
	}
	b.activateCopier(t, copier, 0)
	passNoLateAsk(t, eng, func() bool { return copyOnStack(eng) != 0 })
	copyID := copyOnStack(eng)
	co := eng.G.Obj(copyID)
	if co.CopyMayChooseTarget {
		t.Fatalf("copy unexpectedly elects new targets: %+v", co)
	}
	if len(co.SubTargets) != 1 || co.SubTargets[0].Obj != b.bearA {
		t.Fatalf("copy did not inherit the original's chained target on the stack: %+v", co.SubTargets)
	}
	passNoLateAsk(t, eng, func() bool { return copyOnStack(eng) == 0 })
	if len(eng.G.Stack) != 1 || !eng.G.Obj(b.bearA).Tapped {
		t.Fatalf("the copy did not tap the original's chained creature: stack=%v tapped=%v",
			eng.G.Stack, eng.G.Obj(b.bearA).Tapped)
	}
	passNoLateAsk(t, eng, func() bool { return len(eng.G.Stack) == 0 })
	replayCheck(t, eng, b.cfg)
}
