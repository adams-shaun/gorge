package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Cipher (CR 702.99; gorge engine/cards/kw_cipher.go, effects/cipher.go
// effCipher, rules/cipher.go cipherTailFor, rules/trigger_granted.go
// checkCipherTriggers and the queue's "__kwCipher:" push).
//
//   - the encode (CR 702.99a) is the resolving spell's last instruction: kgen
//     appends OpExt ExtCipherEncode to the printed program (lower_cipher.go),
//     as gorge splices DB$ Cipher onto the chain's tail. A copy never encodes
//     (gorge's spellTailSplices skips a stack copy), and a spell that left the
//     stack mid-resolution encodes nothing. The controller is asked once over
//     the creatures they control in battlefield order, Done declining (gorge's
//     KModes Min 0 Max 1, posed with one creature too; none asks nothing).
//   - a pick moves the card from the stack to exile (gorge's MoveZone
//     "encoded"), so finishResolve finds it gone, and records the link: an
//     osEncoded record in the object-set store, o the card and src the
//     creature at its ver. The link holds while the creature is that
//     incarnation on the battlefield (a departure bumps its ver: gorge clears
//     EncodedCards as it leaves) and the card stays in exile (detach drops the
//     card's records as it leaves exile: gorge's Move prunes it).
//   - each combat damage event a linked creature deals to a player queues one
//     trigger per encoded card, in encode order, after that source's face
//     triggers (gorge runs checkCipherTriggers after the face walk), under the
//     creature's controller: an abDelayed entry (obj the creature, trigObj the
//     card, trigAmt cipherTrigAmt), gorge's KeywordTriggerPush with no face
//     index.
//   - the trigger resolves as gorge's rebuilt body, DB$ Play Defined$
//     Remembered CopyCard$ WithoutManaCost$ Optional$: cipherProg's one
//     OpCastFree over the card (the copy cast, cast_copycast.go). A card that
//     left exile before the trigger resolves stalls (StallCopyCast): gorge's
//     Defined$ population still offers it from its new zone.

// cipherTrigAmt is the abDelayed entry's trigAmt marking a cipher trigger.
const cipherTrigAmt int16 = -11

// cipherProg is the cipher trigger's body: the optional free cast of a copy
// of the remembered encoded card in exile (C: zone | One | CopyCard).
var cipherProg = [1]kop.Instr{
	{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: int16(kop.ZoneExile) | 1<<3 | 1<<6},
}

// DebugCipherNoEncode is the mutation switch: the encode asks nothing.
var DebugCipherNoEncode bool

// DebugCipherKeepLink is the mutation switch: the link survives the
// creature leaving the battlefield (it is read at any ver).
var DebugCipherKeepLink bool

// CipherStats counts the lockstep coverage (TestCipherLockstep).
type CipherStats struct {
	Asked, Encoded, Triggered, Resolved int
}

// DebugCipherStats, when non-nil, accumulates the counters above.
var DebugCipherStats *CipherStats

// prepCipher sets genPrep.anyCipher: some program encodes. The trigger casts
// a copy, so the copy-cast machinery is on too.
func prepCipher(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && kop.ExtOp(in.A.N) == kop.ExtCipherEncode {
					g.anyCipher, g.anyCopyCast, g.anyPermCopy = true, true, true
				}
			}
		}
	}
}

// cipherCands lists the resolving controller's creatures in battlefield order.
func (s *State) cipherCands(k *prep, out *[MaxPerm]Slot) int {
	p := s.res.e.ctrl
	var m slotMask
	for c := s.ctrlM[p].and(s.occ); c.any(); c = c.rest() {
		if sl := c.first(); s.isCreature(k, sl) {
			m = m.with(sl)
		}
	}
	return s.battlefieldOrder(p, m, out[:], 0)
}

