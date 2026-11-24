package kgen

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// api ChooseSource (gorge effChooseSource, engine/effects/choose_control.go)
// and the Effect that follows it: "the next time a source of your choice would
// deal damage to you this turn, prevent that damage" (Circle of Protection:
// Red, Rune of Protection, Invulnerability, Righteous Aura, ...). The two
// nodes lower together, as ChooseColor's prevention does (lower_effect.go):
//
//	OpExt ExtChooseSource, C the Choices$ pool (chooseSourceMask)
//	OpExt ExtPreventNext, C PrevNextAll|PrevNextFrom|PrevNextChosen, B the
//	  damage-time re-check of the source (shield mask, same bit layout)
//
// The ask is posed to the resolving controller over every battlefield
// permanent and stack spell the pool admits, in object order, and records the
// answer in the kernel's chosen set; the shield reads it back
// (dirac/act_choosesource.go, dirac/repl_prevchosen.go). Any other
// ChooseSource chain (a redirect, a rider body, a damage-to-target recipient,
// the prevent-all-this-turn Prevent$ form) stays refused by name.

func init() { apiPS("ChooseSource", (*lowerCtx).chooseSource) }

// chooseSourceMask turns a Choices$ pool (or a damage-time ValidSource$ part
// list) into the mask the kernel reads (colour bits, kop.ChosenSrc*): 0 admits
// every source. An unlisted part refuses.
func chooseSourceMask(spec string) (int, error) {
	mask := 0
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		// gorge reads Card.ChosenCard and Card.ChosenCardStrict as one
		// predicate (effects/filter.go): Pilgrim of Justice's spelling
		if rest, ok := strings.CutPrefix(part, "Card.ChosenCard+"); ok {
			part = "Card.ChosenCardStrict+" + rest
		}
		if part == "Emblem" || strings.HasPrefix(part, "Emblem.") {
			continue // an emblem is no battlefield or stack object: it adds no option
		}
		var bit int
		switch part {
		case "Card", "Card.ChosenCardStrict":
			return 0, nil // every source
		case "Card.WhiteSource", "Card.ChosenCardStrict+WhiteSource":
			bit = int(kop.ColW)
		case "Card.BlueSource", "Card.ChosenCardStrict+BlueSource":
			bit = int(kop.ColU)
		case "Card.BlackSource", "Card.ChosenCardStrict+BlackSource":
			bit = int(kop.ColB)
		case "Card.RedSource", "Card.ChosenCardStrict+RedSource":
			bit = int(kop.ColR)
		case "Card.GreenSource", "Card.ChosenCardStrict+GreenSource":
			bit = int(kop.ColG)
		case "Artifact", "Card.Artifact", "Card.ChosenCardStrict+Artifact":
			bit = kop.ChosenSrcArtifact
		case "Land", "Card.Land", "Card.ChosenCardStrict+Land":
			bit = kop.ChosenSrcLand
		default:
			return 0, refuse("ChooseSource source %q", part)
		}
		mask |= bit
	}
	if mask == 0 {
		return 0, refuse("ChooseSource source pool %q admits no source kind", spec)
	}
	return mask, nil
}

func (lc *lowerCtx) chooseSource(ps *params) error {
	mask, err := chooseSourceMask(ps.str("Choices"))
	if err != nil {
		return err
	}
	if _, has := lc.curNode.Params["UnlessCost"]; has {
		// Rhystic Circle: gorge asks each player in turn whether to pay before
		// the pick, and the kernel's unless wrapper poses the pick first
		return refuse("ChooseSource UnlessCost$ (any player may pay before the pick)")
	}
	n := lc.curNode.Sub
	if n == nil || n.API != "Effect" {
		return refuse("ChooseSource without its prevention Effect")
	}
	ps.consume("AILogic", "SubAbility")
	lc.chosenSrc = true
	lc.emit(lInstr{Op: "ChooseSource", C: mask, Note: "the controller chooses a damage source (a permanent or a spell)"})
	return nil
}

