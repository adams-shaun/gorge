package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// replCountersScen builds a generated-table scenario over deck (both players) with
// the card ids by name, like the oracle's exileanddoublers pool.
func replCountersScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "exileanddoublers")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	land := id["Forest"]
	var lib [20]CardID
	for i := range lib {
		lib[i] = land
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// TestFilteredDoublerKind: a filtered doubler doubles only the counter kind its
// ValidCounterType named. Branching Evolution (+1/+1) must leave a lifelink
// counter alone.
func TestFilteredDoublerKind(t *testing.T) {
	sc, id := replCountersScen(t, []cardset.Entry{
		{Name: "Branching Evolution", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Forest", Count: 1},
	})
	bears := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Branching Evolution"], 0)
	sl := sc.slot(bears)
	if got := sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 1); got != 2 {
		t.Fatalf("P1P1 replCounters = %d, want 2", got)
	}
	if got := sc.s.replCounters(sc.k, sl, int(kop.CtrLifelink), 1); got != 1 {
		t.Fatalf("lifelink replCounters = %d, want 1 (Branching Evolution is P1P1 only)", got)
	}
}

// TestFilteredPlusKind: the same filter for a plus-one replacement. Hardened
// Scales (+1/+1) must not add a counter of another kind.
func TestFilteredPlusKind(t *testing.T) {
	sc, id := replCountersScen(t, []cardset.Entry{
		{Name: "Hardened Scales", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Forest", Count: 1},
	})
	bears := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Hardened Scales"], 0)
	sl := sc.slot(bears)
	if got := sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 1); got != 2 {
		t.Fatalf("P1P1 replCounters = %d, want 2", got)
	}
	if got := sc.s.replCounters(sc.k, sl, int(kop.CtrLifelink), 1); got != 1 {
		t.Fatalf("lifelink replCounters = %d, want 1 (Hardened Scales is P1P1 only)", got)
	}
}

// TestTwoExileOnLeaveOrder: two matching dies-to-exile sources agree on
// Exile, but gorge still asks the affected player to order the replacements
// (CR 616.1). The kernel has no death order ask, so it stalls instead of
// applying an order the reference poses a decision for. Applying Exile
// deterministically instead makes the exileanddoublers lockstep diverge (matchup 0
// seed 36, decision 326: gorge still has the blocker on the battlefield at
// the ask, the kernel has already exiled it).
func TestTwoExileOnLeaveOrder(t *testing.T) {
	sc, id := replCountersScen(t, []cardset.Entry{
		{Name: "Misery's Shadow", Count: 1},
		{Name: "Stone of Erech", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Forest", Count: 1},
	})
	bears := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Misery's Shadow"], 1)
	sc.perm(id["Stone of Erech"], 1)
	if z := sc.s.dieZone(sc.k, sc.slot(bears)); z != kop.ZoneExile {
		t.Fatalf("dieZone = %d, want the stall's Exile (the matching replacement's zone)", z)
	}
	if !sc.s.Stalled() || sc.s.StallReason() != StallReplLeave {
		t.Fatalf("stall = %v reason %d, want StallReplLeave", sc.s.Stalled(), sc.s.StallReason())
	}
}

// TestSourceDyingBesideParkedDeath: a creature with two dies-to-exile sources
// trades with them in one state-based pass. Both apply (CR 704.3), so its death
// is asked about after they have gone, offering them, and the answer exiles it.
func TestSourceDyingBesideParkedDeath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		other string
	}{
		{"one source left", "Stone of Erech"},
		{"no source left", "Misery's Shadow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, id := replCountersScen(t, []cardset.Entry{
				{Name: "Misery's Shadow", Count: 2},
				{Name: "Stone of Erech", Count: 1},
				{Name: "Grizzly Bears", Count: 1},
				{Name: "Forest", Count: 1},
			})
			s, k := sc.s, sc.k
			bears := sc.perm(id["Grizzly Bears"], 0)
			shadow := sc.perm(id["Misery's Shadow"], 1)
			other := sc.perm(id[tc.other], 1)
			for _, o := range []Obj{bears, shadow} {
				s.perms[sc.slot(o)].damage = 9
			}
			if s.objs[other].card == s.objs[shadow].card {
				s.perms[sc.slot(other)].damage = 9
			}
			if !s.sba(k) || s.ctl != ctlExileOrder {
				t.Fatalf("sba posed no order ask, ctl %d", s.ctl)
			}
			if z := s.objs[shadow].zone; z != kop.ZoneGraveyard {
				t.Fatalf("the dying source is in zone %d at the ask, want the graveyard", z)
			}
			if z := s.objs[bears].zone; z != kop.ZoneBattlefield {
				t.Fatalf("the asked creature is in zone %d at the ask, want the battlefield", z)
			}
			var d Decision
			s.exileOrderOptions(k, &d)
			if d.N != 2 || Obj(d.Options[0].Source()) != shadow || Obj(d.Options[1].Source()) != other {
				t.Fatalf("offered %d options %v, want the sources %d and %d", d.N, d.Options[:d.N], shadow, other)
			}
			s.applyExileOrder(k, d.Options[0])
			if z := s.objs[bears].zone; z != kop.ZoneExile {
				t.Fatalf("the answered creature is in zone %d, want exile", z)
			}
			if s.ctl != ctlPriority || s.parked(bears) || s.Stalled() {
				t.Fatalf("after the answer: ctl %d, parked %v, stalled %v", s.ctl, s.parked(bears), s.Stalled())
			}
		})
	}
}

