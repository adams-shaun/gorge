package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters choice of another permanent (CR 614.12): an enters program
// opening with OpExt ExtChooseCardEnter (kgen lower_choosecardenter.go;
// Dauntless Bodyguard's ETB ChooseCard replacement, Defined$ You, Choices$
// Creature.YouCtrl+Other, Mandatory$ True). gorge runs the body once the
// permanent is on the battlefield (effects effChooseCard; the stack_resolve,
// move_zone, then the "choose" ask): a KChoose of "card" options over the
// matching battlefield objects other than the entrant, posed even with one
// candidate and not at all with none, the answer kept on Object.Chosen. The
// kernel never asks it with the spell on the stack (enterChoiceKind has no
// arm for it): the enters program queues it (akCard: runReplProgAs and the
// ExtChooseCardEnter arm of execExt, queueAskAt) as a ChGenChooseCard
// decision whose source and mode are the object, so the queued answer keeps
// a.Mode() as the Obj; the answer lives in the permanent's chosen byte (255:
// none chose, or no candidate). A permanent that left holds nothing (CR
// 400.7).
//
// An ability of the card reads the choice through the resolution's chosen
// set (RefChosen, F2ChosenCard; gorge's ChosenCard / ChosenCardStrict reads,
// which fall back to the source's chosen list): seedEnterChosen starts the
// set with it.
// The ability's Sac<1/CARDNAME> cost moved the source before it resolves, so
// the read is the entry's last known information of it (CR 608.2h; gorge
// TriggerContext.SourceChosen), never the slot, which another permanent may
// hold by then.

// akCard is the ask kind of ExtChooseCardEnter (genState.ask).
const akCard uint8 = 17

// enterCardOp is card c's ExtChooseCardEnter (the head of an enters
// replacement program), or nil.
func enterCardOp(k *prep, c CardID) *kop.Instr {
	if k.gen == nil || int(c) >= len(k.t.Cards) || !k.gen.has(c, gfEnterRepl) {
		return nil
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram && len(a.Prog) > 0 &&
			a.Prog[0].Op == kop.OpExt && a.Prog[0].Ext() == kop.ExtChooseCardEnter {
			return &a.Prog[0]
		}
	}
	return nil
}

// enterCards lists (into d, or just counts) the objects the permanent self,
// on the battlefield under p's control with card c's ExtChooseCardEnter, may
// choose: the battlefield objects matching the op's filter for p, the
// entrant excluded (Other). A pending object (a token or copy waiting for
// its own as-enters answer) is not on the battlefield yet.
//
//go:noinline
func (s *State) enterCards(k *prep, c CardID, self Obj, p uint8, d *Decision) int {
	in := enterCardOp(k, c)
	if in == nil {
		return 0
	}
	src := NoSlot
	if self != NoObj && s.objs[self].zone == kop.ZoneBattlefield {
		src = s.objs[self].slot
	}
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		o := s.perms[sl].obj
		if o == self || s.objs[o].of&ofPending != 0 || !s.matchPermGen(k, in.F, sl, p, src, layerFull) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}

// enterCardNone reports that the permanent at sl has no object to choose
// (gorge's empty KChoose is never posed: the permanent chose nothing).
func (s *State) enterCardNone(k *prep, sl Slot, c CardID) bool {
	return s.enterCards(k, c, s.perms[sl].obj, s.perms[sl].ctrl, nil) == 0
}

// enterCardOptions fills the first queued akCard ask.
//
//go:noinline
func (s *State) enterCardOptions(k *prep, d *Decision) {
	d.Sub = ChGenChooseCard
	if s.g.nask == 0 || s.g.ask[0].sl == NoSlot {
		return
	}
	a := &s.g.ask[0]
	c := a.card
	if c == NoCard {
		c = s.cardAt(a.sl)
	}
	s.enterCards(k, c, s.perms[a.sl].obj, SeatIx(d.Player), d)
}

// seedEnterChosen starts the resolving entry's chosen set with its source's
// as-enters chosen object, when the source card makes that choice: from the
// entry's last known information of the source once it left (the
// Sac<1/CARDNAME> cost), else from the source still on the battlefield as
// the incarnation that created the entry.
//
//go:noinline
func (s *State) seedEnterChosen(k *prep, e *stackEntry) {
	if e.kind == seSpell || e.obj == NoObj || enterCardOp(k, e.card) == nil {
		return
	}
	s.g.nchosen = 0
	ch := objWord(NoObj)
	if r := s.lk(e.li); e.li != 0 && r.obj == e.obj {
		ch = objWord(r.chosen)
	} else if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
		ch = s.perms[ob.slot].chosen
	}
	if ch != objWord(NoObj) && int(ch) < MaxObj {
		s.g.chosen[0], s.g.nchosen = Obj(ch), 1
	}
}
