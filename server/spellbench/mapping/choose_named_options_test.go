package mapping_test

// Named-option choose asks route to the v2
// choose_option / select_object vocabulary instead of unmapped:choose/....

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

func namedOptionEnv(t *testing.T) *mapping.Env {
	t.Helper()
	reg := testcorpus.Registry(t)
	g := testgame.New(t, reg, "Wildfire", "Wildfire", 1, "none")
	return envFor(t, g)
}

func TestChooseOptionRoutesNamedOptions(t *testing.T) {
	env := namedOptionEnv(t)
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "primary", Label: "top"},
			{Index: 1, Kind: "bottom", Label: "bottom"},
		}}
	// Precondition: the labels differ from the kinds the wire must carry.
	if d.Options[0].Label == d.Options[0].Kind {
		t.Fatal("precondition: option 0's label equals its kind")
	}
	if got := mapping.Route(d); got != "choose/option" {
		t.Fatalf("Route = %q, want choose/option", got)
	}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Candidates) != 2 {
		t.Fatalf("%d candidates, want two", len(p.Candidates))
	}
	for i, c := range p.Candidates {
		if c.Sem.Kind != "choose_option" {
			t.Fatalf("candidate %d is %s, want choose_option", i, c.Sem.Kind)
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("candidate %d fails the v2 validator: %v", i, err)
		}
		if got := c.Sem.Fields["purpose"]; got != "effect_option" {
			t.Fatalf("candidate %d purpose %v, want effect_option", i, got)
		}
		if got := c.Sem.Fields["option_index"]; got != uint32(i) {
			t.Fatalf("candidate %d option_index %v", i, got)
		}
		if got := c.Sem.Fields["option_count"]; got != uint32(2) {
			t.Fatalf("candidate %d option_count %v", i, got)
		}
		label, ok := c.Sem.Fields["option_label"].(*string)
		if !ok || label == nil || *label != d.Options[i].Kind {
			t.Fatalf("candidate %d label %v, want the option kind %q (not its presentation label %q)",
				i, c.Sem.Fields["option_label"], d.Options[i].Kind, d.Options[i].Label)
		}
	}
	commit, done, err := tx.Answer(1)
	if err != nil || !done {
		t.Fatalf("answer: done %v, err %v", done, err)
	}
	if want := []decision.Intent{mapping.Intent(d, 1)}; !reflect.DeepEqual(commit, want) {
		t.Fatalf("commit %v, want %v", commit, want)
	}

	// Two options naming the same object cannot be select_object candidates
	// (their semantics would be identical), so they take the option route:
	// riot, opening-hand yes/no and cumulative upkeep all pose this shape.
	same := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "riot", Label: "counter", Obj: 7},
			{Index: 1, Kind: "riot", Label: "haste", Obj: 7},
		}}
	if got := mapping.Route(same); got != "choose/option" {
		t.Fatalf("same-object ask Route = %q, want choose/option", got)
	}
}

