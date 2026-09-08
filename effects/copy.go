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
		// effCopySpellAbility's hardcoded orientation is SWITCHED: paying the
		// UnlessCost$ CAUSES the copy ("If the player does, they may copy").
		// This is the OPPOSITE of effCounter, whose hardcoded default is
		// UNSWITCHED (paying PREVENTS the counter), so the same guard
		// expression means opposite things in the two primitives. Every
		// UnlessSwitched$ True copy shape already agrees with THIS default
		// and must stay exactly as written; only an UnlessCost$ with NO
		// UnlessSwitched$ is the UNSWITCHED shape, which inverts it ("If
		// they don't, you may copy" — Wandering Archaic, the sole corpus
		// carrier) and is what the reversal below fixes. Do not paste
		// effCounter's guard here: it would break the switched copy shapes
		// that main already gets right.
		switched := strings.EqualFold(strings.TrimSpace(sa.Params["UnlessSwitched"]), "True")
		switch c.UnlessPay {
		case "pay":
			// Re-entry: the payer paid the UnlessCost$ in rules'
			// resumeResolution (payMana, so it replays). On the UNSWITCHED
			// shape paying stops the copy; only the SWITCHED shape falls
			// through to the ordinary copy body.
			if !switched {
				return
			}
		case "decline":
			// Re-entry: the payer declined. On the SWITCHED shape that stops
			// the copy; on the UNSWITCHED shape the decline IS the copy path
			// ("if they don't, you may copy"), so fall through.
			if switched {
				return
			}
		default:
			// First pass: pose the pay decision when the host can ask. The
			// payer is the UnlessPayer$: for the corpus's copy shapes that is
			// the targeted player or the controller of the targeted object
			// ("TargetedOrController", "Targeted"), so it resolves from the
			// first target; the resolving effect's own controller is the
			// fallback (Wandering Archaic's "TriggeredActivator" resolves to
			// the trigger's controller, which is c.Controller). The asked
			// decision is the same KModes shape effCharm uses, tagged
			// "unless_pay" so the engine resumes the right continuation. The
			// prompt and labels are keyed on the orientation so the seat sees
			// the real consequence of each choice.
			payer := c.Controller
			switch strings.TrimSpace(sa.Params["UnlessPayer"]) {
			case "TargetedOrController", "Targeted", "TargetedController":
				if len(c.Targets) > 0 {
					payer = PlayerOf(h, c, c.Targets[0])
				}
			}
			cost := strings.TrimSpace(sa.Params["UnlessCost"])
			var prompt, payLabel, declineLabel string
			if switched {
				prompt = "Pay " + cost + " to copy the spell, or decline"
				payLabel = "Pay " + cost + " — make a copy"
				declineLabel = "Don't pay"
			} else {
				prompt = "Pay " + cost + " to stop the copy, or decline to copy"
				payLabel = "Pay " + cost + " — no copy"
				declineLabel = "Don't pay — make a copy"
			}
			d := &decision.Decision{Player: payer, Kind: decision.KModes,
				Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay",
				ResumeSA: sa,
				Prompt:   prompt,
				Options: []decision.Option{
					{Index: 0, Kind: "mode", Label: payLabel, Obj: c.Source, Player: payer},
					{Index: 1, Kind: "mode", Label: declineLabel, Obj: c.Source, Player: payer},
				}}
			if h.Ask(d) {
				return // resolution suspended; the answer re-enters this effect.
			}
			// Fuzz/no-engine host: the deterministic decline (R-9). A
			// SWITCHED shape declines to nothing; an UNSWITCHED shape's
			// deterministic decline is the copy path, so it falls through.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "may pay declined (UnlessCost not asked on this host)"})
			if switched {
				return
			}
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
