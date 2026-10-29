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
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestGrantedAbilityMapsToIndexedActivation covers an "ability" option whose
// body the printed face does not carry (an SVar grant from another permanent).
// v2 has only {source, ability_index}; the index is the position in the
// observation's ordered ability list -- printed abilities first, then the
// grant -- so the client can name the body the number points at. Each
// candidate must pass the protocol validator and equal that position.
func TestGrantedAbilityMapsToIndexedActivation(t *testing.T) {
	reg := testcorpus.Registry(t)
	prodigal, ok := reg.Lookup("Prodigal Sorcerer")
	if !ok {
		t.Fatal(`precondition: corpus has no "Prodigal Sorcerer"`)
	}
	longbow, ok := reg.Lookup("Viridian Longbow")
	if !ok {
		t.Fatal(`precondition: corpus has no "Viridian Longbow"`)
	}
	island, ok := reg.Lookup("Island")
	if !ok {
		t.Fatal(`precondition: corpus has no "Island"`)
	}
	var deck []*cards.Card
	for range 10 {
		deck = append(deck, prodigal)
	}
	for range 10 {
		deck = append(deck, longbow)
	}
	for range 20 {
		deck = append(deck, island)
	}

	// A real game: both cards on the deciding seat's battlefield, so the later
	// AddContinuous grant is exactly the card's static's shape.
	var sorcerer, grantor state.ObjID
	var seat state.PlayerID
	g := repoGame(t, reg, deck, 1)
	found := testgame.RunUntil(t, g, testgame.Bots(1), func(e *rules.Engine) bool {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			return false
		}
		for _, id := range e.G.Zone(state.ZBattlefield, d.Player) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Prodigal Sorcerer":
				sorcerer = id
			case "Viridian Longbow":
				grantor = id
			}
		}
		if sorcerer != 0 && grantor != 0 {
			seat = d.Player
			return true
		}
		return false
	}, 30000)
	if !found {
		t.Fatal("precondition: no game put Prodigal Sorcerer and Viridian Longbow on one battlefield")
	}
	lbFace := longbow.Faces[0]
	if cards.ResolveSVar(lbFace.SVars, "Damage") == nil {
		t.Fatal(`precondition: Viridian Longbow has no "Damage" SVar body`)
	}
	g.E.AddContinuous(rules.ContinuousEffect{Source: grantor, Controller: seat,
		Affects: "Creature.YouCtrl", AddAbilities: []string{"Damage"}, SVars: lbFace.SVars})
	env := envFor(t, g)
	d0 := g.E.Pending()
	if d0 == nil || d0.Player != seat {
		t.Fatalf("precondition: pending %v for seat %d, want a priority decision for %d", d0, seat, seat)
	}

	// Precondition: the observation publishes exactly the printed body then
	// the grant, with the grant anchored on the Longbow.
	sref, err := env.Obs.Ref(seat, sorcerer)
	if err != nil || sref == nil {
		t.Fatalf("sorcerer ref: %v %v", sref, err)
	}
	lref, err := env.Obs.Ref(seat, grantor)
	if err != nil || lref == nil {
		t.Fatalf("longbow ref: %v %v", lref, err)
	}
	obs, err := env.Obs.Observation(seat, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	var rec *protocol.ObjectRecord
	for pi := range obs.Players {
		for i := range obs.Players[pi].Battlefield {
			if obs.Players[pi].Battlefield[i].ObjectID == sref.ObjectID {
				rec = &obs.Players[pi].Battlefield[i]
			}
		}
	}
	if rec == nil {
		t.Fatalf("precondition: observation has no record for %s", sref.ObjectID)
	}
	if len(rec.Abilities) != 2 {
		t.Fatalf("precondition: observed abilities = %d (%+v), want 2", len(rec.Abilities), rec.Abilities)
	}
	if a := rec.Abilities[0]; a.SVar != nil || a.GrantSource != nil || a.GainedFrom != nil || a.GainedIndex != nil || a.Keyword != nil {
		t.Fatalf("precondition: printed entry has anchors: %+v", a)
	}
	granted := rec.Abilities[1]
	if granted.SVar == nil || *granted.SVar != "Damage" || granted.GrantSource == nil ||
		granted.GrantSource.ObjectID != lref.ObjectID {
		t.Fatalf("precondition: granted entry = %+v, want SVar Damage from %s", granted, lref.ObjectID)
	}

	d := synthetic(d0,
		decision.Option{Kind: "ability", Obj: sorcerer, Label: rec.Abilities[0].Label,
			SVar: "Damage", GrantSource: grantor},
		decision.Option{Kind: "ability", Obj: sorcerer, Ability: 0})
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("granted ability is unmapped: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("granted ability pose: %v", err)
	}
	indices := map[int]uint32{}
	for _, c := range p.Candidates {
		if c.Sem.Kind != "activate_ability" {
			continue
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("activate_ability candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		idx, ok := c.Sem.Fields["ability_index"].(uint32)
		if !ok {
			t.Fatalf("ability_index %v is not a u32", c.Sem.Fields["ability_index"])
		}
		indices[c.Op.Option] = idx
	}
	grantedIdx, ok := indices[0]
	if !ok {
		t.Fatalf("no candidate commits the granted synthetic option (0): %+v", indices)
	}
	printedIdx, ok := indices[1]
	if !ok {
		t.Fatalf("no candidate commits the printed synthetic option (1): %+v", indices)
	}
	if grantedIdx != 1 {
		t.Fatalf("granted ability_index = %d, want the grant's observation position 1", grantedIdx)
	}
	if printedIdx != 0 {
		t.Fatalf("printed ability_index = %d, want the printed entry's position 0", printedIdx)
	}
	// The positions the mapping published are the observation's own list.
	if rec.Abilities[grantedIdx].SVar == nil || *rec.Abilities[grantedIdx].SVar != "Damage" {
		t.Fatalf("granted index does not name the observed grant: %+v", rec.Abilities)
	}
	if rec.Abilities[printedIdx].SVar != nil {
		t.Fatalf("printed index does not name the observed printed entry: %+v", rec.Abilities)
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