func TestChooseObjectRoutesObjectOptions(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, e *rules.Engine) bool {
		return d.Kind == decision.KPriority && len(e.G.Zone(state.ZBattlefield, d.Player)) >= 2
	})
	env := envFor(t, g)
	seat := g.E.Pending().Player
	ids := g.E.G.Zone(state.ZBattlefield, seat)
	if len(ids) < 2 {
		t.Fatalf("precondition: seat %d has %d battlefield objects", seat, len(ids))
	}
	visible := ids[:2]
	for _, id := range visible {
		if !observe.Visible(seat, g.E.G.Obj(id)) {
			t.Fatalf("precondition: battlefield object %d is not visible", id)
		}
	}
	if visible[0] == visible[1] {
		t.Fatalf("precondition: the two battlefield ids are equal (%d)", visible[0])
	}
	objectAsk := &decision.Decision{Player: seat, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "card", Label: "first", Obj: visible[0]},
			{Index: 1, Kind: "card", Label: "second", Obj: visible[1]},
		}}
	if got := mapping.Route(objectAsk); got != "choose/object" {
		t.Fatalf("object ask Route = %q, want choose/object", got)
	}
	tx, err := mapping.Begin(env, objectAsk)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Candidates) != 2 {
		t.Fatalf("%d object candidates, want two", len(p.Candidates))
	}
	for i, c := range p.Candidates {
		if c.Sem.Kind != "select_object" {
			t.Fatalf("candidate %d is %s, want select_object", i, c.Sem.Kind)
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("candidate %d fails the v2 validator: %v", i, err)
		}
		if got := c.Sem.Fields["purpose"]; got != "other" {
			t.Fatalf("candidate %d purpose %v, want other", i, got)
		}
		if lo, hi := c.Sem.Fields["minimum"], c.Sem.Fields["maximum"]; lo != uint32(1) || hi != uint32(1) {
			t.Fatalf("candidate %d bounds %v..%v, want 1..1", i, lo, hi)
		}
		sel, ok := c.Sem.Fields["choice"].(protocol.TargetRef)
		if !ok || sel.Object == nil {
			t.Fatalf("candidate %d has no object choice: %v", i, c.Sem.Fields["choice"])
		}
		want, err := env.Obs.Ref(seat, visible[i])
		if err != nil || want == nil {
			t.Fatalf("precondition: ref for %d: %v, %v", visible[i], want, err)
		}
		if sel.Object.ObjectID != want.ObjectID {
			t.Fatalf("candidate %d names object %q, want %q", i, sel.Object.ObjectID, want.ObjectID)
		}
	}

	// A player-only option set is a select_object ask over player refs.
	playerAsk := &decision.Decision{Player: seat, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "player", Player: 0},
			{Index: 1, Kind: "player", Player: 1},
		}}
	if got := mapping.Route(playerAsk); got != "choose/object" {
		t.Fatalf("player ask Route = %q, want choose/object", got)
	}
	ptx, err := mapping.Begin(env, playerAsk)
	if err != nil {
		t.Fatal(err)
	}
	pp, err := ptx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if len(pp.Candidates) != 2 {
		t.Fatalf("%d player candidates, want two", len(pp.Candidates))
	}
	for i, c := range pp.Candidates {
		sel, ok := c.Sem.Fields["choice"].(protocol.TargetRef)
		if c.Sem.Kind != "select_object" || !ok || sel.Player == nil {
			t.Fatalf("candidate %d is not a player select_object: %s %v", i, c.Sem.Kind, c.Sem.Fields["choice"])
		}
		if want := observe.Seat(state.PlayerID(i)); *sel.Player != want {
			t.Fatalf("candidate %d names player %q, want %q", i, *sel.Player, want)
		}
	}

	// A hidden object must not be projected: the ask keeps the not-visible
	// ErrUnmapped and never opens a look.
	other := 1 - seat
	hand := g.E.G.Zone(state.ZHand, other)
	if len(hand) == 0 {
		t.Fatal("precondition: the other seat has no hidden hand card")
	}
	if observe.Visible(seat, g.E.G.Obj(hand[0])) {
		t.Fatalf("precondition: hand card %d is visible to seat %d", hand[0], seat)
	}
	hiddenAsk := &decision.Decision{Player: seat, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "card", Obj: visible[0]},
			{Index: 1, Kind: "card", Obj: hand[0]},
		}}
	if got := mapping.Route(hiddenAsk); got != "choose/object" {
		t.Fatalf("hidden ask Route = %q, want choose/object", got)
	}
	_, err = mapping.Begin(env, hiddenAsk)
	if !errors.Is(err, mapping.ErrUnmapped) {
		t.Fatalf("hidden object: err = %v, want ErrUnmapped", err)
	}
	if !strings.Contains(err.Error(), "not visible") {
		t.Fatalf("hidden object: err = %v, want a not-visible error", err)
	}
}

// TestChooseOptionMultiPick covers a named-option ask that needs more than
// one answer (ChangeText's from+to pair): each pick is one choose_option
// substep, and the last pick commits both native options as one intent.
func TestChooseOptionMultiPick(t *testing.T) {
	env := namedOptionEnv(t)
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 2, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "changetext_from", Label: "black"},
			{Index: 1, Kind: "changetext_to", Label: "white"},
		}}
	if got := mapping.Route(d); got != "choose/option" {
		t.Fatalf("Route = %q, want choose/option", got)
	}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if !p.GroupStart || p.SubstepIndex != 0 || p.SubstepCount != 2 || len(p.Candidates) != 2 {
		t.Fatalf("first substep: start %v index %d of %d, %d candidates",
			p.GroupStart, p.SubstepIndex, p.SubstepCount, len(p.Candidates))
	}
	commit, done, err := tx.Answer(0)
	if err != nil || done || len(commit) != 0 {
		t.Fatalf("first pick: commit %v, done %v, err %v", commit, done, err)
	}
	p, err = tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if p.GroupStart || p.SubstepIndex != 1 || p.SubstepCount != 2 || len(p.Candidates) != 1 {
		t.Fatalf("second substep: start %v index %d of %d, %d candidates",
			p.GroupStart, p.SubstepIndex, p.SubstepCount, len(p.Candidates))
	}
	commit, done, err = tx.Answer(0)
	if err != nil || !done {
		t.Fatalf("second pick: done %v, err %v", done, err)
	}
	if want := []decision.Intent{mapping.Intent(d, 0, 1)}; !reflect.DeepEqual(commit, want) {
		t.Fatalf("commit %v, want %v", commit, want)
	}
}
