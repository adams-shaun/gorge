package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// amass is OpAmass (gorge effAmass, CR 701.55): the amasser's first
// battlefield permanent (their battlefield list order) that is an Army
// creature by its current characteristics (a changeling and a granted Army
// type count: gorge "Creature.Army") gets the riders; with none, the Army token(s) of
// card C are created (a token replacement may make several, each gets the
// riders). The riders: A +1/+1 counters (one counter event, replaceable,
// CounterAddedOnce) and the permanent "also a <Type>" override; with
// RememberAmass$ the Army is remembered.
//
//go:noinline
func (s *State) amass(k *prep, in *kop.Instr) {
	n := s.value(k, in.A)
	if n <= 0 {
		return
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	army := -1
	for i, name := range k.t.Subtypes {
		if name == "Army" {
			army = i
			break
		}
	}
	if army >= 0 {
		var ord [MaxPerm]Slot
		m := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
		for _, sl := range ord[:m] {
			// an Army CREATURE by its current characteristics (gorge
			// MatchesSpecFrom "Creature.Army"): a layer-4 type
			// grant and a changeling (CR 702.73a) both count
			if s.typesGen(k, sl)&kop.TCreature != 0 && s.hasSubGen(k, sl, uint8(army)) {
				s.amassRiders(k, in, sl, n)
				return
			}
		}
	}
	tc, t, ok := s.replTokensCard(k, p, CardID(in.C), 1) // a rewriter's card (repl_tokrewrite.go)
	if !ok {
		return
	}
	for i := 0; i < t; i++ {
		if s.nearCapacity(k) {
			s.stall(StallBoard)
			return
		}
		o := s.newToken(tc, p)
		sl := s.enter(k, o, p, false, kop.ZoneLibrary)
		if k.gen.anyTokCr {
			s.fireTokenCreated(k, sl)
		}
		s.res.lastTok = o
		s.amassRiders(k, in, sl, n)
	}
}

func (s *State) amassRiders(k *prep, in *kop.Instr, sl Slot, n int) {
	m := s.placeCounters(k, sl, int(kop.CtrP1P1), n, s.perms[sl].ctrl, ctrPutEffect)
	if m > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, int(kop.CtrP1P1))
	}
	s.g.lastCK = kop.CtrP1P1
	if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
	}
	if ov := int(uint16(in.B.N) & 0xfff); ov > 0 {
		s.applyOverride(k, sl, ov-1, s.cardAt(sl), false)
	}
	if uint16(in.B.N)&0x8000 != 0 {
		s.remember(s.perms[sl].obj)
	}
}
