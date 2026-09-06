package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("CopySpellAbility", effCopySpellAbility)
}

// effCopySpellAbility duplicates the spell named by Defined$ (Task 17). Two
// shapes exist in the corpus:
//
//   - Defined$ Parent: the spell currently resolving (c.Source) — Chain
//     Lightning's own copy clause.
//   - Defined$ TriggeredSpellAbility: the trigger's remembered object, i.e.
//     the spell whose cast FIRED the trigger — Storm (cards/keywords.go
//     expands kw:Storm into exactly this).
//
// Amount$ copies are placed on the stack by StackCopy events (default 1),
// each copy keeping its targets. MayChooseTarget$ True is a player's
// mid-resolution choice this build still cannot ask (stay-down in the M2r
// approximations list — switching the copy's targets is a later task), so
// the copies keep their targets and each records a Note saying so.
//
// UnlessCost$ (Chain Lightning, String of Disappearances) is a real
// mid-resolution ask since M2d-2 closed R-8: on the first pass the target's
// controller is offered a KModes pay/decline decision and the resolution
// suspends; the answer re-enters this effect with Ctx.UnlessPay set, rules'
// resumeResolution having already paid the cost (payMana) when the payer
// said yes — so the copy loop below runs, or not, exactly once. A host that
// cannot ask (an effects-package test double) keeps the deterministic
// decline with a Note.
func effCopySpellAbility(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	if _, hasUnless := sa.Params["UnlessCost"]; hasUnless {
		switch c.UnlessPay {
		case "pay":
			// Re-entry: the payer paid the UnlessCost$ in rules'
			// resumeResolution (payMana, so it replays); fall through to the
			// ordinary copy body below.
		case "decline":
			return
		default:
			// First pass: pose the pay decision when the host can ask. The
			// payer is the UnlessPayer$: for the corpus's copy shapes that is
			// the targeted player or the controller of the targeted object
			// ("TargetedOrController", "Targeted"), so it resolves from the
			// first target; the resolving effect's own controller is the
			// fallback. The asked decision is the same KModes shape effCharm
			// uses, tagged "unless_pay" so the engine resumes the right
			// continuation.
			payer := c.Controller
			switch strings.TrimSpace(sa.Params["UnlessPayer"]) {
			case "TargetedOrController", "Targeted", "TargetedController":
				if len(c.Targets) > 0 {
					payer = PlayerOf(h, c, c.Targets[0])
				}
			}
			cost := strings.TrimSpace(sa.Params["UnlessCost"])
			d := &decision.Decision{Player: payer, Kind: decision.KModes,
				Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay",
				ResumeSA: sa,
				Prompt:   "Pay " + cost + " to copy the spell, or decline",
				Options: []decision.Option{
					{Index: 0, Kind: "mode", Label: "Pay " + cost + " — make a copy", Obj: c.Source, Player: payer},
					{Index: 1, Kind: "mode", Label: "Don't pay", Obj: c.Source, Player: payer},
				}}
			if h.Ask(d) {
				return // resolution suspended; the answer re-enters this effect.
			}
			// Fuzz/no-engine host: the deterministic decline (R-9), as
			// today — a card that pays nothing gets nothing.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "may pay declined (UnlessCost not asked on this host)"})
			return
		}
	}

	// Resolve which spell to copy. For a trigger the remembered entry is the
	// cast spell (the first object entry); for a direct Parent copy it is the
	// currently resolving spell itself.
	var spell state.ObjID
	switch strings.TrimSpace(sa.Params["Defined"]) {
	case "TriggeredSpellAbility":
		for _, t := range c.Remembered {
			if !t.IsPlayer && t.Obj != 0 {
				spell = t.Obj
				break
			}
		}
	default: // "Parent" and any unset/other name resolve to the source.
		spell = c.Source
	}
	if spell == 0 {
		return
	}
	// The source spell must actually be on the stack; a copy of something
	// that already left it is a no-op, the same totality stance as every
	// other effect primitive.
	if o := g.Obj(spell); o == nil || o.Zone != state.ZStack {
		return
	}

	controller := c.Controller
	mayChoose := strings.EqualFold(strings.TrimSpace(sa.Params["MayChooseTarget"]), "True")
	for n := Num(h, c, sa, "Amount", 1); n > 0; n-- {
		h.Emit(events.Event{Kind: events.StackCopy, Obj: spell, Player: controller})
		if mayChoose {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "copy keeps its targets"})
		}
	}
}
