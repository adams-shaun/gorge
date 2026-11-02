package kgen

import (
	"strings"
)

// revealHand lowers api:RevealHand (gorge effReveal's whole-hand arm: a
// reveal, or with Look$ True a private look, of the target or defined
// player's hand). A reveal has no rules effect in the kernel (CR 701.20;
// gorge records a Note), so the node keeps only its target slot and emits
// nothing. RememberRevealed$/RememberRevealedPlayer$ (the remembered hand),
// Optional$ (gorge's may-reveal ask) and NumCards$ stay unread.
func (lc *lowerCtx) revealHand(ps *params) error {
	look := ps.is("Look", "True")
	ps.consume("Look", "RevealType")
	q := ps
	if _, t := ps.m["ValidTgts"]; t && strings.TrimSpace(ps.m["Defined"]) == "Targeted" {
		// Defined$ Targeted names the node's own target: read the slot
		// from a view without it, then mark both read
		m := make(map[string]string, len(ps.m))
		for k, v := range ps.m {
			if k != "Defined" {
				m[k] = v
			}
		}
		q = newParams(m)
	}
	who, err := lc.objOrTarget(q, "You", zNone)
	if err != nil {
		return err
	}
	if err := lc.lookAck(ps, look, who); err != nil { // lower_handmove.go
		return err
	}
	if q != ps {
		ps.consume("Defined")
		for k := range q.used {
			ps.used[k] = true
		}
	}
	return nil
}

// seek lowers api:Seek (gorge effSeek): the controller (Defined$ You or
// absent) seeks Num$ cards (default 1) matching Type$ (default Card; a
// leading Permanent reads "permanent card") from their library;
// RememberFound$ True makes the found cards the remembered set. Types$,
// ImprintFound$, DefinedCards$ and other Defined$ stay
// unread.
func (lc *lowerCtx) seek(ps *params) error {
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("Seek Defined$ %q", d)
	}
	spec := ps.str("Type")
	if spec == "" {
		spec = "Card"
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	if f.Players != 0 {
		return refuse("Seek Type$ %q names players", spec)
	}
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Seek", Obj: lRef{Kind: rYou}, A: n, F: f}
	if ps.is("RememberFound", "True") {
		in.C = 1
	}
	lc.emit(in)
	return nil
}

// reveal lowers api:Reveal from the controller's own hand (gorge
// effReveal's pickable hand reveal): the pool is the hand's cards matching
// RevealValid$ (all without it), in hand order; NumCards$ (a constant,
// default 1) of them are revealed -- every one when the pool is no larger,
// else the owner picks exactly that many (reveal_pick); AnyNumber$ True
// picks any number. Optional$ True/You first asks yes/no, only when a card
// qualifies. RememberRevealed$ remembers the revealed cards (nothing moves:
// a reveal has no rules effect). Refused: another player's hand (Defined$,
// ValidTgts$), RevealDefined$, Random$, RevealAllValid$.
func (lc *lowerCtx) reveal(ps *params) error {
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("Reveal Defined$ %q", d)
	}
	spec := ps.str("RevealValid")
	if spec == "" {
		spec = "Card"
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("Reveal filter %s", f)
	}
	cs := []string{"from=Hand"}
	n := cst(1)
	if ps.is("AnyNumber", "True") {
		n = cst(255)
	} else {
		if n, err = lc.amount(ps, "NumCards", 1); err != nil {
			return err
		}
		if !n.isConst() || n.N < 1 {
			return refuse("Reveal NumCards$ %v", n)
		}
		cs = append(cs, "Mandatory")
	}
	if o, ok := ps.get("Optional"); ok {
		if o != "True" && o != "You" {
			return refuse("Reveal Optional$ %q", o)
		}
		cs = append(cs, "Confirm")
	}
	if ps.is("RememberRevealed", "True") {
		cs = append(cs, "Remember")
	}
	cs = append(cs, "Reveal")
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: n, F: f, C: zNone, CS: strings.Join(cs, "|")})
	return nil
}

// lookAck lowers RevealHand's Look$ True (a private look at a hand; gorge
// effReveal's look arm poses poseLookAck's one-option look_ack before the
// look, only when the hand holds a card and the look is neither Optional$
// nor Random$) to OpLookAck over the looked-at player.
func (lc *lowerCtx) lookAck(ps *params, look bool, who lRef) error {
	if !look {
		return nil
	}
	for _, k := range []string{"Optional", "RevealOptional", "Random", "NumCards"} {
		if _, ok := ps.get(k); ok {
			return refuse("RevealHand Look$ with %s$", k)
		}
	}
	lc.emit(lInstr{Op: "LookAck", Obj: who, Note: "look at the hand"})
	return nil
}
