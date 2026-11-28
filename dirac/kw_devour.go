package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// akDevour is the as-enters Devour pick (CR 702.82a): a ChSacrifice
// decision over the controller's matching permanents, Done always offered
// (gorge kwDevour's optional sacrifice, Min 0, Max the eligible count).
// The ask kinds continue across their files; 38 is akDrawPick.
const akDevour uint8 = 39

// askRec.ts of an akDevour ask: the count of this ask's own picks, in bits
// 8-14 (bits 0-2 and 15 are the generic marks, askLandPlay ... askETBDeferred,
// which ride the record untouched).
const (
	askDevourShift        = 8
	askDevourPicks uint16 = 0x7f << askDevourShift
)

// kw:Devour (CR 702.82a; gorge engine/cards/kw_devour.go). "As this enters,
// you may sacrifice any number of <type>s. It enters with N times that many
// +1/+1 counters on it." gorge expands the keyword into one Moved ETB
// replacement whose body is an optional sacrifice remembered for the counter
// behind it; kgen folds that exact chain into the lone instruction OpExt
// ExtEnterDevour (B = N, F = the sacrificed permanents' filter,
// lower_devour.go).
//
// gorge's replacement is a ReplacementResult$ Updated body, so it runs after
// the permanent has entered: the kernel queues the ask from the enters
// program once the permanent is on the battlefield (runReplProgAs, akDevour)
// and places the counters on it when the picks commit.
//
// The picks are one decision each over the unpicked matches in battlefield
// order, Done always offered; the picks stay on the battlefield until the
// answer commits and then leave as one batch in pick order (addSac/flushSac,
// CR 701.21a), so a permanent that sees another die sees them all go. The
// counters are placed the way the entry counters are (placeEntryCounters).

// devourEligible counts the controller's unpicked permanents in.F matches.
func (s *State) devourEligible(k *prep, in *kop.Instr, p uint8, src Slot) int {
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.g.sacM.has(sl) {
			continue
		}
		if s.sacMatch(k, in.F, sl, p, src) {
			n++
		}
	}
	return n
}

// devourOptions fills the pick: every unpicked match in battlefield order,
// then Done (Min 0). The player is the permanent's controller.
func (s *State) devourOptions(k *prep, d *Decision, in *kop.Instr, p uint8, src Slot) {
	d.Sub = ChSacrifice
	d.Player = p
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.g.sacM.has(sl) {
			continue
		}
		if s.sacMatch(k, in.F, sl, p, src) {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// devourEnter places the entry counters: B per sacrificed permanent.
func (s *State) devourEnter(k *prep, sl Slot, in *kop.Instr, n int) {
	if n <= 0 {
		return
	}
	s.placeEntryCounters(k, sl, int(kop.CtrP1P1), int(in.B.N)*n, s.perms[sl].ctrl)
}

// devourAnswer applies one answer of the queued ask (posed once the permanent
// has entered): true when the batch committed and the enters program goes on,
// false while the next pick is asked. The next pick goes back to the queue's
// head, so another entry's queued ask waits until this Devour settles (gorge
// settles each entry's Devour completely), and the counters count this ask's
// picks (askDevourPicks), not the whole pending sacrifice batch.
func (s *State) devourAnswer(k *prep, ask askRec, a Action) bool {
	in := &k.t.Cards[ask.card].Abilities[uint8(ask.ab)].Prog[ask.pc-1]
	p := s.perms[ask.sl].ctrl
	if a.Kind() != AkDone {
		s.addSac(s.objs[Obj(a.Source())].slot)
		if ask.ts&askDevourPicks == askDevourPicks {
			s.capFull(StallAsk) // past the pick count's field
			return true
		}
		ask.ts += 1 << askDevourShift // this ask's own picks
		if s.devourEligible(k, in, p, ask.sl) > 0 {
			s.requeueAskHead(ask)
			return false
		}
	}
	n := int(ask.ts&askDevourPicks) >> askDevourShift
	s.flushSac(k)
	s.devourEnter(k, ask.sl, in, n)
	return true
}

// requeueAskHead puts the popped ask back at the queue's head for the next
// answer; a full queue is a declared stall.
func (s *State) requeueAskHead(ask askRec) {
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return
	}
	copy(s.g.ask[1:s.g.nask+1], s.g.ask[:s.g.nask])
	s.g.nask++
	s.g.ask[0] = ask
}

// enterDevourExt is ExtEnterDevour run outside an enters program: the op has
// no effect of its own (runReplProgAs poses and settles it).
//
//kmanifest:ext ExtEnterDevour
func (s *State) enterDevourExt(k *prep, in *kop.Instr) (ok, done bool) {
	return true, true
}
