package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSVarCardCountersFromSourcePermanent pins the Count$CardCounters.<KIND>
// grammar arm: the count is the source permanent's own counters, never a sum
// over every permanent, and anything the observation cannot place as a
// battlefield permanent is unknown rather than a confident zero.
func TestSVarCardCountersFromSourcePermanent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := NewSession(reg)

	src := svarRecord("o-shrine", "Shrine of Burning Rage", "p0", "battlefield")
	src.Permanent = &protocol.Permanent{Counters: map[string]uint32{"charge": 3}}
	other := svarRecord("o-bears", "Grizzly Bears", "p0", "battlefield")
	other.Permanent = &protocol.Permanent{Counters: map[string]uint32{"charge": 1}}
	o := &protocol.Observation{
		Viewer: "p0",
		Players: [2]protocol.PlayerObs{{
			Seat:        "p0",
			Battlefield: []protocol.ObjectRecord{src, other},
		}},
	}
	// The preconditions the assertions depend on: the source is one of two
	// battlefield permanents, and the other one would change a global sum.
	if got := len(o.Players[0].Battlefield); got != 2 {
		t.Fatalf("setup: battlefield has %d permanents, want 2", got)
	}
	found := false
	for _, r := range o.Players[0].Battlefield {
		if r.ObjectID != src.ObjectID {
			continue
		}
		found = true
		if r.Permanent == nil || r.Permanent.Counters["charge"] != 3 {
			t.Fatalf("setup: source permanent counters = %v, want charge 3", r.Permanent)
		}
	}
	if !found {
		t.Fatal("setup: the source is not on the battlefield")
	}
	if got := other.Permanent.Counters["charge"]; got != 1 {
		t.Fatalf("setup: the control permanent has charge %d, want 1", got)
	}

	cases := []struct {
		body string
		want int32
		ok   bool
	}{
		{"Count$CardCounters.CHARGE", 3, true},     // source-only, not the 4 a global sum gives
		{"Count$CardCounters.ALL", 3, true},        // every kind on the source only
		{"Count$CardCounters.P1P1", 0, true},       // known permanent, absent kind
		{"Count$CardCounters.ALL/Mod.2", 0, false}, // unsupported operator stays unknown
	}
	for _, tc := range cases {
		if got, ok := s.svarValue(o, &src.ObjectRef, tc.body); got != tc.want || ok != tc.ok {
			t.Errorf("%s = %d, %v; want %d, %v", tc.body, got, ok, tc.want, tc.ok)
		}
	}

	// A record the observation carries with no permanent data cannot count.
	noPerm := svarRecord("o-shrine", "Shrine of Burning Rage", "p0", "battlefield")
	oNoPerm := &protocol.Observation{Players: [2]protocol.PlayerObs{{
		Battlefield: []protocol.ObjectRecord{noPerm},
	}}}
	if n, ok := s.svarValue(oNoPerm, &noPerm.ObjectRef, "Count$CardCounters.CHARGE"); ok {
		t.Errorf("a permanent with no counters payload = %d, true; want unknown", n)
	}

	// A source the observation does not place on a battlefield is unknown.
	absent := svarRecord("o-absent", "Shrine of Burning Rage", "p0", "battlefield")
	if n, ok := s.svarValue(o, &absent.ObjectRef, "Count$CardCounters.CHARGE"); ok {
		t.Errorf("an absent source = %d, true; want unknown", n)
	}

	// The control: the new arm must not swallow the existing zone heads.
	if n, ok := s.svarValue(o, nil, "Count$Valid Creature"); !ok || n != 1 {
		t.Errorf("Count$Valid Creature = %d, %v; want 1, true", n, ok)
	}
}

// TestTargetEffectCardCountersAmount wires the grammar arm into the ask the
// target decision publishes: Shrine of Burning Rage's NumDmg$ X is its charge
// counters, and the neutral DealDamage must carry that amount for the bot's
// value-kill ranking.
func TestTargetEffectCardCountersAmount(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := NewSession(reg)
	name := "Shrine of Burning Rage"
	f := s.B.face(&name, false, nil)
	if f == nil {
		t.Fatalf("setup: no face for %s", name)
	}
	if got := f.SVars["X"]; got != "Count$CardCounters.CHARGE" {
		t.Fatalf("setup: X = %q, want Count$CardCounters.CHARGE", got)
	}
	src := svarRecord("o-shrine", name, "p1", "battlefield")
	src.Permanent = &protocol.Permanent{Counters: map[string]uint32{"charge": 3}}
	obs := func(battlefield []protocol.ObjectRecord) *protocol.Observation {
		return &protocol.Observation{
			Viewer: "p0",
			Players: [2]protocol.PlayerObs{
				{Seat: "p0"},
				{Seat: "p1", Battlefield: battlefield},
			},
		}
	}
	s.activated[src.ObjectID] = 0

	te := s.targetEffect(obs([]protocol.ObjectRecord{src}), src.ObjectRef, 0)
	if te == nil {
		t.Fatal("targetEffect returned nil")
	}
	if te.API != "DealDamage" {
		t.Fatalf("API = %q, want DealDamage", te.API)
	}
	if te.Damage == nil || te.Damage.Amount == nil {
		t.Fatal("Damage.Amount is nil; the charge counters were not read")
	}
	if got := *te.Damage.Amount; got != 3 {
		t.Errorf("Damage.Amount = %d, want 3", got)
	}

	// A known permanent with no charge counter is a known zero.
	src.Permanent.Counters = map[string]uint32{}
	te = s.targetEffect(obs([]protocol.ObjectRecord{src}), src.ObjectRef, 0)
	if te == nil || te.Damage == nil || te.Damage.Amount == nil {
		t.Fatal("known permanent without the counter: Damage.Amount is nil")
	}
	if got := *te.Damage.Amount; got != 0 {
		t.Errorf("known permanent without the counter: Damage.Amount = %d, want 0", got)
	}

	// A source not on the battlefield is unknown, not zero.
	te = s.targetEffect(obs(nil), src.ObjectRef, 0)
	if te == nil || te.Damage == nil {
		t.Fatal("absent source: no Damage effect")
	}
	if te.Damage.Amount != nil {
		t.Errorf("absent source: Damage.Amount = %d, want nil (unknown)", *te.Damage.Amount)
	}
}
