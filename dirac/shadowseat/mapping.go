package shadowseat

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

// The mapping meets dirac's actions and gorge's options at the canonical
// action (dirac/canon), the vocabulary the oracle and the adapter share. The
// gorge side mirrors dirac/oracle's priorityVocabulary and optionAction for
// the kinds the seat stages: a priority option or planned cast, an attacker, a
// blocker and a target. Every other option has no canon and is never chosen.

// answer is the gorge answer one canonical action stands for: the option
// index, or for a planned cast the payment action (which wins over the plain
// cast option of the same object, as the oracle's planned entry does).
type answer struct {
	idx int
	pay *decision.PaymentAction
}

// ref is the canonical reference of the view-shown object id: its dirac
// object index, or its place on the stack.
func refOf(v *view.View, num *shadow.Numbering, id int) (canon.Ref, bool) {
	if id == 0 {
		return canon.RefNone, false
	}
	if o, ok := num.ObjOf(id); ok {
		return canon.Ref(o), true
	}
	for i := range v.Stack {
		if int(v.Stack[i].ID) == id && i < int(canon.RefPlayer-canon.RefStack) {
			return canon.RefStack + canon.Ref(i), true
		}
	}
	return canon.RefNone, false
}

// gorgeAnswers is the canonical action of each option of d the seat can
// answer with. Two options with one canon keep the first and the canon is
// listed in dups (the package's test pins that none occurs).
func gorgeAnswers(v *view.View, d *decision.Decision, num *shadow.Numbering) (m map[canon.Canon]answer, dups []canon.Canon) {
	m = map[canon.Canon]answer{}
	add := func(c canon.Canon, a answer) {
		if _, dup := m[c]; dup {
			dups = append(dups, c)
			return
		}
		m[c] = a
	}
	ref := func(id int) (canon.Ref, bool) { return refOf(v, num, id) }
	for i := range d.Options {
		o := &d.Options[i]
		a := answer{idx: o.Index}
		switch d.Kind {
		case decision.KPriority:
			switch o.Kind {
			case "pass":
				add(canon.Pack(canon.ActPass, canon.RefNone, canon.RefNone, 0), a)
			case "play_land":
				if r, ok := ref(int(o.Obj)); ok {
					add(canon.Pack(canon.ActPlayLand, r, canon.RefNone, canon.LandFaceCode(o.Mode)), a)
				}
			case "ability":
				if o.SVar != "" || o.Keyword != "" || o.GainedSource != 0 || o.AltCostIndex != 0 || o.Ability > 0x7F || o.Ability < 0 {
					continue
				}
				if r, ok := ref(int(o.Obj)); ok {
					add(canon.Pack(canon.ActActivate, r, canon.RefNone, uint8(o.Ability)), a)
				}
			case "cast":
				if o.Mode != "" || o.AltCostIndex != 0 {
					continue
				}
				if r, ok := ref(int(o.Obj)); ok {
					mc, _ := canon.CastModeCode("")
					add(canon.Pack(canon.ActCast, r, canon.RefNone, mc), a)
				}
			}
		case decision.KAttackers:
			if o.Battle != 0 {
				continue
			}
			if r, ok := ref(int(o.Obj)); ok {
				add(canon.Pack(canon.ActAttack, r, canon.RefPlayer+canon.Ref(o.Player), 0), a)
			}
		case decision.KBlockers:
			r, ok := ref(int(o.Obj))
			t, ok2 := ref(int(o.Attacker))
			if ok && ok2 {
				add(canon.Pack(canon.ActBlock, r, t, 0), a)
			}
		case decision.KTarget:
			if o.Obj == 0 {
				add(canon.Pack(canon.ActTarget, canon.RefNone, canon.RefPlayer+canon.Ref(o.Player), 0), a)
			} else if r, ok := ref(int(o.Obj)); ok {
				add(canon.Pack(canon.ActTarget, canon.RefNone, r, 0), a)
			}
		}
	}
	if d.Kind != decision.KPriority {
		return m, dups
	}
	// planned casts: the payment action replaces the plain cast option
	for i := range d.PaymentActions {
		pa := &d.PaymentActions[i]
		if len(pa.Plans) == 0 || pa.Cast.CastMode() != "" {
			continue
		}
		if r, ok := ref(int(pa.Cast.Object)); ok {
			mc, _ := canon.CastModeCode("")
			m[canon.Pack(canon.ActCast, r, canon.RefNone, mc)] = answer{pay: pa}
		}
	}
	return m, dups
}

// actionNames are the dirac action kinds' names in an unmapped reason.
var actionNames = [dirac.AkCount]string{
	dirac.AkPass: "pass", dirac.AkPlayLand: "land", dirac.AkCast: "cast", dirac.AkActivate: "activate",
	dirac.AkAttack: "attack", dirac.AkBlock: "block", dirac.AkDone: "done", dirac.AkTarget: "target",
	dirac.AkMode: "mode", dirac.AkChoose: "choose", dirac.AkKeep: "keep", dirac.AkMulligan: "mulligan",
	dirac.AkOrder: "order", dirac.AkTurnFaceUp: "turnfaceup", dirac.AkPlot: "plot", dirac.AkUnlock: "unlock",
	dirac.AkCastCasualty: "castcasualty", dirac.AkSneak: "sneak", dirac.AkBestow: "bestow",
	dirac.AkWebSling: "websling", dirac.AkMayFlash: "mayflash", dirac.AkCastConspire: "castconspire",
}

func unmapped(a dirac.Action) string {
	if k := a.Kind(); int(k) < len(actionNames) && actionNames[k] != "" {
		return "unmapped/" + actionNames[k]
	}
	return fmt.Sprintf("unmapped/kind%d", a.Kind())
}

// mapPick is the gorge answer of dirac action a.
func mapPick(tr *canon.Translator, ans map[canon.Canon]answer, a dirac.Action) (answer, string) {
	r, ok := ans[tr.Action(a)]
	if !ok {
		return answer{}, unmapped(a)
	}
	return r, ""
}

// mapPicks is the gorge answers of the picks acts make, up to AkDone. A
// target pick of NoTarget, the up-to slot left empty, is no pick.
func mapPicks(tr *canon.Translator, ans map[canon.Canon]answer, acts []dirac.Action) ([]answer, string) {
	var out []answer
	for _, a := range acts {
		if a.Kind() == dirac.AkDone {
			break
		}
		if a.Kind() == dirac.AkTarget && a.Target() == dirac.NoTarget {
			continue
		}
		r, reason := mapPick(tr, ans, a)
		if reason != "" {
			return nil, reason
		}
		out = append(out, r)
	}
	return out, ""
}

// intentOf is the gorge intent of d answered by picks. A priority answer is
// one pick: a planned cast selects its payment action and first plan, any
// other pick is its option. A declaration or target answer is the option
// indices in pick order (none for "no attackers").
func intentOf(d *decision.Decision, picks []answer) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	if d.Kind == decision.KPriority && len(picks) == 1 && picks[0].pay != nil {
		pa := picks[0].pay
		in.Payment = &decision.PaymentSelection{ActionID: pa.ID, Plan: decision.ClonePaymentPlan(pa.Plans[0])}
		return in
	}
	in.Choices = make([]int, 0, len(picks))
	for _, p := range picks {
		in.Choices = append(in.Choices, p.idx)
	}
	return in
}
