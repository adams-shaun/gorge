package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func svarRecord(id, name, seat, zone string) protocol.ObjectRecord {
	return protocol.ObjectRecord{ObjectRef: protocol.ObjectRef{
		ObjectID: id, CardName: &name, OwnerSeat: seat, ControllerSeat: seat, Zone: zone,
	}}
}

// TestSVarCountScopeIsUnknownNotBothPlayers pins the MAJOR review finding:
// a scoped Count$Valid body must not be counted over every player's zones.
// The observation here would make the naive all-players scan answer 4 for
// Creature.YouCtrl and 3 for Creature.YouOwn (the opponent's zones change the
// number), so the only honest answer is "unknown", never 4/3 with ok=true.
func TestSVarCountScopeIsUnknownNotBothPlayers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := &Session{B: NewBuilder(reg)}
	o := &protocol.Observation{
		Viewer: "p0",
		Players: [2]protocol.PlayerObs{
			{
				Seat:        "p0",
				Battlefield: []protocol.ObjectRecord{svarRecord("o-mine", "Grizzly Bears", "p0", "battlefield")},
				Graveyard:   []protocol.ObjectRecord{svarRecord("o-g0", "Grizzly Bears", "p0", "graveyard")},
			},
			{
				Seat: "p1",
				Battlefield: []protocol.ObjectRecord{
					svarRecord("o-a", "Grizzly Bears", "p1", "battlefield"),
					svarRecord("o-b", "Grizzly Bears", "p1", "battlefield"),
					svarRecord("o-c", "Grizzly Bears", "p1", "battlefield"),
				},
				Graveyard: []protocol.ObjectRecord{
					svarRecord("o-g1", "Grizzly Bears", "p1", "graveyard"),
					svarRecord("o-g2", "Grizzly Bears", "p1", "graveyard"),
				},
			},
		},
	}
	// The precondition the assertion depends on: both players' zones are
	// present, and the opponent's would change the uncorrected counts.
	if got := len(o.Players[0].Battlefield) + len(o.Players[1].Battlefield); got != 4 {
		t.Fatalf("setup: battlefield has %d creatures, want 4", got)
	}
	if got := len(o.Players[0].Graveyard) + len(o.Players[1].Graveyard); got != 3 {
		t.Fatalf("setup: graveyards have %d creatures, want 3", got)
	}
	for _, body := range []string{
		"Count$Valid Creature.YouCtrl",
		"Count$ValidGraveyard Creature.YouOwn",
		"Count$Valid Creature.OppCtrl",
	} {
		if n, ok := s.svarValue(o, body); ok {
			t.Errorf("%s = %d, true; a source-scoped count must stay unknown, not count both players", body, n)
		}
	}
}

// TestSVarCountUnknownPredicateIsUnknown pins the second MAJOR defect: a
// predicate the evaluator cannot read must not fall through to a type-line
// test and report a confident zero. Creature.attacking and friends are real
// corpus bodies; none can be decided from a printed face alone.
func TestSVarCountUnknownPredicateIsUnknown(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := &Session{B: NewBuilder(reg)}
	o := &protocol.Observation{
		Viewer: "p0",
		Players: [2]protocol.PlayerObs{{
			Seat:        "p0",
			Battlefield: []protocol.ObjectRecord{svarRecord("o-mine", "Grizzly Bears", "p0", "battlefield")},
		}},
	}
	if len(o.Players[0].Battlefield) != 1 {
		t.Fatal("setup: the count would be a legitimate zero, hiding a fail-open")
	}
	for _, body := range []string{
		"Count$Valid Creature.attacking",
		"Count$Valid Creature.powerGE4",
		"Count$Valid Card.IsCommander",
		"Count$Valid Creature.withDefender",
	} {
		if n, ok := s.svarValue(o, body); ok {
			t.Errorf("%s = %d, true; an unreadable predicate must be unknown", body, n)
		}
	}
}

// TestSVarCountKeepsNamedAcrossAllGraveyards is the control: Feast of Flesh's
// body has no scope word, so it still counts both graveyards and applies its
// Plus.1 suffix. Without this the target-effect fix the ticket shipped would
// regress.
func TestSVarCountKeepsNamedAcrossAllGraveyards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := &Session{B: NewBuilder(reg)}
	o := &protocol.Observation{
		Viewer: "p0",
		Players: [2]protocol.PlayerObs{
			{
				Seat:      "p0",
				Graveyard: []protocol.ObjectRecord{svarRecord("o-f0", "Feast of Flesh", "p0", "graveyard")},
			},
			{
				Seat:      "p1",
				Graveyard: []protocol.ObjectRecord{svarRecord("o-f1", "Feast of Flesh", "p1", "graveyard")},
			},
		},
	}
	if got, ok := s.svarValue(o, "Count$ValidGraveyard Card.namedFeast of Flesh/Plus.1"); !ok || got != 3 {
		t.Fatalf("Feast of Flesh count = %d, %v; want 3, true (two named cards plus Plus.1)", got, ok)
	}
}
