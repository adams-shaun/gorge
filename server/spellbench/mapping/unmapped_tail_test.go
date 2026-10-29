package mapping_test

// Tests for the v2 mapping gaps in the sbp-unmapped-tail-priority ticket:
// granted/gained abilities, a hand-to-exile choice, the evoke method, and the
// station special action. Each asserts the protocol shape (Sem.Check) and the
// native option the candidate commits.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestGrantedAbilityMapsToIndexedActivation covers an "ability" option whose
// body the printed face does not carry (an SVar grant from another permanent,
// and a gained ability). v2 has only {source, ability_index}; the mapping
// places the printed non-mana abilities first and the grants after them in
// engine offer order, and each candidate must pass the protocol validator.
func TestGrantedAbilityMapsToIndexedActivation(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, e *rules.Engine) bool {
		return d.Kind == decision.KPriority && len(e.G.Zone(state.ZBattlefield, d.Player)) >= 2
	})
	env := envFor(t, g)
	d0 := g.E.Pending()
	bf := g.E.G.Zone(state.ZBattlefield, d0.Player)
	recipient, grantor := bf[0], bf[1]
	if recipient == grantor {
		t.Fatal("need two distinct battlefield permanents")
	}
	if z := g.E.G.Obj(grantor).Zone; z != state.ZBattlefield {
		t.Fatalf("grant source %d is in %s, want the battlefield", grantor, z)
	}

	// Precondition: the recipient is on the battlefield and its printed face
	// has a measurable non-mana ability base; the two grants then take the
	// next two indices.
	base, err := mapping.GrantedAbilityIndex(g.E.G.Obj(recipient), 0)
	if err != nil {
		t.Fatalf("GrantedAbilityIndex: %v", err)
	}
	next, err := mapping.GrantedAbilityIndex(g.E.G.Obj(recipient), 1)
	if err != nil || next != base+1 {
		t.Fatalf("second grant index = %d, %v; want %d", next, err, base+1)
	}

	d := synthetic(d0,
		decision.Option{Kind: "ability", Obj: recipient, Label: "Granted: pump",
			SVar: "GrantedPump", GrantSource: grantor},
		decision.Option{Kind: "ability", Obj: recipient, Label: "Gained: draw",
			GainedSource: grantor, GainedIdx: 0})
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("granted ability is unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("granted ability pose: %v", err)
	}
	var got []uint32
	first := -1
	for i, c := range p.Candidates {
		if c.Sem.Kind != "activate_ability" {
			continue
		}
		if first < 0 {
			first = i
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("activate_ability candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		idx, ok := c.Sem.Fields["ability_index"].(uint32)
		if !ok {
			t.Fatalf("ability_index %v is not a u32", c.Sem.Fields["ability_index"])
		}
		got = append(got, idx)
	}
	if want := []uint32{base, base + 1}; !slices.Equal(got, want) {
		t.Fatalf("granted ability indices %v, want %v", got, want)
	}
	// The first granted candidate commits its native option (the first
	// synthetic option; pass is native option 2 but Finalize moves its
	// candidate to the front).
	if first < 0 {
		t.Fatalf("no activate_ability candidate: %s", semJSON(p.Candidates))
	}
	commit, done, err := tx.Answer(first)
	if err != nil || !done || len(commit) != 1 || len(commit[0].Choices) != 1 || commit[0].Choices[0] != 0 {
		t.Fatalf("grant candidate commit %v, done %v, err %v", commit, done, err)
	}
}

// TestHandMoveExileMapsToSelect covers a hand_move ask whose destination is
// Exile when the cards are the chooser's own: the select_object purpose is the
// vocabulary's "exile", and each candidate passes the validator.
func TestHandMoveExileMapsToSelect(t *testing.T) {
	env, d0, hand, _ := synthPosition(t)
	if len(hand) < 2 {
		t.Fatalf("need two hand cards, got %d", len(hand))
	}
	d := &decision.Decision{Kind: decision.KChoose, Seq: d0.Seq, Player: d0.Player, Min: 1, Max: 1,
		ResumeSA: &cards.SA{Kind: "AB", Params: map[string]string{"Destination": "Exile"}}}
	for i, id := range hand[:2] {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "hand_move", Obj: id, Label: "Exile a card"})
	}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("hand_move/Exile is unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("hand_move/Exile pose: %v", err)
	}
	n := 0
	for _, c := range p.Candidates {
		if c.Sem.Kind != "select_object" {
			continue
		}
		if c.Sem.Fields["purpose"] != "exile" {
			t.Fatalf("select_object purpose %v, want exile", c.Sem.Fields["purpose"])
		}
		choice, ok := c.Sem.Fields["choice"].(protocol.TargetRef)
		if !ok || choice.Object == nil || choice.Object.Zone != "hand" {
			t.Fatalf("select_object choice %s", semJSON(c.Sem))
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("select_object candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		n++
	}
	if n != len(d.Options) {
		t.Fatalf("%d select_object candidates for %d hand cards", n, len(d.Options))
	}
	commit, done, err := tx.Answer(0)
	if err != nil || !done || len(commit) != 1 || commit[0].Choices[0] != 0 {
		t.Fatalf("exile candidate commit %v, done %v, err %v", commit, done, err)
	}
}

// TestEvokeCastMethodMaps pins the evoke row of the Section 7.4 method table
// and a posed evoke cast. The protocol vocabulary carries "evoke" (so the
// documented refusal does not apply); gorge emits it as cast_spell.method,
// which is the same place flashback and the other cast modes ride.
func TestEvokeCastMethodMaps(t *testing.T) {
	if !slices.Contains(protocol.Vocab["method"], "evoke") {
		t.Fatal("protocol method vocabulary has no evoke word")
	}
	m, optional, special, ok := mapping.CastMethod(decision.Option{Kind: "cast", Mode: "evoked"}, "")
	if !ok || m != "evoke" || optional != "" || special != "" {
		t.Fatalf("evoked = %q %q %q %v, want method evoke", m, optional, special, ok)
	}

	env, d0, hand, _ := synthPosition(t)
	d := synthetic(d0, decision.Option{Kind: "cast", Obj: hand[0], Label: "Cast evoked", Mode: "evoked"})
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("evoked cast is unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("evoked cast pose: %v", err)
	}
	found := false
	for _, c := range p.Candidates {
		if c.Sem.Kind != "cast_spell" || c.Sem.Fields["method"] != "evoke" {
			continue
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("evoke candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		found = true
	}
	if !found {
		t.Fatalf("no evoke cast_spell candidate: %s", semJSON(p.Candidates))
	}
}

// TestStationMapsToSpecialAction covers kw:Station's priority option. v2 has
// no station kind or ability anchor for it, so it rides the closed
// special_action vocabulary's catch-all "other".
func TestStationMapsToSpecialAction(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, e *rules.Engine) bool {
		return d.Kind == decision.KPriority && len(e.G.Zone(state.ZBattlefield, d.Player)) >= 1
	})
	env := envFor(t, g)
	d0 := g.E.Pending()
	obj := g.E.G.Zone(state.ZBattlefield, d0.Player)[0]
	d := synthetic(d0, decision.Option{Kind: "station", Obj: obj, Label: "Station"})
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("station is unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("station pose: %v", err)
	}
	for _, c := range p.Candidates {
		if c.Sem.Kind != "special_action" {
			continue
		}
		if c.Sem.Fields["action"] != "other" {
			t.Fatalf("station action %v, want other", c.Sem.Fields["action"])
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("station candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		return
	}
	t.Fatalf("no special_action candidate for station: %s", semJSON(p.Candidates))
}
