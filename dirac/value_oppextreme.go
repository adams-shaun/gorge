package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// oppExtreme is ExOppExtreme: a figure over the living players other than
// ctrl, walked in seat order as gorge walks opponentGroup's slice (never a
// map). OppExtAmount counts them (gorge playerGroupCount); the rest take the
// highest, or with OppExtLowest the lowest, of one figure per opponent
// (gorge lifeExtreme, playerCountExtreme). An empty group reads 0: gorge
// reports a life extreme unresolvable there, and the group is empty only at
// the game's end, where no later read decides anything. OppExtNonTrigPl
// walks every living player but the trigger entry's trigger player instead
// (gorge nonTriggeredTargetGroup: with no trigger entry, every living player).
func (s *State) oppExtreme(k *prep, x *kop.Expr, ctx evalCtx) int {
	what := int(x.N) & 0xff
	lowest := int(x.N)&kop.OppExtLowest != 0
	skip := ctx.ctrl
	if int(x.N)&kop.OppExtAllPl != 0 {
		skip = 255
	}
	if int(x.N)&kop.OppExtCountCmp != 0 {
		return s.groupCountCmp(k, x, ctx, what, skip)
	}
	if what == kop.OppExtRemCtrl {
		return s.remCtrlCount(skip) // value_remctrl.go
	}
	if int(x.N)&kop.OppExtNonTrigPl != 0 {
		skip = 255
		if e := ctx.e; e != nil && e.kind == seTrigger {
			skip = e.trigPl
		}
	}
	best, seen := 0, false
	for p := uint8(0); p < s.nSeats(); p++ {
		if p == skip || s.out(p) {
			continue
		}
		v := 1
		pl := &s.pl[p]
		switch what {
		case kop.OppExtLife:
			v = int(pl.life)
		case kop.OppExtHand:
			v = int(pl.handN)
		case kop.OppExtGrave:
			v = 0
			for j := 0; j < int(pl.graveN); j++ {
				if s.objs[pl.grave[j]].of&ofToken == 0 {
					v++ // a token that died is no card (CR 111.7), as ExCountGraveyard
				}
			}
		case kop.OppExtValid:
			at := ctx
			at.ctrl = p
			v = s.countPerm(k, x.F, at)
		case kop.OppExtLifeLost:
			v = int(pl.lifeLost)
			if DebugOppLifeLostZero {
				v = 0
			}
		case kop.OppExtSpellsCast:
			v = int(s.g.hist[p][histSpells])
		}
		if what == kop.OppExtAmount {
			if !DebugOppCountOne || best == 0 {
				best++
			}
			continue
		}
		if !seen || (lowest && v < best) || (!lowest && v > best) {
			best, seen = v, true
		}
	}
	return best
}

// DebugOppLifeLostZero is a test-only mutation (TestLifeLostExtremeMutation):
// the life-lost extremes read no life lost.
var DebugOppLifeLostZero bool

// DebugOppCountOne is a test-only mutation: the controller's living
// opponents count at most one, as in a two-player game.
var DebugOppCountOne bool

// groupCountCmp is ExOppExtreme with OppExtCountCmp: the living players
// other than skip (255: every living player) whose figure compares x.Op
// against Sub[0], the bound read once from the controller's side (gorge
// hasPropertyStateBacked: hasPropertyCount over the group, each member's
// zone length against the one threshold). A two-player game reads the one
// opponent; a pod reads each of them (CR 102.2). The spell figure reads the
// caster's own turn tally (histSpells), so a group count reaches when any
// one opponent has cast that many, not the table total.
func (s *State) groupCountCmp(k *prep, x *kop.Expr, ctx evalCtx, what int, skip uint8) int {
	th := s.evalV(k, x.Sub[0], ctx)
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if p == skip || s.out(p) {
			continue
		}
		v := int(s.pl[p].handN)
		switch what {
		case kop.OppExtGraveAll:
			v = int(s.pl[p].graveN)
		case kop.OppExtSpellsCast:
			v = int(s.g.hist[p][histSpells])
		case kop.OppExtMemberExpr:
			v = s.memberFigure(k, x, ctx, p) // value_oppmemberexpr.go
		}
		var hit bool
		switch x.Op {
		case kop.CmpGE:
			hit = v >= th
		case kop.CmpGT:
			hit = v > th
			if DebugHasCardsGE { // lockstep mutation (value_hascards.go)
				hit = v >= th
			}
		case kop.CmpLE:
			hit = v <= th
		}
		if hit {
			n++
		}
	}
	return n
}
