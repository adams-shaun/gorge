package mapping_test

// The hidden own-library
// priority ref must mint one look id and one Known entry per object even when
// several options name it, and choose/object over hidden library cards must
// stay validator-clean.

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
)

// checkValidatorClean runs the Section 11.3 stream validator over a posed
// decision and the observation that carries its Known entries.
func checkValidatorClean(t *testing.T, env *mapping.Env, p *mapping.Pose) {
	t.Helper()
	obs, err := env.Obs.Observation(p.Seat, observe.State{Known: p.Known})
	if err != nil {
		t.Fatalf("observation: %v", err)
	}
	sd := protocol.SeatDecision{ActingSeat: observe.Seat(p.Seat), SeatStep: 0,
		Group:       protocol.Group{GroupID: 0, SubstepIndex: p.SubstepIndex, SubstepCount: p.SubstepCount},
		Context:     p.Context,
		Observation: obs,
		Extensions:  map[string]json.RawMessage{}}
	for i, c := range p.Candidates {
		sd.Candidates = append(sd.Candidates, protocol.Candidate{CandidateID: uint32(i), Semantic: c.Sem})
	}
	if err := validate.NewStream(validatorProfile()).Check(sd); err != nil {
		t.Fatalf("the posed decision is not validator-clean: %v", err)
	}
}

// hiddenLibraryCard returns designator's own hidden library card and asserts
// the preconditions the priority look path depends on.
func hiddenLibraryCard(t *testing.T, env *mapping.Env, seat state.PlayerID, at int) state.ObjID {
	t.Helper()
	lib := env.G.E.G.Zone(state.ZLibrary, seat)
	if len(lib) <= at {
		t.Fatalf("precondition: seat %d's library holds %d cards, want at least %d", seat, len(lib), at+1)
	}
	id := lib[at]
	obj := env.G.E.G.Obj(id)
	if obj == nil || obj.Zone != state.ZLibrary || obj.Owner != seat {
		t.Fatalf("precondition: library entry %d is not seat %d's library card", at, seat)
	}
	if observe.Visible(seat, obj) {
		t.Fatalf("precondition: library card %d is visible to seat %d", id, seat)
	}
	return id
}

// TestHiddenPriorityKnownEntriesDedupe: two priority options naming the same
// hidden library card (a cast and its flashback variant) must share one look
// ref and one Known entry. A Known entry per ref call puts the same object id
// in the observation twice, which V4 rejects.
func TestHiddenPriorityKnownEntriesDedupe(t *testing.T) {
	env, d0, _, _ := synthPosition(t)
	card := hiddenLibraryCard(t, env, d0.Player, 0)
	d := synthetic(d0,
		decision.Option{Kind: "cast", Obj: card, Label: "Cast"},
		decision.Option{Kind: "cast", Obj: card, Mode: "flashback", Label: "Cast with flashback"})
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("hidden cast is unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("hidden cast pose: %v", err)
	}
	if !p.Look {
		t.Fatal("pose does not open the look the hidden card needs")
	}
	if len(p.Known) != 1 {
		t.Fatalf("known entries %d, want one for the shared hidden card: %+v", len(p.Known), p.Known)
	}
	knownID := p.Known[0].ObjectID
	if knownID == nil {
		t.Fatalf("known entry has no object id: %+v", p.Known[0])
	}
	if p.Known[0].How != "revealed" || p.Known[0].Zone != "library" || p.Known[0].PositionFromTop == nil || *p.Known[0].PositionFromTop != 0 {
		t.Fatalf("known entry %+v is not the revealed top library card", p.Known[0])
	}
	var casts int
	for _, c := range p.Candidates {
		if c.Sem.Kind != "cast_spell" {
			continue
		}
		casts++
		if !c.Hidden || c.SortID != *knownID || c.SortName == "" {
			t.Fatalf("cast candidate %s is not the hidden look card %s", semJSON(c.Sem), *knownID)
		}
	}
	if casts != 2 {
		t.Fatalf("cast_spell candidates %d, want two (normal and flashback): %s", casts, semJSON(p.Candidates))
	}
	checkValidatorClean(t, env, p)
}

