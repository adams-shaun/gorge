package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const (
	xbBear  CardID = 0 // 2/2
	xbTwins CardID = 1 // 7/7, CardDef.ExtraBlocks
	xbPlain CardID = 2 // 7/7, no extra block
	xbA3    CardID = 3 // 3/3
	xbA4    CardID = 4 // 4/4
	xbLure  CardID = 5 // 2/2 lure
)

func xbScen(t *testing.T, cfg Config) *scen {
	t.Helper()
	tbl := &kop.Table{
		Cards: []kop.CardDef{
			{Name: "Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			{Name: "Twins", Types: kop.TCreature, Power: 7, Toughness: 7, ExtraBlocks: 1},
			{Name: "Plain", Types: kop.TCreature, Power: 7, Toughness: 7},
			{Name: "Three", Types: kop.TCreature, Power: 3, Toughness: 3},
			{Name: "Four", Types: kop.TCreature, Power: 4, Toughness: 4},
			{Name: "Lure", Types: kop.TCreature, Power: 2, Toughness: 2, Lure: true},
		},
		Filters: []kop.Filter{{}},
		Conds:   []kop.Cond{},
	}
	sc := newScenTableLib(t, tbl, xbBear, cfg)
	if sc.k.gen == nil || !sc.k.gen.anyExtraBlock {
		t.Fatal("the table is not a generated one with anyExtraBlock")
	}
	if len(sc.s.xblkRecs()) != MaxPerm {
		t.Fatalf("xblk records = %d, want %d", len(sc.s.xblkRecs()), MaxPerm)
	}
	return sc
}

// xbAttack declares atk as attackers and stops at the blocker declaration.
func xbAttack(t *testing.T, sc *scen, atk ...Obj) {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	for _, a := range atk {
		sc.do(t, AkAttack, TCode(a), scAny)
	}
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepBlockers)
}

func xbOffered(d Decision, b, a Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if o := d.Options[i]; o.Kind() == AkBlock && o.Source() == TCode(uint8(b)) && o.Target() == TCode(uint8(a)) {
			return true
		}
	}
	return false
}

// xbToDivide passes until the damage division is asked (or fails).
func xbToDivide(t *testing.T, sc *scen) Decision {
	t.Helper()
	for i := 0; i < 60; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChDivide {
			return d
		}
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d sub %d before any division", d.Kind, d.Sub)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatal("no division asked")
	return Decision{}
}

// TestExtraBlockSecondBlockOffered: a blocker with ExtraBlocks is offered a
// second attacker after its first, never the same one again, and both blocks
// count as one blocker each.
func TestExtraBlockSecondBlockOffered(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	a1, a2 := sc.perm(xbBear, 0), sc.perm(xbBear, 0)
	tw := sc.perm(xbTwins, 1)
	xbAttack(t, sc, a1, a2)
	if d := sc.s.Step(); !xbOffered(d, tw, a1) || !xbOffered(d, tw, a2) {
		t.Fatalf("first picks not offered: %v", d.Options[:d.N])
	}
	sc.do(t, AkBlock, TCode(tw), TCode(a1))
	d := sc.s.Step()
	if !xbOffered(d, tw, a2) {
		t.Fatalf("the second block is not offered: %v", d.Options[:d.N])
	}
	if xbOffered(d, tw, a1) {
		t.Fatal("the same pair is offered twice")
	}
	sc.do(t, AkBlock, TCode(tw), TCode(a2))
	s, b, x, y := sc.s, sc.slot(tw), sc.slot(a1), sc.slot(a2)
	if !s.blocks(b, x) || !s.blocks(b, y) {
		t.Fatalf("blocks(tw,a1)=%v blocks(tw,a2)=%v", s.blocks(b, x), s.blocks(b, y))
	}
	if s.perms[x].nblock != 1 || s.perms[y].nblock != 1 {
		t.Fatalf("nblock = %d, %d, want 1, 1", s.perms[x].nblock, s.perms[y].nblock)
	}
	if got := s.blockProjected(b); got == NoSlot || s.perms[got].obj != min(a1, a2) {
		t.Fatalf("projected block %v, want the lower instance %d", got, min(a1, a2))
	}
}

// TestExtraBlockNoThirdBlock: two picks fill the capacity; a creature without
// ExtraBlocks gets no second pick.
func TestExtraBlockNoThirdBlock(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	a1, a2, a3 := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 0)
	tw, pl := sc.perm(xbTwins, 1), sc.perm(xbPlain, 1)
	xbAttack(t, sc, a1, a2, a3)
	sc.do(t, AkBlock, TCode(tw), TCode(a1))
	sc.do(t, AkBlock, TCode(pl), TCode(a1))
	d := sc.s.Step()
	if !xbOffered(d, tw, a2) || !xbOffered(d, tw, a3) {
		t.Fatalf("the Twins' second block is not offered: %v", d.Options[:d.N])
	}
	if xbOffered(d, pl, a2) || xbOffered(d, pl, a3) {
		t.Fatal("a plain blocker is offered a second block")
	}
	sc.do(t, AkBlock, TCode(tw), TCode(a2))
	d = sc.s.Step()
	for _, a := range []Obj{a1, a2, a3} {
		if xbOffered(d, tw, a) {
			t.Fatalf("a third Twins block on %d is offered", a)
		}
	}
}

