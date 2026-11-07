package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// KC9: sweep, five grammar sub-classes. Cold and out of line.

// DebugIgnoreTapPower is the graveexile mutation switch: tapPowerValue returns 0,
// so a Crew/Saddle payment that needs the TapPowerValue bonus cannot be
// offered. TestGraveExileAndTapPowerMutation requires the lockstep to diverge.
var DebugIgnoreTapPower bool

// DebugIgnoreGraveExile is the graveexile mutation switch: the dual-origin walk
// drops the Exile half (both the hidden pick and ChangeZoneAll).
// TestGraveExilePickMutation and TestGraveExileChangeZoneAllMutation (kernel
// scenarios) require each half to stop offering/moving the exiled card.
var DebugIgnoreGraveExile bool

// tapPowerValue is the candidate's Mode$ TapPowerValue bonus (graveexile): only a
// Crew cost reads TapPowerCrew, only a Saddle cost TapPowerSaddle, so a
// Station tap-power cost never sees it.
//
//go:noinline
func (s *State) tapPowerValue(k *prep, sl Slot, flags kop.CostFlags) int {
	if DebugIgnoreTapPower {
		return 0
	}
	v := 0
	switch {
	case flags&kop.CostCrew != 0:
		v = int(k.t.Cards[s.cardAt(sl)].TapPowerCrew)
	case flags&kop.CostSaddle != 0:
		v = int(k.t.Cards[s.cardAt(sl)].TapPowerSaddle)
	}
	if v == 0 {
		return 0
	}
	// CR 613.1f: the bonus is the creature's own static, so a "loses all
	// abilities" effect removes it (gorge reads the same static through
	// activeStatics and stops matching once the source is blanked).
	if s.genLost(k, sl) {
		return 0
	}
	if v == int(kop.TapPowToughness) {
		// Value$ Toughness: the callers add this to
		// power, so the sum is the toughness.
		if DebugToughAssignOff {
			return 0
		}
		p, t := s.pt(k, sl)
		return t - p
	}
	return v
}

// graveExileCands adds a Graveyard,Exile hidden pick's candidates (graveexile):
// gorge's gameWide pool walks the origin zones in order (Graveyard, then
// Exile), each player in seat order, and matches ChangeType$ against the
// card. The fetch player is the controller, so an Owner$ clause reads the
// card's real owner against it.
//
//go:noinline
func (s *State) graveExileCands(k *prep, in *kop.Instr, d *Decision) {
	ctrl := s.res.e.ctrl
	nz := 2
	if DebugIgnoreGraveExile {
		nz = 1
	}
	for zi := 0; zi < nz; zi++ {
		for p := uint8(0); p < 2; p++ {
			pl := &s.pl[p]
			var list []Obj
			if zi == 0 {
				list = pl.grave[:pl.graveN]
			} else {
				list = pl.exile[:pl.exileN]
			}
			for _, o := range list {
				if s.picksHas(o) {
					continue
				}
				if s.matchCardGen(k, in.F, s.objs[o].card, s.objs[o].owner(), ctrl, o) {
					d.add(MakeAction(AkChoose, uint8(o), 0, 0))
				}
			}
		}
	}
}
