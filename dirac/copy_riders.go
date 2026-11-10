package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Copy exception riders (CR 707.9b, 707.3, 613.1f):
// AddTypes$ and AddColors$ ride the layer-1 exception records of
// copy_exceptions.go (the template's records are inherited by a copy of the
// copy); PumpKeywords$ does not. It is a permanent keyword override record
// on each token (kop.CopyPumpShift), applied after the token enters and
// never copied.

// DebugCopyRidersPumpCopiable is a test mutation: the PumpKeywords grant is
// also pushed as a copy exception record, so a copy of the copy inherits
// it. Never set outside tests.
var DebugCopyRidersPumpCopiable bool

// DebugCopyRidersNoTypes is a test mutation: a copy exception's AddTypes is
// ignored. Never set outside tests.
var DebugCopyRidersNoTypes bool

// CopyRidersStats counts the copy exception records a game applied (test
// hook).
type CopyRidersStats struct {
	Own       int // copies whose op named an exception record
	Inherited int // copies of a template that carried an AddTypes or AddColors record
	Pumped    int // PumpKeywords$ grants applied
}

// DebugCopyRidersStats is the test hook; nil in normal play.
var DebugCopyRidersStats *CopyRidersStats

func (s *State) copyRidersCount(k *prep, src Slot, idx uint8) {
	st := DebugCopyRidersStats
	if idx != 0 {
		st.Own++
	}
	if src != NoSlot && s.fxM.has(src) {
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.sl == src && r.kind == fxCopyExc {
				if o := &k.t.Overrides[r.r.idx-1]; o.AddTypes != 0 || o.AddColors != 0 {
					st.Inherited++
					return
				}
			}
		}
	}
}

// copyExcTypes is the card types and supertypes slot sl's copy exceptions
// add (layer 1; the template's records first).
//
//go:noinline
func (s *State) copyExcTypes(k *prep, sl Slot) (m kop.TypeMask) {
	if DebugCopyRidersNoTypes {
		return
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			m |= k.t.Overrides[r.r.idx-1].AddTypes
		}
	}
	return
}

// copyExcTySlots is the slots whose copy exceptions add a card type: their
// types ride perm.gty (refreshTy).
//
//go:noinline
func (s *State) copyExcTySlots(k *prep) (m slotMask) {
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.kind != fxCopyExc {
			continue
		}
		if o := &k.t.Overrides[r.r.idx-1]; o.AddTypes != 0 || o.RemoveLegendary {
			m.set(r.sl)
		}
	}
	return
}

// copyCtrl is the controller of copy op in's tokens: the resolving entry's
// controller, or under kop.CopyNonRemCtrl (Fractured Identity's "each player
// other than its controller") the opponent of the copied remembered
// object's controller, its RememberLKI$ capture once it left the battlefield.
// The mint reads the entry's controller as the tokens' controller, so the
// callers set it for the op and put it back after; an ask the mint parks (an
// Aura copy's host, an as-enters choice) is that player's, and its resume
// sets it again.
//
//go:noinline
func (s *State) copyCtrl(in *kop.Instr) uint8 {
	if in.Op != kop.OpCopyAsTokenEx || uint16(in.B.N)&kop.CopyNonRemCtrl == 0 {
		return s.res.e.ctrl
	}
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return s.res.e.ctrl
	}
	ctrl := s.objs[o].owner()
	if s.objs[o].zone == kop.ZoneBattlefield {
		ctrl = s.perms[s.objs[o].slot].ctrl
	} else if l := s.remLKIRec(o); l != nil {
		ctrl = l.ctrl
	}
	return s.nextAlive(ctrl) // two live seats only (kop.TwoSeatInstr: CopyNonRemCtrl)
}

// copyPumpEnter grants token slot sl the PumpKeywords$ keyword named by the
// copy op in (B.N >> kop.CopyPumpShift: its KwMask bit + 1): a permanent
// keyword override record (the table's AddKw-only record for that keyword),
// not a copy exception; with kop.CopyPumpEOT an until-end-of-turn grant.
//
//go:noinline
func (s *State) copyPumpEnter(k *prep, sl Slot, in *kop.Instr) {
	kw := kop.KwMask(1) << (uint16(in.B.N)>>kop.CopyPumpShift&0x3f - 1)
	if in.B.N&kop.CopyPumpEOT != 0 {
		s.grantEOT(sl, kw)
		s.promoKw(k, sl, kw)
		s.touchSl(sl)
		return
	}
	for i := range k.t.Overrides {
		o := k.t.Overrides[i]
		if o.AddKw != kw {
			continue
		}
		o.AddKw = 0
		if o != (kop.Override{}) {
			continue
		}
		if i >= 255 {
			s.capFull(StallTables) // fx records name an override by a byte
			return
		}
		s.applyOverrideFl(k, sl, i, s.res.e.card, 0)
		if DebugCopyRidersStats != nil {
			DebugCopyRidersStats.Pumped++
		}
		if DebugCopyRidersPumpCopiable {
			s.copyExcPush(sl, uint8(i+1))
		}
		return
	}
	s.capFull(StallLowering) // kgen interns the record; absent means a broken table
}
