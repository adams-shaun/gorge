package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cardPTEqV is PredPowerGEV / PredToughnessLEV with PredN 1 on a spell: its
// power (toughness) equals the value PredV (gorge powerEQ<X> /
// toughnessEQ<X> through specCtx's Resolve: Talion, the Kindly Lord's
// powerEQChosen). A card that prints no power or toughness never matches,
// even a 0 (CR 208.3; gorge binds a P/T-less object's compare to false); a
// face-down spell is a 2/2 (CR 708.2a). ctx is the bound's source context,
// nil for the entry's own (ctxCur).
//
//go:noinline
func (s *State) cardPTEqV(k *prep, fl *kop.Filter, c CardID, owner uint8, o Obj, fd bool, ctx *evalCtx) bool {
	if k.t.Cards[c].NoPT && !fd {
		return false
	}
	s.offZonePTRead(k, c, owner, o) // static_offzonept.go
	pc := &k.cards[c]
	pw, t := int(pc.power), int(pc.tough)
	if fd {
		pw, t = 2, 2
	} else if k.cda && k.gen.has(c, gfCDAPT) {
		pw, t = s.cardCDAPT(k, c, owner, o)
	}
	cx := s.ctxCur()
	if ctx != nil {
		cx = *ctx
	}
	v := s.evalV(k, fl.PredV, cx)
	if fl.Pred == kop.PredPowerGEV {
		return pw == v
	}
	return t == v
}

// cardPowerGEV is PredPowerGEV with PredN 0 on a card off the battlefield:
// its power is at least the value PredV, the printed power (a face-down card
// a 2/2), or the last known power while matchTrigLKI reads a departed
// trigger object (CR 608.2h). A card that prints no power never matches.
//
//go:noinline
func (s *State) cardPowerGEV(k *prep, fl *kop.Filter, c CardID, owner uint8, o Obj, fd bool, ctx *evalCtx) bool {
	if s.xBoundPending(fl) {
		return true
	}
	if k.t.Cards[c].NoPT && !fd {
		return false
	}
	s.offZonePTRead(k, c, owner, o) // static_offzonept.go
	pw := int(k.cards[c].power)
	if fd {
		pw = 2
	} else if k.cda && k.gen.has(c, gfCDAPT) {
		pw, _ = s.cardCDAPT(k, c, owner, o)
	}
	if s.g.tli&tliOn != 0 {
		pw, _ = s.lkiPT()
	}
	cx := s.ctxCur()
	if ctx != nil {
		cx = *ctx
	}
	if fl.PredN == 2 {
		return pw <= s.evalV(k, fl.PredV, cx) // a graveyard card's power at most the value (Ruthless Technomancer's powerLEX)
	}
	return pw >= s.evalV(k, fl.PredV, cx)
}

// cardToughnessGEV is PredToughnessLEV with PredN 3 on a card off the
// battlefield: its toughness is at least the value PredV, the printed
// toughness (a face-down card a 2/2), or the last known toughness while
// matchTrigLKI reads a departed trigger object (CR 608.2h; Hulkling,
// Burgeoning Bruiser's toughnessGTZ as >= Z+1). A card that prints no
// toughness never matches.
//
//go:noinline
func (s *State) cardToughnessGEV(k *prep, fl *kop.Filter, c CardID, owner uint8, o Obj, fd bool, ctx *evalCtx) bool {
	if s.xBoundPending(fl) {
		return true
	}
	if k.t.Cards[c].NoPT && !fd {
		return false
	}
	s.offZonePTRead(k, c, owner, o) // static_offzonept.go
	t := int(k.cards[c].tough)
	if fd {
		t = 2
	} else if k.cda && k.gen.has(c, gfCDAPT) {
		_, t = s.cardCDAPT(k, c, owner, o)
	}
	if s.g.tli&tliOn != 0 {
		_, t = s.lkiPT()
	}
	cx := s.ctxCur()
	if ctx != nil {
		cx = *ctx
	}
	return t >= s.evalV(k, fl.PredV, cx)
}
