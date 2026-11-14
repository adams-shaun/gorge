package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestThisTurnEnteredCardPower exercises the $CardPower / $CardToughness /
// $CardManaCost sum over one ring entry. The only corpus carrier of
// Count$ThisTurnEntered_...$CardPower is Genesis of the Daleks, which kgen
// refuses for the unrelated VillainousChoice API, so this arm cannot be
// locked against gorge in a generated pool; the values below are gorge's
// objectProperty (Face() plus P/T counters, CopyFace while the permanent is
// live, the card's own face after it leaves).
// enteredRing turns on the zone-entry ring for a table that does not ask for it.
func (s *State) enteredRing() {
	l := *s.cold.layout
	coldEntered.set(&l, maxEntered)
	l.size()
	s.resetCold(&l)
	s.g.enteredOn = true
}

func TestThisTurnEnteredCardPower(t *testing.T) {
	sc := newScen(t, Config{})
	o := sc.perm(v.GrizzlyBears, 0) // 2/2, mana value 2
	sl := sc.slot(o)
	e := &zoneEntry{o: o}
	if got := sc.s.enteredValue(sc.k, e, kop.PropCardPower); got != 2 {
		t.Fatalf("printed power: got %d, want 2", got)
	}

	p := &sc.s.perms[sl]
	p.addCtrN(sc.s, int(kop.CtrP1P1), 2)
	if got := sc.s.enteredValue(sc.k, e, kop.PropCardPower); got != 4 {
		t.Fatalf("power plus two +1/+1 counters: got %d, want 4", got)
	}

	// A live copy reads its copy face: Hill Giant is 3/3, mana value 4.
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = v.HillGiant
	if got := sc.s.enteredValue(sc.k, e, kop.PropCardPower); got != 5 {
		t.Fatalf("copied power plus counters: got %d, want 5", got)
	}
	if got := sc.s.enteredValue(sc.k, e, kop.PropCardToughness); got != 5 {
		t.Fatalf("copied toughness plus counters: got %d, want 5", got)
	}
	if got := sc.s.enteredValue(sc.k, e, kop.PropCardManaCost); got != 4 {
		t.Fatalf("copied mana value: got %d, want 4", got)
	}

	// The permanent dies: the copy face and the counters are gone, so the
	// entry sums the card's own printed power.
	sc.s.destroy(sc.k, sl)
	if sc.s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatalf("object zone %d, want the graveyard", sc.s.objs[o].zone)
	}
	if got := sc.s.enteredValue(sc.k, e, kop.PropCardPower); got != 2 {
		t.Fatalf("power after the permanent died: got %d, want 2", got)
	}
}

// TestThisTurnEnteredSaturates proves the ring stops loudly at maxEntered:
// the 25th add records nothing, sets enteredFull and stalls with
// StallEntered, so no generated table can read a truncated list.
func TestThisTurnEnteredSaturates(t *testing.T) {
	sc := newScen(t, Config{})
	o := sc.perm(v.GrizzlyBears, 0)
	sc.s.turn = 1
	sc.s.enteredRing()
	for i := 0; i < maxEntered; i++ {
		sc.s.noteEnter(o, kop.ZoneGraveyard, kop.ZoneBattlefield, true)
	}
	if sc.s.g.enteredN != maxEntered || sc.s.g.enteredFull || sc.s.g.stalled != 0 {
		t.Fatalf("after %d adds: n=%d full=%v stalled=%d", maxEntered, sc.s.g.enteredN, sc.s.g.enteredFull, sc.s.g.stalled)
	}
	sc.s.noteEnter(o, kop.ZoneGraveyard, kop.ZoneBattlefield, true)
	if !sc.s.g.enteredFull || sc.s.g.enteredN != maxEntered {
		t.Fatalf("overflow: n=%d full=%v, want n=%d full", sc.s.g.enteredN, sc.s.g.enteredFull, maxEntered)
	}
	if sc.s.g.stalled != StallEntered {
		t.Fatalf("overflow stall %d, want StallEntered %d", sc.s.g.stalled, StallEntered)
	}
}

