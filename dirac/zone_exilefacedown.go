package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The face-down may-look exile of a Dig (CR 406.3: cards
// exiled face down are turned up only by an effect that says so, CR 406.3a-b).
// A Dig into exile with ExileFaceDown$ and WithMayLook$ (kop.DigFaceDown) goes
// through gorge's applyMoveFaceDown (engine/effects/zone_change.go:1286-1335,
// called by the Dig mover's emitEntryMove): the move is stamped
// exiled_with_face_down_maylook, the looker the resolving controller, and the
// fold (engine/events/apply_zone.go:222-240) sets Object.FaceDown, MayLookPlayer
// and ExiledWith. Every later move clears them (apply_zone.go:144-150). The
// lockstep projection carries FaceDown on an exiled object (dirac/oracle
// project.go FFaceDown), so the mark is state, not a view.
//
// It rides the exiled-with link pool (zone_exiledwith.go) as two bits of
// exwRec.fl: exwFace (the card is exiled face down) and exwLook1 (the looker
// is player 1). No State byte. Readers that gorge makes skip a face-down
// object (hasXCost, faceUp, the exile ability walk) are not reached by this
// pool: the kernel lowers no ActivationZone$ Exile ability and no CMC-partial
// filter over exile, so there is no further reader to agree with.

const (
	exwFace  uint8 = 1 << 3 // the card is exiled face down (a Dig's ExileFaceDown$ WithMayLook$)
	exwLook1 uint8 = 1 << 4 // its looker is player 1 (else player 0)
)

// DebugExileFaceUp is the mutation hook (TestExilefdMutation): the Dig's
// face-down exile is skipped, the card stays face up.
var DebugExileFaceUp bool

// ExileFDStats counts the lockstep coverage (TestExilefdLockstep).
type ExileFDStats struct {
	Exiled int // cards exiled face down
	By     [64]int
}

// DebugExileFDStats, when non-nil, accumulates the counters above.
var DebugExileFDStats *ExileFDStats

// faceDownExile marks o, just placed into exile by the resolving Dig src,
// face down with looker. It runs after the placement: exwExile's clearing of
// the reverse links never touches the face bits.
//
//go:noinline
func (s *State) faceDownExile(o, src Obj, looker uint8) {
	if DebugExileFaceUp || s.objs[o].zone != kop.ZoneExile {
		return
	}
	fl := exwFace
	if looker == 1 {
		fl |= exwLook1
	}
	s.exwPut(o, src, s.objs[src].ver, fl, -1)
	if st := DebugExileFDStats; st != nil {
		st.Exiled++
		if c := int(s.objs[src].card); c >= 0 && c < len(st.By) {
			st.By[c]++
		}
	}
}

// faceDownExiled: o is in exile face down.
func (s *State) faceDownExiled(o Obj) bool {
	for i := 0; i < int(s.g.nexw); i++ {
		if r := &s.cold.exw[i]; r.o == o && r.fl&exwFace != 0 {
			return true
		}
	}
	return false
}

// FaceDownLooker is the player who may look at the face-down exiled card o
// (gorge MayLookPlayer), or 255 when o is not exiled face down.
func (s *State) FaceDownLooker(o Obj) uint8 {
	for i := 0; i < int(s.g.nexw); i++ {
		if r := &s.cold.exw[i]; r.o == o && r.fl&exwFace != 0 {
			return r.fl >> 4 & 1
		}
	}
	return 255
}

// prepExileFD sets anyFaceExile: some program is a Dig with DigFaceDown.
func prepExileFD(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpDigEx && in.C>>3&kop.DigFaceDown != 0 {
					g.anyFaceExile = true
				}
			}
		}
	}
}
