package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
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
// UnlessCost$ (Chain Lightning, String of Disappearances) rides the ONE
// shared unless gate (effects.Resolve's unlessProceed dispatch, shared by
// every API): the payer is UnlessPayer$'s resolved target (default the
// target's controller), the pay/decline labels live in poseUnlessAsk's
// CopySpellAbility arm, and the copy loop below runs, or not, exactly once
// per the gate's orientation (rules' unless_pay resume arm charged the cost
// on an affordable "pay"). A host that cannot ask (an effects-package test
// double) keeps the deterministic decline.
func effCopySpellAbility(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
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