// cipherEncodeExt is ExtCipherEncode: the encode ask, or nothing.
//
//kmanifest:ext ExtCipherEncode
func (s *State) cipherEncodeExt(k *prep, in *kop.Instr) (ok, done bool) {
	e := &s.res.e
	if DebugCipherNoEncode || e.kind != seSpell || s.objs[e.obj].zone != kop.ZoneStack || s.objs[e.obj].of&ofCopy != 0 {
		return true, true
	}
	var buf [MaxPerm]Slot
	if s.cipherCands(k, &buf) == 0 {
		return true, true
	}
	if DebugCipherStats != nil {
		DebugCipherStats.Asked++
	}
	s.res.ask, s.res.askP = raGenCipher, e.ctrl
	return false, true
}

// cipherOptions poses the encode: each creature (mode its index), then Done.
//
//kmanifest:askopts raGenCipher
func (s *State) cipherOptions(k *prep, d *Decision) {
	d.Sub = ChGenCipher
	var buf [MaxPerm]Slot
	n := s.cipherCands(k, &buf)
	for i, sl := range buf[:n] {
		d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, uint8(i)))
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// cipherAnswer encodes the card on the picked creature, or declines.
//
//kmanifest:askapply raGenCipher
func (s *State) cipherAnswer(k *prep, a Action) {
	s.res.ask = raNone
	e := &s.res.e
	if a.Kind() == AkChoose {
		cr := Obj(a.Source())
		ob := &s.objs[e.obj]
		if co := &s.objs[cr]; co.zone == kop.ZoneBattlefield && ob.zone == kop.ZoneStack {
			s.touch(e.obj)
			ob.zone = kop.ZoneNone
			s.placeFromK(k, e.obj, kop.ZoneExile, false, kop.ZoneStack, false, false)
			if s.objs[e.obj].zone == kop.ZoneExile {
				s.osAdd(osEncoded, cr, e.obj)
				if DebugCipherStats != nil {
					DebugCipherStats.Encoded++
				}
			}
		}
	}
	s.continueRes(k, 0)
}

// cipherLive reports whether record r is a live encode link: the creature is
// the encoding incarnation on the battlefield and the card is in exile.
func (s *State) cipherLive(r *osRec) bool {
	if s.objs[r.o].zone != kop.ZoneExile {
		return false
	}
	if DebugCipherKeepLink {
		return s.objs[r.src].zone == kop.ZoneBattlefield
	}
	return s.osLiveNow(r)
}

// cipherHit queues the cipher triggers of combat damage source sl, which has
// just dealt combat damage to a player: one per card encoded on it, in
// encode order, under its controller.
func (s *State) cipherHit(sl Slot) {
	if !s.occ.has(sl) {
		return
	}
	p := &s.perms[sl]
	o := p.obj
	if s.out(p.ctrl) {
		return
	}
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			return
		}
		rec := &r[i]
		if rec.src != o || rec.osKind() != osEncoded || !s.cipherLive(rec) {
			continue
		}
		s.enqueue(o, s.objs[o].ver, s.cardAt(sl), abDelayed, p.ctrl, rec.o, p.ctrl, cipherTrigAmt)
		if DebugCipherStats != nil {
			DebugCipherStats.Triggered++
		}
	}
}

// cipherUnlink drops card o's encode links as it leaves exile (gorge's Move
// prunes EncodedCards).
func (s *State) cipherUnlink(o Obj) {
	s.osDrop(func(r *osRec) bool { return r.o == o && r.osKind() == osEncoded })
}

// cipherResolve resolves a cipher trigger: the free copy cast of its card.
func (s *State) cipherResolve(k *prep) {
	e := &s.res.e
	if s.objs[e.trigObj].zone != kop.ZoneExile {
		s.capFull(StallCopyCast) // gorge's Defined$ population offers it from its new zone
		return
	}
	if DebugCipherStats != nil {
		DebugCipherStats.Resolved++
	}
	s.g.nrem, s.g.remCnt = 0, 0
	s.g.rem[0] = e.trigObj
	s.remCtlClear(0)
	s.g.nrem = 1
	s.runProg(k)
}

// isCipherTrig reports whether e is a cipher trigger.
func isCipherTrig(e *stackEntry) bool { return e.ab == abDelayed && e.trigAmt == cipherTrigAmt }