// TestParkedDeathsAskOneAtATime: two creatures with two dies-to-exile sources
// die in one pass: each is asked about once, in battlefield order, and leaves on
// its own answer.
func TestParkedDeathsAskOneAtATime(t *testing.T) {
	sc, id := replCountersScen(t, []cardset.Entry{
		{Name: "Misery's Shadow", Count: 1},
		{Name: "Stone of Erech", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 1},
	})
	s, k := sc.s, sc.k
	first := sc.perm(id["Grizzly Bears"], 0)
	second := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Misery's Shadow"], 1)
	sc.perm(id["Stone of Erech"], 1)
	s.perms[sc.slot(first)].damage = 9
	s.perms[sc.slot(second)].damage = 9
	for _, o := range []Obj{first, second} {
		if !s.sba(k) || s.ctl != ctlExileOrder {
			t.Fatalf("no order ask for object %d, ctl %d", o, s.ctl)
		}
		if s.cold.parked[0].obj != o {
			t.Fatalf("asked about object %d, want %d", s.cold.parked[0].obj, o)
		}
		var d Decision
		s.exileOrderOptions(k, &d)
		s.applyExileOrder(k, d.Options[1])
		if z := s.objs[o].zone; z != kop.ZoneExile {
			t.Fatalf("object %d is in zone %d after its answer, want exile", o, z)
		}
	}
	if s.sba(k) || s.ctl != ctlPriority || s.Stalled() {
		t.Fatalf("after both answers: ctl %d, stalled %v", s.ctl, s.Stalled())
	}
}

// TestCounterKeywordRemoved: a keyword counter removed to zero must stop
// granting its keyword. A seen kind keeps its sparse slot after the last
// counter is removed (ctrFree only frees unseen kinds), so ctrKw has to
// read the count and not just the packed kind.
func TestCounterKeywordRemoved(t *testing.T) {
	cases := []struct {
		kind kop.CounterKind
		kw   kop.KwMask
	}{
		{kop.CtrFlying, kop.KwFlying},
		{kop.CtrFirstStrike, kop.KwFirstStrike},
		{kop.CtrDoubleStrike, kop.KwDoubleStrike},
		{kop.CtrDeathtouch, kop.KwDeathtouch},
		{kop.CtrHaste, kop.KwHaste},
		{kop.CtrHexproof, kop.KwHexproof},
		{kop.CtrIndestructible, kop.KwIndestructible},
		{kop.CtrLifelink, kop.KwLifelink},
		{kop.CtrMenace, kop.KwMenace},
		{kop.CtrReach, kop.KwReach},
		{kop.CtrShadow, kop.KwShadow},
		{kop.CtrTrample, kop.KwTrample},
		{kop.CtrVigilance, kop.KwVigilance},
	}
	s := &State{}
	for _, c := range cases {
		var p perm
		if !p.setCtrN(s, int(c.kind), 1) {
			t.Fatalf("kind %d: no slot for the first counter", c.kind)
		}
		if got := ctrKw(s, &p); got&c.kw == 0 {
			t.Fatalf("kind %d: counter grants no keyword", c.kind)
		}
		p.seeCtr(s)
		if got := ctrKw(s, &p); got&c.kw == 0 {
			t.Fatalf("kind %d: seen counter grants no keyword", c.kind)
		}
		if !p.setCtrN(s, int(c.kind), 0) {
			t.Fatalf("kind %d: could not remove the counter", c.kind)
		}
		if got := ctrKw(s, &p); got&c.kw != 0 {
			t.Fatalf("kind %d: removed counter still grants %b", c.kind, got)
		}
	}
}
