package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// preReturnCost is preReturn for a card whose "until this leaves" source was
// sacrificed as a cast's or an activation's cost, outside any resolution
// (Werefox Bodyguard's "{1}{W}, Sacrifice Werefox Bodyguard" returning
// Talion, the Kindly Lord): gorge asks the card's as-enters choice with it
// still in exile, mid-payment, and the card enters once answered (CR 614.12,
// 610.3). The payment's sacrifice is its last step, so the kernel poses the
// ask once the build is done, before the state-based actions, the triggers
// and priority (poseReturnCost, answerReturnCost).
const preReturnCost uint8 = 8

// parkReturnCost parks returning card ret's as-enters choice kind when its
// source left as a cost paid outside a resolution. It reports whether the
// card waits in exile.
func (s *State) parkReturnCost(k *prep, srcCtrl uint8, o, ret Obj, kind uint8) bool {
	if s.res.active || s.ctl != ctlBuild || s.g.pre.state != 0 || kind == akNameLook {
		return false
	}
	s.g.pre = preAsk{obj: ret, kind: kind, state: preReturnCost}
	if k.gen.anyLinkCtr {
		s.linkRetStall(k, srcCtrl, o, ret)
	}
	return true
}

// poseReturnCost poses the parked choice (parkReturnCost) to the card's owner.
func (s *State) poseReturnCost(k *prep, d *Decision) bool {
	if s.g.pre.state != preReturnCost || s.ctl != ctlPriority || s.res.active {
		return false
	}
	if s.objs[s.g.pre.obj].zone != kop.ZoneExile {
		s.g.pre = preAsk{} // the card left exile meanwhile: nothing returns
		return false
	}
	s.g.ctlRet, s.ctl = ctlPriority, ctlGenAsk
	return s.genAskOptions(k, d)
}

// answerReturnCost applies the answer to the parked choice: the card enters
// from exile with it.
func (s *State) answerReturnCost(k *prep, a Action) bool {
	if s.g.pre.state != preReturnCost {
		return false
	}
	o := s.g.pre.obj
	s.g.pre.val = objWord(a.Mode())
	if s.g.pre.kind == akType {
		s.g.pre.val = objWord(typeVocabSub(k, int(a.Target())<<8|int(a.Mode())))
	}
	s.g.pre.state = 2
	if s.objs[o].zone == kop.ZoneExile {
		from := s.detach(o)
		s.enterFromGrantedZone(k, o, s.objs[o].owner(), false, from) // the enters program reads the choice
	}
	s.g.pre = preAsk{}
	s.ctl = s.g.ctlRet
	return true
}

// preAskPosed reports a pre-entry as-enters ask posed with its card off the
// battlefield: a spell's or a land's (state 1), or a cost-returned card's
// (preReturnCost). The option builders read the card from s.g.pre.
func (s *State) preAskPosed() bool {
	return s.g.pre.state == 1 || s.g.pre.state == preReturnCost
}
