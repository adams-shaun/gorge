package kgen

import "strings"

// attachObject lowers an Attach whose Object$ names another object than
// the source (gorge effAttach's definedSpec Object$, the first object it
// resolves): one known object -- the node's own target (Targeted,
// ThisTargetedCard), the chain's earlier target (ParentTarget) or the
// trigger's card -- attached to one destination, the node's target or a
// Defined$ object. gorge's Attachable is always true (the attachment SBAs
// own legality); an object no longer on the battlefield, or the
// destination itself, attaches nothing. Optional$, Choices$, Move$,
// RememberAttached$ and plural Object$ selectors (Valid ..., AttachedTo
// ...) are refused.
//
// Object$ ParentTarget alone is plural (gorge attachObjectParentTarget: every
// object of the parent link's answered set, Thorin, Mountain-king's "attach
// any number of target Equipment"): its object slot may take several objects,
// which the kernel's AttachObj attaches each, and RememberAttached$ True
// remembers each object attached (B = 1).
func (lc *lowerCtx) attachObject(ps *params, o string) error {
	if strings.HasPrefix(o, attachGroupPrefix) {
		return lc.attachGroupChoices(ps, o) // lower_attachgroup.go
	}
	remember := false
	if o == "ParentTarget" {
		if v, ok := ps.get("RememberAttached"); ok {
			if v != "True" {
				return refuse("Attach Object %q with RememberAttached$ %q", o, v)
			}
			ps.consume("RememberAttached")
			remember = true
		}
	}
	for _, k := range []string{"Optional", "Choices", "Move", "RememberAttached", "PlayerChoices", "Unattach"} {
		if _, ok := ps.get(k); ok && !(k == "RememberAttached" && remember) {
			return refuse("Attach Object %q with %s$", o, k)
		}
	}
	var obj, dest lRef
	var err error
	_, hasT := ps.get("ValidTgts")
	switch o {
	case "Targeted", "ThisTargetedCard":
		if !hasT {
			return refuse("Attach Object %q without ValidTgts$", o)
		}
		if _, ok := ps.get("Defined"); !ok {
			return refuse("Attach Object %q without Defined$", o)
		}
		d := ps.str("Defined")
		ps.consume("Defined")
		tp := map[string]string{}
		for _, k := range []string{"ValidTgts", "TargetMin", "TargetMax", "TgtPrompt"} {
			if v, ok := ps.get(k); ok {
				tp[k] = v
			}
		}
		if obj, err = lc.objOrTarget(newParams(tp), "", zBattlefield); err != nil {
			return err
		}
		if dest, err = lc.defined(d); err != nil {
			return err
		}
	case "ParentTarget", "TriggeredCard", "TriggeredCardLKICopy":
		if obj, err = lc.defined(o); err != nil {
			return err
		}
		if dest, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	case "Remembered":
		if hasT {
			return refuse("Attach Object %q with ValidTgts$", o)
		}
		if ps.str("Defined") != "Self" {
			return refuse("Attach Object %q without Defined$ Self", o)
		}
		ps.consume("Defined")
		if !lc.kattremOneWriter() {
			return refuse("Attach Object Remembered over a multi-object remembered set")
		}
		obj, dest = lRef{Kind: rRemembered}, lRef{Kind: rSelf}
	default:
		return refuse("Attach Object %q", o)
	}
	for _, r := range []lRef{obj, dest} {
		switch r.Kind {
		case rSelf, rTriggered, rRemembered:
		case rTarget:
			if f := lc.ab.Targets[r.N].F; f == nil || len(f.Clauses) == 0 || lc.ab.Targets[r.N].Zone != zBattlefield {
				return refuse("Attach Object %q: target slot %d is not a permanent", o, r.N)
			}
			if mx := lc.ab.Targets[r.N].Max; (!mx.isConst() || mx.N > 1) && !(o == "ParentTarget" && r == obj) {
				// "any number of target Equipment" attaches them all
				return refuse("Attach Object %q: target slot %d takes several objects", o, r.N)
			}
		default:
			return refuse("Attach Object %q: ref %s", o, r)
		}
	}
	if obj == dest {
		return refuse("Attach Object %q onto itself", o)
	}
	in := lInstr{Op: "AttachObj", Obj: dest, Obj2: obj}
	if remember {
		in.Note = attachRememberNote
	}
	lc.emit(in)
	return nil
}

// attachRememberNote marks an AttachObj that remembers each object it
// attached (RememberAttached$ True; kremWrites sees "remember").
const attachRememberNote = "remember attached"

// attachChoices lowers `DB$ Attach | Choices$ <filter> | Defined$ <dest>`
// with no Object$ (Zack Fair, Vow to Erebor, Winter Soldier): "(you may)
// attach an Equipment you control to it" is the object-side pool of gorge's
// effAttach. The resolving controller picks one battlefield permanent the
// filter admits (Optional$ True: up to one), and the pick attaches to the
// Defined$ destination (Targeted or Self, one object). The ChooseCard
// carries the chooseAttachPick codes so the kernel reads the destination
// from the AttachObj after it: candidates are narrowed to those gorge's
// destCandidatesFor admits, no ask for none or (mandatory) one. Optional$
// is the pick's Min 0, not a yes/no, so mayGate is not used here.
// RememberAttached$, ChoiceZone$, Chooser$ and PlayerChoices$ are refused.
func (lc *lowerCtx) attachChoices(ps *params) error {
	for _, k := range []string{"RememberAttached", "ChoiceZone", "Chooser", "PlayerChoices", "Move", "Unattach"} {
		if _, ok := ps.get(k); ok {
			return refuse("Attach Choices$ with %s$", k)
		}
	}
	opt := false
	if v, ok := ps.get("Optional"); ok {
		if v != "True" {
			return refuse("Attach Choices$ with Optional$ %q", v)
		}
		opt = true
	}
	if d := ps.str("Defined"); d != "Targeted" && d != "Self" {
		return refuse("Attach Choices$ with Defined$ %q", d)
	}
	f, err := lc.filter(ps.str("Choices"))
	if err != nil {
		return err
	}
	ps.consume("Choices", "ChoiceTitle")
	dest, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	in := lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: f, C: zBattlefield, CS: "AttachPick"}
	if opt {
		in.CS = "AttachPickOpt"
	}
	lc.emit(in)
	lc.emit(lInstr{Op: "AttachObj", Obj: dest, Obj2: lRef{Kind: rChosen}})
	return nil
}