// TestExtraBlockDivisionAskedOfBlocker: a blocker of two attackers has its
// controller divide its damage, every split in ascending order of the first
// attacker's share.
func TestExtraBlockDivisionAskedOfBlocker(t *testing.T) {
	for _, tc := range []struct {
		mode         uint8
		firstDies    bool
		secondDamage int
	}{{3, true, 4}, {7, true, 0}} {
		sc := xbScen(t, Config{NoElide: true})
		a3, a4 := sc.perm(xbA3, 0), sc.perm(xbA4, 0)
		tw := sc.perm(xbTwins, 1)
		if sc.s.perms[sc.slot(a3)].ts >= sc.s.perms[sc.slot(a4)].ts {
			t.Fatal("the 3/3 is not the lower-ts attacker")
		}
		xbAttack(t, sc, a3, a4)
		sc.do(t, AkBlock, TCode(tw), TCode(a4)) // declared in the other order: recipients still go by ts
		sc.do(t, AkBlock, TCode(tw), TCode(a3))
		sc.do(t, AkDone, scAny, scAny)
		d := xbToDivide(t, sc)
		if d.Player != 1 || d.N != 8 || d.Options[0].Source() != TCode(uint8(tw)) {
			t.Fatalf("division: player %d, N %d, source %d; want player 1, N 8, source %d", d.Player, d.N, d.Options[0].Source(), tw)
		}
		sc.s.Apply(MakeAction(AkChoose, TCode(tw), 0, tc.mode))
		sc.passUntil(t, kop.StepMain2)
		switch tc.mode {
		case 3: // 3 to the 3/3, 4 to the 4/4: both die
			if sc.onBF(a3) || sc.onBF(a4) {
				t.Fatalf("mode 3: 3/3 alive=%v 4/4 alive=%v, want both dead", sc.onBF(a3), sc.onBF(a4))
			}
		case 7: // all 7 to the 3/3
			if sc.onBF(a3) || !sc.onBF(a4) || sc.s.perms[sc.slot(a4)].damage != 0 {
				t.Fatalf("mode 7: 3/3 alive=%v 4/4 alive=%v", sc.onBF(a3), sc.onBF(a4))
			}
		}
		if sc.onBF(tw) {
			t.Fatalf("mode %d: the Twins took 7 and lives", tc.mode)
		}
	}
}

// TestExtraBlockFallbackDivision: with the ask off the damage is lethal to the
// first attacker and the rest to the second (gorge bandBlockerAssignments).
func TestExtraBlockFallbackDivision(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true, AutoDivide: true})
	a3, a4 := sc.perm(xbA3, 0), sc.perm(xbA4, 0)
	tw := sc.perm(xbTwins, 1)
	sc.combat(t, []Obj{a3, a4}, [][2]Obj{{tw, a3}, {tw, a4}})
	if sc.onBF(a3) || sc.onBF(a4) {
		t.Fatalf("3/3 alive=%v 4/4 alive=%v, want both dead (3 then 4)", sc.onBF(a3), sc.onBF(a4))
	}
}

