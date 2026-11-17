package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// prevXScen is a scenario whose table carries a Chant of Vitu-Ghazi-shaped
// instant (a scoped prevention of all damage dealt by creatures, with the
// gain rider), a creature and a plain burn instant.
type prevXScen struct {
	*scen
	chant, bear, burn CardID
	emb               uint8
}

func newPrevXScen(t *testing.T) *prevXScen {
	t.Helper()
	var ids prevXScen
	sc, _ := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}}, func(tbl *kop.Table, _ map[string]CardID) {
		noPrevEmblemTweak(&ids.emb)(tbl, nil)
		in := kop.ExtInstr(kop.ExtPreventDamageScope)
		in.C = int16(kop.PrevScopeAll | kop.PrevScopeSrcCreature | kop.PrevScopeGain)
		add := func(d kop.CardDef) CardID {
			tbl.Cards = append(tbl.Cards, d)
			return CardID(len(tbl.Cards) - 1)
		}
		ids.chant = add(kop.CardDef{Name: "Test Chant", Types: kop.TInstant, Abilities: []kop.Ability{{Prog: []kop.Instr{in}}}})
		ids.bear = add(kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
		ids.burn = add(kop.CardDef{Name: "Test Burn", Types: kop.TInstant})
	})
	ids.scen = sc
	return &ids
}

// resolveChant registers the Chant of seat 0 as its resolution does.
func (f *prevXScen) resolveChant() {
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.chant, 0), 0, f.chant
	f.s.prevScopeAdd(f.k, &f.k.t.Cards[f.chant].Abilities[0].Prog[0])
}

// creatureSrc makes a creature of seat 1 the resolving source of the damage.
func (f *prevXScen) creatureSrc() {
	bear := f.perm(f.bear, 1)
	e := &f.s.res.e
	e.kind, e.obj, e.srcVer, e.card, e.ctrl = seAbility, bear, f.s.objs[bear].ver, f.bear, 1
}

// burnSrc makes a burn spell of seat 1 the resolving source of the damage.
func (f *prevXScen) burnSrc() {
	e := &f.s.res.e
	e.kind, e.obj, e.card, e.ctrl = seSpell, f.obj(f.burn, 1), f.burn, 1
}

// TestPrevXGainScope: Chant's scoped record prevents the damage creatures
// would deal and its controller gains that much; a non-creature source is
// dealt; under a can't-prevent static, or beside a second scoped record, the
// kernel stalls StallPrevRider; DebugNoPrevRider keeps the prevention and
// drops the gain.
func TestPrevXGainScope(t *testing.T) {
	t.Run("creature source", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		f.resolveChant()
		if x := s.g.prevX[0]; x != pxRidGain {
			t.Fatalf("prevX[0] = %#x, want %#x (a gain rider for seat 0)", x, pxRidGain)
		}
		if s.g.prevCol[0].sc&(kop.PrevScopeGain) != 0 {
			t.Fatalf("the instruction-only bit reached the record's sc %#x", s.g.prevCol[0].sc)
		}
		f.creatureSrc()
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 0 {
			t.Fatalf("damageTo dealt %d, want 0 (prevented)", n)
		}
		if s.pl[1].life != life[1] || s.pl[0].life != life[0]+3 {
			t.Fatalf("life %d/%d from %d/%d: want the victim unharmed and the Chant's controller +3", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
		if s.StallReason() != 0 {
			t.Fatalf("stalled %s", StallName(s.StallReason()))
		}
	})
	t.Run("non-creature source", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		f.resolveChant()
		f.burnSrc()
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 3 {
			t.Fatalf("damageTo dealt %d, want 3", n)
		}
		if s.pl[1].life != life[1]-3 || s.pl[0].life != life[0] {
			t.Fatalf("life %d/%d from %d/%d: want the victim -3 and no gain", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
	})
	t.Run("can't be prevented", func(t *testing.T) {
		f := newPrevXScen(t)
		f.resolveChant()
		f.creatureSrc()
		embDurPut(f.scen, 0, NoObj, f.emb)
		if !f.s.noPrev(f.k, false) {
			t.Fatal("precondition: the can't-prevent static is not in force")
		}
		f.s.damageTo(f.k, PlayerCode(1), 3)
		if got := f.s.StallReason(); got != StallPrevRider {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevRider))
		}
	})
	t.Run("second scoped record", func(t *testing.T) {
		f := newPrevXScen(t)
		f.resolveChant()
		f.resolveChant()
		if f.s.g.prevX[0] == 0 || f.s.g.prevX[1] == 0 {
			t.Fatal("precondition: two rider records")
		}
		f.creatureSrc()
		f.s.damageTo(f.k, PlayerCode(1), 3)
		if got := f.s.StallReason(); got != StallPrevRider {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevRider))
		}
	})
	t.Run("mutation", func(t *testing.T) {
		DebugNoPrevRider = true
		defer func() { DebugNoPrevRider = false }()
		f := newPrevXScen(t)
		s := f.s
		f.resolveChant()
		f.creatureSrc()
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 0 {
			t.Fatalf("damageTo dealt %d, want 0 (still prevented)", n)
		}
		if s.pl[0].life != life[0] || s.pl[1].life != life[1] {
			t.Fatalf("life %d/%d from %d/%d: want no gain", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
	})
}
