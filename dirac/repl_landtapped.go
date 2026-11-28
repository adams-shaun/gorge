package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtTurnRepl TurnReplEtbLandTapped (kop/turnrepl.go): "lands you control
// enter tapped this turn", an Effect's Moved replacement with a DB$ Tap |
// ETB$ True body over its controller's lands (Nahiri's Lithoforming). gorge
// registers it as a live effect-created replacement (effects/restrictions.go
// replacementMovedBodyLive's Tap arm) until end of turn. The kernel keeps a
// per-player count of resolutions in the cold buffer (etbCreRec.land, beside
// the extra-counter rider's count, ended with it at cleanup); enterGen taps each land
// that player's side enters while it is not zero. gorge applies the tap as
// the move completes and only then asks a shock land's own pay-life election
// (no CR 616.1 order ask: the land is tapped while the election is pending,
// and paying leaves it tapped), so enterGen taps after the entrant's own
// enters programs queued that election.

// useEtbLand: some program is ExtTurnRepl TurnReplEtbLandTapped, so the cold
// buffer holds the per-player count and enterGen taps the entering land.
var useEtbLand = newGenUse(tableHasEtbLandTapped)

// tableHasEtbLandTapped reports whether some ability's program carries the
// lands-enter-tapped turn record.
func tableHasEtbLandTapped(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtTurnRepl && in.C == kop.TurnReplEtbLandTapped {
					return true
				}
			}
		}
	}
	return false
}

// etbLandSet records one resolution for the resolving effect's controller.
func (s *State) etbLandSet() {
	if c := s.etbCreCold(); c != nil {
		if p := s.res.e.ctrl; c.land[p] < 255 {
			c.land[p]++
		}
	}
}

// etbLandEnter taps the land entering at sl when a record names its
// controller. A face-down entrant is no land (CR 708.2).
func (s *State) etbLandEnter(k *prep, sl Slot, p *perm, face uint8) {
	if !k.gen.uses.has(useEtbLand) || DebugTurnReplOff || face != 0 {
		return
	}
	c := s.etbCreCold()
	if c == nil || c.land[p.ctrl] == 0 || s.typesAt(k, sl)&kop.TLand == 0 {
		return
	}
	p.flags |= pfTapped
}
