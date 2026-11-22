package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// twinSeqBearerSrc is a creature whose dies trigger is a "choose two" Charm
// with two target-bearing modes: a 1..1 player target and a plural 0..2
// graveyard-card target. The plural bound forces the sequential distinct-mode
// ask path, whose per-answer re-ask (charmSeqSVars) is one of the two sibling
// sites that still read the source's ACTIVE face's SVars.
const twinSeqBearerSrc = "Name:Twin Seq Bearer\nManaCost:1B\nTypes:Creature Bear\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ TrigCharm | TriggerDescription$ When this creature dies, choose two.\n" +
	"SVar:TrigCharm:DB$ Charm | CharmNum$ 2 | Choices$ LoseMode,ExileMode\n" +
	"SVar:LoseMode:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 2 | SpellDescription$ Target player loses 2 life.\n" +
	"SVar:ExileMode:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 2 | Origin$ Graveyard | Destination$ Exile | ValidTgts$ Card | TgtPrompt$ Choose target card in a graveyard | SpellDescription$ Exile up to two target cards from graveyards.\n" +
	"Oracle:x\n"

// twinTapBearerSrc is a creature whose dies trigger is a "choose two" Charm
// with two 1..1 creature-target modes of OPPOSITE effects. Both modes are
// 1..1, so placement uses the one combined decision; the per-mode split is
// then rechecked at resolution (CR 608.2b, recheckCharmTargets) -- the other
// sibling site. With the split lost, both modes run over the combined flat
// list and the tap is undone by the untap mode.
const twinTapBearerSrc = "Name:Twin Tap Bearer\nManaCost:1B\nTypes:Creature Bear\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ TrigCharm | TriggerDescription$ When this creature dies, choose two.\n" +
	"SVar:TrigCharm:DB$ Charm | CharmNum$ 2 | Choices$ TapMode,UntapMode\n" +
	"SVar:TapMode:DB$ Tap | ValidTgts$ Creature | SpellDescription$ Tap target creature.\n" +
	"SVar:UntapMode:DB$ Untap | ValidTgts$ Creature | SpellDescription$ Untap target creature.\n" +
	"Oracle:x\n"

// twinCorpseSrc is a plain second fixture: the graveyard card the exile mode
// targets, or a battlefield creature for the tap mode.
const twinCorpseSrc = "Name:Twin Corpse\nManaCost:1G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// embalmCopiedBearer drives the Vizier of Many Faces embalm route against a
// battlefield fixture creature: the minted token's ETB copy election copies
// the bearer, and the copied bearer is returned. This is the shape the
// copied_trigger_mode_targets repro recorded (an embalmed Vizier copy): the
// copy's triggers are the template's, while the active face is not the face
// that owns the printed trigger.
func embalmCopiedBearer(t *testing.T, e *Engine, bearerSrc string) (bearer, tok state.ObjID) {
	t.Helper()
	bearer = putCreature(t, e, 0, bearerSrc)
	viz := findCardObj(t, e, 0, "Vizier of Many Faces", state.ZGraveyard)
	addMana(t, e, 0, "CCUUU") // Embalm {3}{U}{U}
	submitChoices(t, e, embalmAbilityOption(t, e, viz, "Embalm"))
	answered := false
	for i := 0; i < 40 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			if len(e.G.Stack) == 0 {
				break
			}
			e.priorityRound()
			continue
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "etb" {
			idx := -1
			for _, o := range d.Options {
				if o.Obj == bearer {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("the battlefield bearer was not offered to the copy election: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			answered = true
			continue
		}
		answerMisc(t, e, d)
	}
	if !answered {
		t.Fatal("the embalmed token's ETB copy election was never posed")
	}
	tok = embalmToken(t, e, cardName(bearerSrc))
	if tok == bearer {
		t.Fatalf("the token is the bearer itself: %d", tok)
	}
	return bearer, tok
}

// answerMisc answers a non-priority, non-target, non-mode decision the drain
// encounters (a hand-size discard, a yes/no): the first option is legal for
// every such ask this suite poses.
func answerMisc(t *testing.T, e *Engine, d *decision.Decision) {
	t.Helper()
	if d.Kind == decision.KPriority {
		passOnceP(t, e)
		return
	}
	if len(d.Options) == 0 {
		t.Fatalf("no options to answer: %+v", d)
	}
	if d.Min == 0 {
		// An optional ask (declare attackers, an up-to-N target): decline.
		submitChoices(t, e)
		return
	}
	submitChoices(t, e, d.Options[0].Index)
}

// killCopiedBearer kills the copied bearer and drives its trigger through
// mode announcement and target asks until the stack is empty. pick answers a
// KTarget ask with the caller's chosen option indices.
func killCopiedBearer(t *testing.T, e *Engine, tok state.ObjID, pick func(d *decision.Decision) []int) {
	t.Helper()
	o := e.G.Obj(tok)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the copied bearer %d is not on the battlefield: %+v", tok, o)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: tok, From: state.ZBattlefield, To: state.ZGraveyard})
	e.putTriggersOnStack()
	for i := 0; i < 120 && len(e.G.Stack) > 0 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			e.priorityRound()
			continue
		}
		switch d.Kind {
		case decision.KModes:
			// A CharmNum$ 2 over exactly two modes: both are mandatory.
			submitChoices(t, e, 0, 1)
		case decision.KTarget:
			idx := pick(d)
			if len(idx) == 0 && d.Min > 0 {
				t.Fatalf("no answer for the mandatory target ask: %+v", d)
			}
			submitChoices(t, e, idx...)
		default:
			answerMisc(t, e, d)
		}
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("the copied trigger never finished draining: stack %v, pending %+v", e.G.Stack, e.Pending())
	}
}