// TestHiddenPriorityPlayLandKnownEntry is the single-option case: one
// play_land on the top of the owner's library mints exactly one Known entry,
// a hidden candidate, and a validator-clean pose.
func TestHiddenPriorityPlayLandKnownEntry(t *testing.T) {
	env, d0, _, _ := synthPosition(t)
	card := hiddenLibraryCard(t, env, d0.Player, 0)
	d := synthetic(d0, decision.Option{Kind: "play_land", Obj: card, Label: "Play land"})
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("hidden play_land is unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("hidden play_land pose: %v", err)
	}
	if !p.Look {
		t.Fatal("pose does not open the look the hidden card needs")
	}
	if len(p.Known) != 1 || p.Known[0].ObjectID == nil {
		t.Fatalf("known entries %+v, want one with an object id", p.Known)
	}
	var lands int
	for _, c := range p.Candidates {
		if c.Sem.Kind != "play_land" {
			continue
		}
		lands++
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("play_land candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		if !c.Hidden || c.SortID != *p.Known[0].ObjectID {
			t.Fatalf("play_land candidate %s is not the hidden known card", semJSON(c.Sem))
		}
	}
	if lands != 1 {
		t.Fatalf("play_land candidates %d, want one: %s", lands, semJSON(p.Candidates))
	}
	checkValidatorClean(t, env, p)
}

// TestChooseObjectHiddenLibraryMaps covers the choose/object library class:
// hidden options in a library (the chooser's own or the opponent's) are cards
// a resolving look effect offered, so they map through mixedSelect with look
// ids and Known entries. A hidden hand card mixed in keeps the fail-closed
// not-visible refusal, since an unrevealed hand is not this class.
func TestChooseObjectHiddenLibraryMaps(t *testing.T) {
	env, d0, _, _ := synthPosition(t)
	seat := d0.Player
	own := hiddenLibraryCard(t, env, seat, 0)
	other := hiddenLibraryCard(t, env, 1-seat, 0)
	d := &decision.Decision{Player: seat, Kind: decision.KChoose, Seq: d0.Seq, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "card", Label: "own library", Obj: own},
			{Index: 1, Kind: "card", Label: "opponent library", Obj: other},
		}}
	if got := mapping.Route(d); got != "choose/object" {
		t.Fatalf("Route = %q, want choose/object", got)
	}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("hidden library chooses are unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("hidden library choose pose: %v", err)
	}
	if !p.Look {
		t.Fatal("pose does not open the look the hidden options need")
	}
	if len(p.Known) != 2 {
		t.Fatalf("known entries %d, want one per hidden library card: %+v", len(p.Known), p.Known)
	}
	known := map[string]bool{}
	for _, k := range p.Known {
		if k.ObjectID == nil || k.How != "revealed" || k.Zone != "library" || k.PositionFromTop == nil || *k.PositionFromTop != 0 {
			t.Fatalf("known entry %+v is not a revealed top library card", k)
		}
		known[*k.ObjectID] = true
	}
	var selects int
	for _, c := range p.Candidates {
		if c.Sem.Kind != "select_object" {
			continue
		}
		selects++
		if c.Sem.Fields["purpose"] != "other" {
			t.Fatalf("select_object purpose %v, want other", c.Sem.Fields["purpose"])
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("select_object candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		if !c.Hidden || !known[c.SortID] {
			t.Fatalf("select_object candidate %s is not a hidden known library card", semJSON(c.Sem))
		}
	}
	if selects != 2 {
		t.Fatalf("select_object candidates %d, want two: %s", selects, semJSON(p.Candidates))
	}
	checkValidatorClean(t, env, p)

	// The same ask with a hidden hand option is not the library class: the
	// hand card may not have been revealed, so the whole ask stays refused.
	hand := env.G.E.G.Zone(state.ZHand, 1-seat)
	if len(hand) == 0 {
		t.Fatal("precondition: the other seat has no hidden hand card")
	}
	if observe.Visible(seat, env.G.E.G.Obj(hand[0])) {
		t.Fatalf("precondition: hand card %d is visible to seat %d", hand[0], seat)
	}
	mixed := &decision.Decision{Player: seat, Kind: decision.KChoose, Seq: d0.Seq, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "card", Label: "library", Obj: own},
			{Index: 1, Kind: "card", Label: "hidden hand", Obj: hand[0]},
		}}
	if _, err := mapping.Begin(env, mixed); !errors.Is(err, mapping.ErrUnmapped) {
		t.Fatalf("mixed library+hand ask: err = %v, want ErrUnmapped", err)
	}
}