// TestExtraBlockAttackerLeavesPromotes: when the first attacker leaves, the
// blocker still blocks the second and deals it everything with no ask.
func TestExtraBlockAttackerLeavesPromotes(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	a1, a2 := sc.perm(xbBear, 0), sc.perm(xbBear, 0)
	tw := sc.perm(xbTwins, 1)
	xbAttack(t, sc, a1, a2)
	sc.do(t, AkBlock, TCode(tw), TCode(a1))
	sc.do(t, AkBlock, TCode(tw), TCode(a2))
	b, s2 := sc.slot(tw), sc.slot(a2)
	if sc.s.perms[b].blocking != sc.slot(a1) || sc.s.xblkOf(b) != s2 {
		t.Fatalf("blocking %d, xblk %d; want the first and second attackers", sc.s.perms[b].blocking, sc.s.xblkOf(b))
	}
	sc.do(t, AkDone, scAny, scAny)
	sc.s.removePerm(sc.k, sc.slot(a1), kop.ZoneGraveyard, false)
	if sc.s.perms[b].blocking != s2 || sc.s.xblkOf(b) != NoSlot || sc.s.perms[b].flags&pfBlocking == 0 {
		t.Fatalf("after the first attacker left: blocking %d, xblk %d, pfBlocking %v", sc.s.perms[b].blocking, sc.s.xblkOf(b), sc.s.perms[b].flags&pfBlocking != 0)
	}
	for i := 0; i < 60 && sc.s.step != kop.StepMain2; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose {
			t.Fatalf("a division is asked for a single-attacker blocker: sub %d", d.Sub)
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if sc.onBF(a2) {
		t.Fatal("the second attacker survives the Twins' 7 damage")
	}
}

// TestExtraBlockLostAbilitiesOneBlock: a Twins that lost its abilities has no
// extra block.
func TestExtraBlockLostAbilitiesOneBlock(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	a1, a2 := sc.perm(xbBear, 0), sc.perm(xbBear, 0)
	tw := sc.perm(xbTwins, 1)
	if sc.s.blockCap(sc.k, sc.slot(tw)) != 2 {
		t.Fatal("the Twins' capacity is not 2 before it loses its abilities")
	}
	sc.s.perms[sc.slot(tw)].ovr.flags |= ovLoseAll
	if sc.s.blockCap(sc.k, sc.slot(tw)) != 1 {
		t.Fatal("a Twins that lost its abilities has capacity above 1")
	}
	xbAttack(t, sc, a1, a2)
	sc.do(t, AkBlock, TCode(tw), TCode(a1))
	if d := sc.s.Step(); xbOffered(d, tw, a2) {
		t.Fatalf("a second block is offered: %v", d.Options[:d.N])
	}
}

// TestExtraBlockMustBlockStalls: a lure attacker with a two-attacker blocker
// ready stalls the declaration by name, and the same board without the Twins
// does not.
func TestExtraBlockMustBlockStalls(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	lure := sc.perm(xbLure, 0)
	sc.perm(xbTwins, 1)
	xbAttack(t, sc, lure)
	if sc.s.StallReason() != StallExtraBlockReq {
		t.Fatalf("stall = %d, want StallExtraBlockReq", sc.s.StallReason())
	}
	sc = xbScen(t, Config{NoElide: true})
	lure = sc.perm(xbLure, 0)
	sc.perm(xbPlain, 1)
	xbAttack(t, sc, lure)
	if sc.s.StallReason() != 0 {
		t.Fatalf("a plain blocker stalls: %d", sc.s.StallReason())
	}
}

// TestExtraBlockBothAttackersDivide: two attackers each blocked by the Twins
// and one other creature each divide their own damage between them, and the
// Twins is dealt each attacker's own share (not one overwriting the other).
func TestExtraBlockBothAttackersDivide(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	a3, a4 := sc.perm(xbA3, 0), sc.perm(xbA4, 0)
	tw, c1, c2 := sc.perm(xbTwins, 1), sc.perm(xbBear, 1), sc.perm(xbBear, 1)
	xbAttack(t, sc, a3, a4)
	for _, p := range [][2]Obj{{tw, a3}, {c1, a3}, {tw, a4}, {c2, a4}} {
		sc.do(t, AkBlock, TCode(p[0]), TCode(p[1]))
	}
	sc.do(t, AkDone, scAny, scAny)
	// mode i gives i to the first blocker in block order (the Twins, declared
	// first for both attackers) and the rest to the other
	modes := map[Obj]uint8{a3: 1, a4: 3, tw: 0}
	asked := map[Obj]bool{}
	for i := 0; i < 80 && sc.s.step != kop.StepMain2; i++ {
		d := sc.s.Step()
		switch d.Kind {
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case DkChoose:
			if d.Sub != ChDivide {
				t.Fatalf("choose sub %d", d.Sub)
			}
			src := Obj(d.Options[0].Source())
			m, ok := modes[src]
			if !ok || asked[src] {
				t.Fatalf("unexpected division of %d", src)
			}
			asked[src] = true
			sc.s.Apply(MakeAction(AkChoose, TCode(src), 0, m))
		default:
			t.Fatalf("decision kind %d", d.Kind)
		}
	}
	if len(asked) != 3 {
		t.Fatalf("divisions asked: %v, want the two attackers and the Twins", asked)
	}
	if got := sc.s.perms[sc.slot(tw)].damage; got != 4 {
		t.Fatalf("the Twins has %d damage, want 1 (from the 3/3) + 3 (from the 4/4) = 4", got)
	}
	if sc.onBF(c1) || !sc.onBF(c2) || sc.s.perms[sc.slot(c2)].damage != 1 {
		t.Fatalf("c1 alive=%v c2 alive=%v damage %d; want c1 dead (2), c2 hit for 1", sc.onBF(c1), sc.onBF(c2), sc.s.perms[sc.slot(c2)].damage)
	}
}