// TestCopiedModalTriggerSequentialReaskUsesTheTriggerFace is the sequential
// sibling (CR 603.3d with 707.2/113.7a): an embalmed Vizier of Many Faces copy
// of the bearer dies, its copied Charm announces both modes, the 1..1 player
// mode asks and lands -- and then the plural graveyard mode's own ask must be
// posed from the trigger-owning face's SVar table, not the source's active
// face (whose table lacks the mode bodies, so ResolveSVar is nil and the ask
// silently records an empty group). The graveyard card must be exiled.
func TestCopiedModalTriggerSequentialReaskUsesTheTriggerFace(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 7701, []string{"Vizier of Many Faces"},
		[]string{twinSeqBearerSrc, twinCorpseSrc}, nil)
	corpse := addToGraveyard(t, e, 0, twinCorpseSrc)
	if o := e.G.Obj(corpse); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: the exile fodder is in %s, want the graveyard", o.Zone)
	}
	if e.G.Players[1].Life != 20 {
		t.Fatalf("precondition: seat 1 life = %d, want 20", e.G.Players[1].Life)
	}
	_, tok := embalmCopiedBearer(t, e, twinSeqBearerSrc)
	killCopiedBearer(t, e, tok, func(d *decision.Decision) []int {
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player == 1 {
				return []int{o.Index}
			}
			if o.Obj == corpse {
				return []int{o.Index}
			}
		}
		return nil // an empty 0..2 exile ask has no target to give
	})
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("seat 1 life = %d, want 18 (the 1..1 mode landed)", got)
	}
	if o := e.G.Obj(corpse); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the graveyard card was never asked for: zone = %+v, want exile (the plural mode's ask resolved the trigger-owning face's bodies)", o)
	}
	replayCheck(t, e, cfg)
}

// TestCopiedModalTriggerRecheckBindsPerMode is the resolution sibling: the
// same copied Charm, but with two 1..1 creature-target modes, so placement's
// one combined ask records both groups and CR 608.2b's recheck must split
// them per mode at resolution. With the recheck resolving the slots against
// the source's active face, it finds no slots, reports the charm unhandled,
// and both modes run over the combined flat list: the untap mode undoes the
// tap mode.
func TestCopiedModalTriggerRecheckBindsPerMode(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 7702, []string{"Vizier of Many Faces"},
		[]string{twinTapBearerSrc, twinCorpseSrc}, nil)
	victim := putCreature(t, e, 0, twinCorpseSrc)
	if e.G.Obj(victim).Tapped {
		t.Fatalf("precondition: the victim is already tapped")
	}
	bearer, tok := embalmCopiedBearer(t, e, twinTapBearerSrc)
	killCopiedBearer(t, e, tok, func(d *decision.Decision) []int {
		// One target per mode: the tap mode (first group) takes the victim,
		// the untap mode takes the original bearer, which is untapped anyway.
		var out []int
		for g, want := range map[string]state.ObjID{"charm-mode-0": victim, "charm-mode-1": bearer} {
			for _, o := range d.Options {
				if o.Group == g && o.Obj == want {
					out = append(out, o.Index)
				}
			}
		}
		return out
	})
	if o := e.G.Obj(victim); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the victim left the battlefield: %+v", o)
	}
	if !e.G.Obj(victim).Tapped {
		t.Fatalf("the victim was never tapped: the per-mode recheck lost the mode split (both modes ran over the flat list)")
	}
	replayCheck(t, e, cfg)
}