// chosenSrcEffect is the Effect half: the chosen-source DamageDone
// replacement, either the shield whose body is the Effect's self-exile (spent
// on the first event it prevents: PreventionEffect$ True) or the bodyless
// Prevent$ True line that prevents every event from the source this turn
// (kop.PrevNextChosenTurn: Pay No Heed). The recipient is You, any (no
// ValidTarget$: Pilgrim of Justice, kop.PrevNextChosenAny), or the Effect's
// own target (kop.PrevNextChosenTo, chosenSrcTarget).
func (lc *lowerCtx) chosenSrcEffect(ps *params) error {
	lc.chosenSrc = false
	if d := ps.peek("ConditionDefined"); d != "" {
		if d != "ChosenCard" || (ps.peek("ConditionPresent") != "Card" && ps.peek("ConditionPresent") != "Card,Emblem") ||
			(ps.peek("ConditionCompare") != "" && ps.peek("ConditionCompare") != "GE1") {
			return refuse("ChooseSource Effect condition")
		}
		ps.consume("ConditionDefined", "ConditionPresent", "ConditionCompare")
	}
	name := ps.peek("ReplacementEffects")
	if name == "" || strings.Contains(name, ",") {
		return refuse("ChooseSource Effect replacement shape")
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "DamageDone" {
		return refuse("ChooseSource Effect replacement shape")
	}
	rm := r.Params
	// Sorted: a map range would name a different unknown parameter run to run and make the census TSV differ.
	keys := make([]string, 0, len(rm))
	for k := range rm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch k {
		case "Event", "Description", "ValidSource", "ValidTarget", "ReplaceWith", "PreventionEffect", "Prevent":
		default:
			return refuse("ChooseSource Effect replacement param %s", k)
		}
	}
	c := kop.PrevNextAll | kop.PrevNextFrom | kop.PrevNextChosen
	_, hasW := rm["ReplaceWith"]
	_, hasPE := rm["PreventionEffect"]
	switch {
	case strings.TrimSpace(rm["Prevent"]) == "True" && !hasW && !hasPE:
		c |= kop.PrevNextChosenTurn
	case strings.TrimSpace(rm["PreventionEffect"]) == "True" && rm["Prevent"] == "" && lc.bodyIsSelfExile(strings.TrimSpace(rm["ReplaceWith"])):
	default:
		return refuse("ChooseSource Effect replacement shape")
	}
	mask, err := chooseSourceMask(strings.TrimSpace(rm["ValidSource"]))
	if err != nil {
		return err
	}
	var obj lRef
	switch vt := strings.TrimSpace(rm["ValidTarget"]); vt {
	case "You":
	case "":
		c |= kop.PrevNextChosenAny
	default:
		if obj, err = lc.chosenSrcTarget(ps, vt); err != nil {
			return err
		}
		c |= kop.PrevNextChosenTo
	}
	if v := ps.peek("RememberObjects"); v != "" && c&kop.PrevNextChosenTo == 0 {
		// Righteous Aura's RememberObjects$ Remembered remembers what the
		// chain already remembers, which nothing here reads
		if v != "Remembered" {
			return refuse("ChooseSource Effect RememberObjects$ %q", v)
		}
		ps.consume("RememberObjects")
	}
	ps.consume("ReplacementEffects", "AILogic", "SubAbility")
	lc.emit(lInstr{Op: "PreventNext", Obj: obj, C: c, A: cst(mask),
		Note: "prevent the next damage the chosen source would deal to you this turn"})
	return nil
}

// chosenSrcTarget reads the Effect's own target as the shield's recipient:
// ValidTarget$ Card.IsRemembered,Player.IsRemembered over RememberObjects$
// Targeted (Martyr's Cause, Sanctum Guardian), with ExileOnMoved$ or
// ForgetOnMoved$ Battlefield (the kernel matches the target at its version,
// so a permanent that left is no recipient either way), or ValidTarget$
// Card.IsImprinted over ImprintCards$ Targeted (Samite Blessing). It consumes
// the Effect's target and memory parameters.
func (lc *lowerCtx) chosenSrcTarget(ps *params, vt string) (lRef, error) {
	if _, ok := ps.get("ValidTgts"); !ok {
		return lRef{}, refuse("ChooseSource Effect replacement shape")
	}
	switch vt {
	case "Card.IsRemembered,Player.IsRemembered", "Player.IsRemembered,Card.IsRemembered", "Card.IsRemembered":
		if ps.peek("RememberObjects") != "Targeted" {
			return lRef{}, refuse("ChooseSource Effect RememberObjects$ %q", ps.peek("RememberObjects"))
		}
		ps.consume("RememberObjects")
	case "Card.IsImprinted":
		if ps.peek("ImprintCards") != "Targeted" {
			return lRef{}, refuse("ChooseSource Effect ImprintCards$ %q", ps.peek("ImprintCards"))
		}
		ps.consume("ImprintCards")
	default:
		return lRef{}, refuse("ChooseSource Effect replacement shape")
	}
	for _, k := range []string{"ExileOnMoved", "ForgetOnMoved"} {
		if v, ok := ps.get(k); ok && strings.TrimSpace(v) != "Battlefield" {
			return lRef{}, refuse("ChooseSource Effect %s$ %q", k, v)
		}
	}
	return lc.objOrTarget(ps, "", zBattlefield)
}

// bodyIsSelfExile reports whether SVar name is exactly the Effect's own
// self-exile, DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile.
func (lc *lowerCtx) bodyIsSelfExile(name string) bool {
	m := etbParams(lc.face.SVars[name])
	return name != "" && len(m) == 4 && m["DB"] == "ChangeZone" && m["Defined"] == "Self" &&
		m["Origin"] == "Command" && m["Destination"] == "Exile"
}

// opChooseSource builds OpExt ExtChooseSource, C the pool mask.
func (ic *instrCtx) opChooseSource() error {
	ki := &ic.ki
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChooseSource)), int16(ic.in.C)
	return nil
}