// TestThisTurnEnteredRing pins the reader's three properties directly:
// a double entry counts twice, a moved-on entry stays listed where it
// entered, and a token that ceased is still listed. It also checks the
// departure-time wasCreature gate used by the Graveyard_from_Battlefield_
// Creature split.
func TestThisTurnEnteredRing(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path
	tbl.Filters = append(append([]kop.Filter(nil), v.Table.Filters...), kop.Filter{Types: kop.TCreature})
	fid := kop.FilterID(len(v.Table.Filters))
	sc := newScenTable(t, &tbl, Config{})
	sc.s.enteredRing()
	o := sc.perm(v.GrizzlyBears, 0)
	sc.s.noteEnter(o, kop.ZoneGraveyard, kop.ZoneBattlefield, true)
	sc.s.noteEnter(o, kop.ZoneGraveyard, kop.ZoneBattlefield, true)
	x := &kop.Expr{Kind: kop.ExThisTurnEntered, N: int16(kop.ZoneGraveyard), F: fid, F2: fid, From: int8(kop.ZoneBattlefield)}
	if got := sc.s.thisTurnEntered(sc.k, x, evalCtx{}); got != 2 {
		t.Fatalf("double entry: got %d, want 2", got)
	}
	// It moves on to exile: the two graveyard entries stay, and the move
	// itself is one exile entry.
	sc.s.placeFrom(o, kop.ZoneExile, false, kop.ZoneBattlefield, true)
	if got := sc.s.thisTurnEntered(sc.k, x, evalCtx{}); got != 2 {
		t.Fatalf("after moving on, graveyard entries: got %d, want 2", got)
	}
	y := *x
	y.N, y.From = int16(kop.ZoneExile), -1
	if got := sc.s.thisTurnEntered(sc.k, &y, evalCtx{}); got != 1 {
		t.Fatalf("exile entries: got %d, want 1", got)
	}
	// A token that ceased is still listed (gorge keeps the entry).
	tok := sc.obj(v.GrizzlyBears, 0)
	sc.s.objs[tok].of |= ofToken
	sc.s.noteEnter(tok, kop.ZoneGraveyard, kop.ZoneBattlefield, true)
	sc.s.objs[tok].zone = kop.ZoneNone
	if got := sc.s.thisTurnEntered(sc.k, x, evalCtx{}); got != 3 {
		t.Fatalf("ceased token: got %d, want 3", got)
	}
	// The wasCreature bit gates the Creature-base alternative: a permanent
	// that was not a creature when it left does not match it.
	w := sc.obj(v.GrizzlyBears, 0)
	sc.s.noteEnter(w, kop.ZoneHand, kop.ZoneBattlefield, false)
	z := &kop.Expr{Kind: kop.ExThisTurnEntered, N: int16(kop.ZoneHand), F: fid, F2: fid, From: int8(kop.ZoneBattlefield)}
	if got := sc.s.thisTurnEntered(sc.k, z, evalCtx{}); got != 0 {
		t.Fatalf("non-creature departure matched the creature split: got %d, want 0", got)
	}
	sc.s.noteEnter(w, kop.ZoneHand, kop.ZoneBattlefield, true)
	if got := sc.s.thisTurnEntered(sc.k, z, evalCtx{}); got != 1 {
		t.Fatalf("creature departure: got %d, want 1", got)
	}
}

// TestThisTurnEnteredBattlefieldCountsACeasedCopyToken pins CR 111.1 /
// 700.14 / 603.10a for dest=Battlefield: a token copy that entered and has
// since ceased (zone ZoneNone) still counts for Idol of Oblivion's
// Count$ThisTurnEntered_Battlefield_Card.tokenCreated+YouCtrl lowering --
// F is the permanent-base filter, F4 the Card-base filter, and From is -1
// (zero would mean "from ZoneNone" and return 0 for the wrong reason). A
// plain token is the control: it counted before the fix and must keep
// counting. gorge pairs this in
// engine/effects.TestThisTurnEnteredCountsADeadTokenCopyEntry.
func TestThisTurnEnteredBattlefieldCountsACeasedCopyToken(t *testing.T) {
	for _, tc := range []struct {
		name string
		copy bool
	}{
		{"copy", true},
		{"plain", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl := *v.Table
			f := kop.Filter{Flags: kop.FToken, Ctrl: kop.CtrlYou}
			tbl.Filters = append(append([]kop.Filter(nil), v.Table.Filters...), f, f)
			fid := kop.FilterID(len(v.Table.Filters))
			f4 := kop.FilterID(len(v.Table.Filters) + 1)
			sc := newScenTable(t, &tbl, Config{})
			sc.s.enteredRing()
			o := sc.perm(v.GrizzlyBears, 0)
			sc.s.objs[o].of |= ofToken
			if tc.copy {
				sc.s.objs[o].of |= ofCopy
			}
			sc.s.noteEnter(o, kop.ZoneBattlefield, kop.ZoneLibrary, true)
			if sc.s.g.enteredN != 1 {
				t.Fatalf("precondition: ring holds %d entries, want 1", sc.s.g.enteredN)
			}
			// The object has left: ZoneNone is off both the battlefield and
			// the stack, the shape ceasedCopy rejects for a copy.
			sc.s.objs[o].zone = kop.ZoneNone
			if got := sc.s.objs[o].of; got&ofToken == 0 || (got&ofCopy != 0) != tc.copy {
				t.Fatalf("precondition: obj flags %#x, want token=%v copy=%v", got, true, tc.copy)
			}
			x := &kop.Expr{Kind: kop.ExThisTurnEntered, N: int16(kop.ZoneBattlefield), F: fid, F4: f4, From: -1}
			if got := sc.s.thisTurnEntered(sc.k, x, evalCtx{}); got != 1 {
				t.Errorf("copy=%v: got %d, want 1", tc.copy, got)
			}
		})
	}
}
