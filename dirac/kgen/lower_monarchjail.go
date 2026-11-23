package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Palace Jailer's "exile target creature an opponent controls until an
// opponent becomes the monarch" (CR 610.3): a ChangeZone to exile whose
// SubAbility$ is an Effect arming a one-shot Mode$ BecomeMonarch promise.
// The pair lowers as ONE instruction, OpExt ExtMonarchJail over the target
// slot (dirac/act_monarchjail.go): the exile, CR 610.3b's skip when an
// opponent became the monarch since the ability was put on the stack, and
// the monarch link that returns the card. changeZone rewrites the exile
// (monarchJailMark) and the Effect node then lowers to nothing
// (monarchJailEffect), after its whole shape is read: any other Effect
// trigger shape keeps monarchEffectTrigger's refusal.

// monarchJailSub is the Effect node of a ChangeZone's SubAbility$ when it
// arms a Mode$ BecomeMonarch trigger, else nil.
func (lc *lowerCtx) monarchJailSub(ps *params) *cards.SA {
	sub := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(ps.m["SubAbility"]))
	if sub == nil || sub.API != "Effect" {
		return nil
	}
	for _, name := range strings.Fields(sub.Params["Triggers"]) {
		if t, ok := cards.ParseTriggerLine(lc.face.SVars[name]); ok && t.Mode == "BecomeMonarch" {
			return sub
		}
	}
	return nil
}

// monarchJailMark turns the exile changeZoneMove emitted at prog[start] into
// the Jailer's op when the move is a plain targeted exile whose Effect
// SubAbility$ arms the promise. Any other shape is left alone.
func (lc *lowerCtx) monarchJailMark(ps *params, start int) {
	if len(lc.ab.Prog) != start+1 || lc.monarchJailSub(ps) == nil {
		return
	}
	if in := &lc.ab.Prog[start]; in.Op == "Exile" && (in.CS == "" || in.CS == "from=Battlefield") && in.Obj.Kind == rTarget {
		in.Op = "MonarchJail"
		in.Note = "exiled until an opponent becomes the monarch; CR 610.3b skip (gorge exileUntilOpponentMonarchMet)"
	}
}

// monarchJailEffect lowers the Effect node behind a MonarchJail op to nothing
// (the op carries the promise) when its params and trigger chain are exactly
// Palace Jailer's: Triggers$ ComeBack, RememberObjects$ You & Targeted,
// Duration$ Permanent, EffectOwner$ TargetedOwner, ForgetOnMoved$ Exile; the
// trigger returns the remembered card to the battlefield (ChangeZoneAll) and
// exiles the Effect itself. handled is false when the previous instruction is
// not a MonarchJail.
func (lc *lowerCtx) monarchJailEffect(ps *params) (handled bool, err error) {
	if n := len(lc.ab.Prog); n == 0 || lc.ab.Prog[n-1].Op != "MonarchJail" {
		return false, nil
	}
	want := map[string]string{"Triggers": "ComeBack", "RememberObjects": "You & Targeted", "Duration": "Permanent", "EffectOwner": "TargetedOwner", "ForgetOnMoved": "Exile"}
	if !sameParams(ps.m, want) {
		return true, monarchEffectTrigger()
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars["ComeBack"])
	if !ok || t.Mode != "BecomeMonarch" || !sameParams(t.Params, map[string]string{"Mode": "BecomeMonarch", "ValidPlayer": "Player.OpponentOf Remembered", "Execute": "TrigReturn", "TriggerZones": "Command", "Static": "True"}) {
		return true, monarchEffectTrigger()
	}
	ret := cards.ResolveSVar(lc.face.SVars, "TrigReturn")
	if ret == nil || ret.API != "ChangeZoneAll" || !sameParams(ret.Params, map[string]string{"Origin": "Exile", "Destination": "Battlefield", "ChangeType": "Card.IsRemembered", "SubAbility": "ExileSelf"}) {
		return true, monarchEffectTrigger()
	}
	self := cards.ResolveSVar(lc.face.SVars, "ExileSelf")
	if self == nil || self.API != "ChangeZone" || !sameParams(self.Params, map[string]string{"Origin": "Command", "Destination": "Exile", "Defined": "Self"}) {
		return true, monarchEffectTrigger()
	}
	ps.consume("Triggers", "RememberObjects", "Duration", "EffectOwner", "ForgetOnMoved")
	return true, nil
}

// sameParams reports whether m holds exactly the want pairs (values trimmed),
// ignoring description text.
func sameParams(m, want map[string]string) bool {
	n := 0
	for k, v := range m {
		switch k {
		case "TriggerDescription", "SpellDescription", "StackDescription", "AILogic", "PlayMain1":
			continue
		}
		if w, ok := want[k]; !ok || strings.TrimSpace(v) != w {
			return false
		}
		n++
	}
	return n == len(want)
}
