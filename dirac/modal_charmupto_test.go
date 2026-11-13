package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// charmUpToTable is vanilla's table with Test Charm a "choose up to V" charm:
// ModesPick 2 and ModesFew 2 (the minimum is 0), and ModesPickV the constant
// V (kop.ExConst stands in for the corpus's general count -- the graveyard
// count or the paid X a real carrier reads; the cap's evaluation is
// modal_charmcmp.go's charmCap either way).
func charmUpToTable(upto int) *kop.Table {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	t.Exprs = append([]kop.Expr(nil), v.Table.Exprs...)
	if len(t.Exprs) == 0 {
		t.Exprs = append(t.Exprs, kop.Expr{}) // Exprs[0] is unused
	}
	a := t.Cards[v.TestCharm].Abilities[0]
	a.NModes, a.ModesPick, a.ModesFew = 2, 2, 2
	t.Cards[v.TestCharm].Abilities = []kop.Ability{a}
	t.Cards[v.TestCharm].ModesPickV = kop.ValueRef{K: kop.VExpr, N: int16(len(t.Exprs))}
	t.Exprs = append(t.Exprs, kop.Expr{Kind: kop.ExConst, N: int16(upto)})
	return &t
}

// TestCharmUpToDynamic pins a dynamic "choose up to V" charm's offers: V = 2
// takes 0..2 picks (Done after every pose, the cap ends the ask), V = 1 one
// pick, and V = 0 none.
func TestCharmUpToDynamic(t *testing.T) {
	// V = 2: Done at the first pose, Done after one pick, the cap after two.
	sc := newScenTable(t, charmUpToTable(2), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	bear := sc.perm(v.GrizzlyBears, 1)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	if d := sc.s.Step(); d.Kind != DkModes {
		t.Fatalf("want the mode ask, got kind %d", d.Kind)
	} else if m, done := modeOpts(&d); m != 3 || !done {
		t.Fatalf("first pose %+v: want both modes and Done", d.Options[:d.N])
	}
	// zero picks is a legal answer: the spell resolves with no mode at all
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
	sc.resolveAll(t)
	if sc.s.objs[fc].zone != kop.ZoneGraveyard {
		t.Fatalf("the spell is in zone %d, want the graveyard", sc.s.objs[fc].zone)
	}
	if sc.s.objs[bear].zone != kop.ZoneBattlefield || sc.s.pl[1].life != 20 {
		t.Fatalf("a zero-pick resolution touched the game: bear %d life %d", sc.s.objs[bear].zone, sc.s.pl[1].life)
	}

	// V = 2 again, two picks: no Done between them is not required, but the
	// cap ends the ask after the second and both modes resolve.
	sc = newScenTable(t, charmUpToTable(2), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	bear = sc.perm(v.GrizzlyBears, 1)
	fc = sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	charmnumPickMode(t, sc, 0)
	if d := sc.s.Step(); d.Kind != DkModes {
		t.Fatalf("after one pick: want the second mode ask, got kind %d", d.Kind)
	} else if m, done := modeOpts(&d); m != 2 || !done {
		t.Fatalf("second pose %+v: want mode 1 and Done", d.Options[:d.N])
	}
	charmnumPickMode(t, sc, 1)
	d := sc.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("after two picks: decision kind %d, want the combined target ask", d.Kind)
	}
	// both targets are asked as one decision: the creature for mode 0, the
	// player for mode 1
	sc.do(t, AkTarget, scAny, TCode(bear))
	d = sc.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("after the creature: decision kind %d, want the player slot", d.Kind)
	}
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
	sc.resolveAll(t)
	if sc.s.objs[bear].zone != kop.ZoneGraveyard || sc.s.pl[1].life != 18 {
		t.Fatalf("both modes did not resolve: bear zone %d, opponent life %d", sc.s.objs[bear].zone, sc.s.pl[1].life)
	}

	// V = 1 ("up to one"): one pick ends the ask.
	sc = newScenTable(t, charmUpToTable(1), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	sc.perm(v.GrizzlyBears, 1)
	fc = sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	charmnumPickMode(t, sc, 1) // the player-damage mode
	if d := sc.s.Step(); d.Kind != DkTarget {
		t.Fatalf("after one pick at V=1: decision kind %d, want no further mode ask", d.Kind)
	}
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
	sc.resolveAll(t)
	if sc.s.pl[1].life != 18 {
		t.Fatalf("opponent life %d, want 18 after the one pick", sc.s.pl[1].life)
	}

	// V = 0 ("up to zero"): no mode is chosen -- the cast announces none and
	// resolves as nothing (CR 700.2a).
	sc = newScenTable(t, charmUpToTable(0), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	bear = sc.perm(v.GrizzlyBears, 1)
	fc = sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	if d := sc.s.Step(); d.Kind == DkModes {
		t.Fatalf("V=0: a mode ask was posed: %+v", d.Options[:d.N])
	}
	sc.resolveAll(t)
	if sc.s.objs[fc].zone != kop.ZoneGraveyard {
		t.Fatalf("V=0: the spell is in zone %d, want the graveyard", sc.s.objs[fc].zone)
	}
	if sc.s.objs[bear].zone != kop.ZoneBattlefield || sc.s.pl[1].life != 20 {
		t.Fatalf("V=0 touched the game: bear zone %d, opponent life %d", sc.s.objs[bear].zone, sc.s.pl[1].life)
	}
}
