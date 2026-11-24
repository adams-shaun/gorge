package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"strings"
)

// sacrificeOptional reads Sacrifice's Optional$ (gorge effSacrifice: a
// non-strict optional asks 0..min(Amount, eligible) over the matching
// permanents; an object target's ask is the same 0..1 shape). Any value
// but True refuses, and the Optional$+StrictAmount$ disjoint
// {0, Amount} shape (gorge's KModes yes/no then an exact batch) refuses:
// the corpus carries it on one card, whose follow-up token lines (its
// RememberSacrificed$ conditions) refuse anyway.
func (lc *lowerCtx) sacrificeOptional(ps *params, in *lInstr) error {
	v, ok := ps.get("Optional")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("Optional$ %q on a sacrifice", v)
	}
	if sa, ok := ps.get("StrictAmount"); ok {
		return refuse("Optional$ with StrictAmount$ %q on a sacrifice", sa)
	}
	in.C |= int(kop.OptAsk)
	return nil
}

func (lc *lowerCtx) discard(ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "NumCards", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Discard", Obj: who, A: n}
	// Optional$ True (gorge discardBounds: the pick's min lowers to 0; the
	// TgtChoose arm poses a may-election first, chooseMode's Min 0 pick
	// needs none). Any value but True refuses. AnyNumber$ rides the
	// same bounds (Min 0, Max the eligible count, no election); the corpus
	// population is Optional$-only, so AnyNumber$ without Optional$ keeps
	// refusing.
	if v, ok := ps.get("Optional"); ok {
		if v != "True" {
			return refuse("Optional$ %q on a discard", v)
		}
		in.C |= int(kop.OptAsk)
		if an, ok := ps.get("AnyNumber"); ok {
			if an != "True" {
				return refuse("AnyNumber$ %q on a discard", an)
			}
			in.C |= int(kop.OptAny)
		}
	}
	switch m := ps.str("Mode"); m {
	case "TgtChoose", "":
		in.CS = "DiscarderChooses"
	case "RevealYouChoose", "LookYouChoose", "YouChoose":
		// gorge effDiscard chooseMode: the three are one path (the
		// controller picks from the discarder's hand; reveal vs look is
		// information only)
		in.CS = "YouChoose"
	case "Hand":
		// gorge effDiscard Mode$ Hand: the whole hand in hand order, no
		// ask, NumCards$ never read (the corpus carries none). Optional$
		// True is a per-player may-election whose "yes" discards the whole
		// hand (effDiscardHand); the corpus population is each-player only,
		// whose per-player ask the kernel does not pose -- refused below.
		if _, ok := ps.get("NumCards"); ok {
			return refuse("discard mode Hand with NumCards")
		}
		if in.C&int(kop.OptAsk) != 0 && who.Kind != rYou {
			return refuse("Optional$ on a Hand discard of a non-You player (each-player election)")
		}
		in.A = cst(127)
		in.CS = "DiscarderChooses"
		if in.C&int(kop.OptAsk) != 0 {
			in.C |= int(kop.OptAll) // the whole-hand election discards every card
		}
	case "RevealDiscardAll":
		// gorge effDiscardRevealDiscardAll: every card DiscardValid$
		// allows, hand order, no ask (the kernel's filtered discard with
		// N >= hand size discards every match without an ask)
		if _, ok := ps.get("NumCards"); ok {
			return refuse("discard mode RevealDiscardAll with NumCards")
		}
		if _, ok := ps.get("DiscardValid"); !ok {
			return refuse("discard mode RevealDiscardAll without DiscardValid")
		}
		in.A = cst(127)
		in.CS = "DiscarderChooses"
	case "Random":
		// gorge effDiscardRandom (CR 701.8b): the engine RNG picks
		// NumCards$ cards from the whole hand, no ask. The kernel's
		// ChooseRandom arm reads the whole hand and poses no election, so
		// any rider that would narrow the eligible set or ask first
		// refuses; an each-player Defined$ has no kernel path for
		// ChooseRandom (interp.go's each-player branch is ChooseSelf only).
		if _, ok := ps.get("Optional"); ok {
			return refuse("discard mode Random with Optional$")
		}
		if _, ok := ps.get("AnyNumber"); ok {
			return refuse("discard mode Random with AnyNumber$")
		}
		if _, ok := ps.get("DiscardValid"); ok {
			return refuse("discard mode Random with DiscardValid$")
		}
		if who.Kind == rEachPlayer {
			return refuse("discard mode Random of an each-player Defined$")
		}
		in.CS = "Random"
	case "Defined":
		// gorge effDiscardDefined (cardflow.go: the arm under
		// effDiscardCodes' Defined): the cards DefinedCards$ names that are
		// still in the acting player's hand move, no ask, and NumCards$ is
		// never read (discardAndRemember per remembered card; Breathstealer's
		// Crypt's comment: the Remembered drawn card). The corpus's only
		// DefinedCards$ Remembered over a You discarder is
		// Fa'adiyah Seer's/Sindbad's reveal-and-maybe-discard chain: the
		// remembered cards lower to a filtered discard whose filter
		// Card.IsRemembered takes every hand match without an ask
		// (kop.OptDefined, act_discarddefined.go). The node's own Condition*
		// gate (ConditionDefined$ Remembered | ConditionPresent$
		// Card.nonLand | ConditionCompare$ EQ1, nodeCondition's
		// cRefMatches over RefRemembered) skips the discard unless exactly
		// one remembered card is a nonland -- the drawn card this chain
		// remembered. Every other DefinedCards$ spelling (Imprinted,
		// ValidHand, ChosenCard, Targeted) or a non-You discarder is its own
		// card's blocker.
		if who.Kind != rYou {
			return refuse("discard mode Defined of a non-You discarder")
		}
		dc, ok := ps.get("DefinedCards")
		if !ok {
			return refuse("discard mode Defined without DefinedCards$")
		}
		if strings.TrimSpace(dc) != "Remembered" {
			return refuse("discard mode Defined with DefinedCards$ %q", dc)
		}
		if _, ok := ps.get("NumCards"); ok {
			return refuse("discard mode Defined with NumCards$")
		}
		if _, ok := ps.get("Optional"); ok {
			return refuse("discard mode Defined with Optional$")
		}
		f, err := lc.filter("Card.IsRemembered")
		if err != nil {
			return err
		}
		in.F = f
		in.A = cst(1)
		in.CS = "DiscarderChooses"
		in.C |= int(kop.OptDefined)
	default:
		return refuse("discard mode %q", m)
	}
	if dv, ok := ps.get("DiscardValid"); ok {
		if in.F, err = lc.filter(dv); err != nil {
			return err
		}
		// gorge discardEligible (cardflow.go:442) matches DiscardValid$ with
		// matchesInOwnZone (filter.go:3382): a bare Permanent base is a
		// permanent card in hand (CR 110.4a), the kernel's
		// zone-scoped filter.
	}
	if who.Kind == rEachPlayer {
		// the kernel expands the unfiltered, non-optional, self-choosing
		// each-player discard player by player (act_eachdiscard.go);
		// a filtered, optional or controller-chooses shape still resolves
		// one player only
		if in.F != nil {
			return refuse("each-player discard with a filter (DiscardValid$)")
		}
		if in.C&int(kop.OptAsk) != 0 {
			return refuse("each-player optional discard")
		}
		if in.CS != "DiscarderChooses" {
			return refuse("each-player discard chooser %q", in.CS)
		}
	}
	if err := rememberDiscarded(ps, &in); err != nil {
		return err
	}
	if err := lc.discardUnless(ps, &in, who); err != nil { // lower_handmove.go
		return err
	}
	lc.emit(in)
	return nil
}

// chooseType lowers a mid-resolution ChooseType$ ("choose a creature
// type": gorge effects/choose.go effChooseType asks KChoose over the
// creature-type vocabulary). The answer is the resolution's chosen subtype,
// which a later filter's Creature.ChosenType reads (Kindred Charge's copy
// filter).
func (lc *lowerCtx) chooseType(ps *params) error {
	if ps.str("Type") == "Card" {
		return lc.chooseCardType(ps) // lower_choosetype.go
	}
	if ps.str("Type") == "Basic Land" {
		return lc.chooseBasicLandType(ps) // lower_choosetype.go
	}
	if strings.EqualFold(ps.str("Type"), "CreatureInTargetedDeck") {
		return lc.chooseCreatureInDeckType(ps) // lower_choosetype.go
	}
	if ps.str("Type") != "Creature" {
		return refuse("ChooseType shape")
	}
	return lc.chooseCreatureTypeMid(ps) // lower_choosetype.go
}
