package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the object-side Attach pick (gorge effAttach with
// Choices$ and no Object$: Zack Fair, Vow to Erebor, Winter Soldier). The
// resolving controller picks one battlefield permanent the filter admits and
// the AttachObj right after attaches it to its destination. gorge keeps a
// candidate only when it has a legal destination (destCandidatesFor: the
// destination is on the battlefield and is not the candidate itself;
// Attachable is always true, the attachment SBAs own the rest), poses no ask
// for an empty pool, takes the one candidate of a mandatory pick unasked, and
// otherwise asks one KChoose (Min 0 when Optional$ True) over the pool in
// battlefield order. The ChooseCard instruction carries C = chooseAttachPick
// (mandatory) or chooseAttachPickOpt (up to one: the ask adds a Done) and the
// destination is the NEXT instruction's Obj, resolved here once per use.
const (
	chooseAttachPick    = 4
	chooseAttachPickOpt = 5
)

// DebugAttachPickAlwaysAsk poses the ask even for the single candidate of a
// mandatory pick, which gorge takes unasked. Test-only
// (TestAttachchoicesMutation).
var DebugAttachPickAlwaysAsk bool

// attachPickScan walks the battlefield for the pick's candidates: the
// permanents f admits for chooser p, other than the destination of the
// AttachObj after instruction pc. It lists them into d (when non-nil) and
// returns their count and the first.
func (s *State) attachPickScan(k *prep, pc uint8, p uint8, d *Decision) (n int, first Obj) {
	prog := s.prog(k)
	dest := s.objRef(k, prog[int(pc)+1].Obj)
	if dest == NoSlot {
		dest = s.attachPickSelf(k, pc+1)
	}
	if dest == NoSlot {
		return 0, NoObj // a gone destination: gorge's destCandidatesFor is empty
	}
	f := prog[pc].F
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if sl == dest || !s.matchPermGen(k, f, sl, p, s.resSrcSlot(k), layerFull) {
			continue
		}
		o := s.perms[sl].obj
		if n == 0 {
			first = o
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n, first
}

// attachPick is OpChooseCard for the two attach-pick codes: no candidate,
// nothing chosen (the AttachObj then does nothing); a mandatory pick with
// one candidate takes it unasked; otherwise the ask is posed.
func (s *State) attachPick(k *prep, in *kop.Instr, p uint8) (ok, done bool) {
	n, first := s.attachPickScan(k, s.res.pc, p, nil)
	switch {
	case n == 0:
		return true, true
	case n == 1 && in.C == chooseAttachPick && !DebugAttachPickAlwaysAsk:
		s.g.chosen[0], s.g.nchosen = first, 1
		return true, true
	}
	s.res.ask, s.res.askP = raGenChooseCard, p
	return false, true
}

// attachPickOptions lists the ask's options: the candidates in battlefield
// order, and a Done for the up-to-one pick.
func (s *State) attachPickOptions(k *prep, d *Decision, pc, p uint8) {
	s.attachPickScan(k, pc, p, d)
	if s.prog(k)[pc].C == chooseAttachPickOpt {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
}

// attachPickSelf is the destination of the AttachObj at pc when it is the
// attach pick's (the instruction before is a ChooseCard with a pick code)
// and Defined$ Self: gorge's Defined resolves Self to the source's ObjID and
// accepts it while that is on the battlefield, so a source returned by the
// ability itself (Winter Soldier's "return this card from your graveyard to
// the battlefield, then attach") is the destination although it is a new
// object. NoSlot otherwise.
func (s *State) attachPickSelf(k *prep, pc uint8) Slot {
	prog := s.prog(k)
	if pc == 0 || prog[pc].Obj != kop.RefSelf || prog[pc-1].Op != kop.OpChooseCard ||
		prog[pc-1].C != chooseAttachPick && prog[pc-1].C != chooseAttachPickOpt {
		return NoSlot
	}
	if ob := &s.objs[s.res.e.obj]; ob.zone == kop.ZoneBattlefield {
		return ob.slot
	}
	return NoSlot
}
