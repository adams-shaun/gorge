package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugNoPlExtreme (dirac/oracle TestOneRingExtremeMutation): ExPlExtreme reads 0.
var DebugNoPlExtreme bool

// plExtreme is ExPlExtreme: the highest, over a living player group walked
// in seat order, of the member's filtered object count across the named
// zones (gorge effects/count_player.go playerCountExtremeZones). Each
// member is read from their OWN side: at.ctrl = p rebinds the spec's
// "YouCtrl"/"YouOwn" to the counted member (gorge c.SpecContext(p)) while
// every source-relative word keeps the resolution's source. The walked
// lists are the plain zone arrays in order (never a map), like gorge's
// g.Zone(zone, p). An empty group reads 0 (a count always has one answer;
// it is empty only at the game's end, where no later read decides
// anything). A token in a graveyard is no card (CR 111.7, 704.5d): every
// lowered spec names Card, so the matcher would drop it -- the walker skips
// one up front, as ExCountGraveyard does.
func (s *State) plExtreme(k *prep, x *kop.Expr, ctx evalCtx) int {
	if DebugNoPlExtreme {
		return 0
	}
	group := int(x.N) & 1
	zones := int(x.N) &^ 3
	best, seen := 0, false
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) || group == kop.PlExtOpponents && p == ctx.ctrl {
			continue
		}
		at := ctx
		at.ctrl = p
		v := 0
		pl := &s.pl[p]
		if zones&kop.PlExtGrave != 0 {
			for j := 0; j < int(pl.graveN); j++ {
				o := pl.grave[j]
				if s.objs[o].of&ofToken != 0 {
					continue
				}
				if s.matchCardGenAt(k, x.F, s.objs[o].card, p, p, o, &at) {
					v++
				}
			}
		}
		if zones&kop.PlExtLibrary != 0 {
			for _, o := range pl.lib[pl.libLo:pl.libHi] {
				if s.matchCardGenAt(k, x.F, s.objs[o].card, p, p, o, &at) {
					v++
				}
			}
		}
		if zones&kop.PlExtExile != 0 {
			for j := 0; j < int(pl.exileN); j++ {
				o := pl.exile[j]
				if s.matchCardGenAt(k, x.F, s.objs[o].card, p, p, o, &at) {
					v++
				}
			}
		}
		if !seen || v > best {
			best, seen = v, true
		}
	}
	return best
}
