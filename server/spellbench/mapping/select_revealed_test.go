package mapping_test

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
)

// pilferGame builds a deck of four Pilfer and 56 Swamps in both seats and
// plays until Pilfer's "you choose a card from their revealed hand" discard
// ask. Pilfer is `SP$ Discard | Mode$ RevealYouChoose`: the engine poses a
// KModes "discard" ask to the caster over the target's hand
// (engine/effects/cardflow.go: discardAsk), which is the reveal fact — there
// is no separate RevealHand event for this shape.
func pilferGame(t *testing.T, secretByte byte) *gamecfg.Game {
	t.Helper()
	reg := testcorpus.Registry(t)
	pilfer, ok := reg.Lookup("Pilfer")
	if !ok {
		t.Fatal("corpus lacks Pilfer")
	}
	swamp, ok := reg.Lookup("Swamp")
	if !ok {
		t.Fatal("corpus lacks Swamp")
	}
	deck := make([]*cards.Card, 0, 60)
	for i := 0; i < 4; i++ {
		deck = append(deck, pilfer)
	}
	for i := 0; i < 56; i++ {
		deck = append(deck, swamp)
	}
	for s := secretByte; s < secretByte+40; s++ {
		buf := make([]byte, 32)
		buf[0] = s
		g, err := gamecfg.New(reg, secrets.NewGame(buf), [2][]*cards.Card{deck, deck},
			gamecfg.Rules{Mulligan: "none", StartingSeat: state.PlayerID(0)})
		if err != nil {
			t.Fatal(err)
		}
		if testgame.RunUntil(t, g, testgame.Bots(uint64(s)), func(e *rules.Engine) bool {
			d := e.Pending()
			if d == nil || d.Kind != decision.KModes || mapping.Route(d) != "modes/discard" {
				return false
			}
			src := e.G.Obj(d.Source)
			return src != nil && src.Face() != nil && src.Face().Name == "Pilfer"
		}, 60000) {
			return g
		}
	}
	t.Fatal("no Pilfer game reached its discard ask")
	return nil
}

// hiddenOptions is the cards Pilfer reveals to the chooser: the ask's options
// that sit in the other player's hand.
func hiddenOptions(t *testing.T, g *gamecfg.Game, d *decision.Decision) []state.ObjID {
	t.Helper()
	var out []state.ObjID
	for _, o := range d.Options {
		obj := g.E.G.Obj(o.Obj)
		if obj == nil {
			t.Fatalf("option %d has no object", o.Index)
		}
		if !observe.Visible(d.Player, obj) {
			out = append(out, o.Obj)
		}
	}
	if len(out) == 0 {
		t.Fatalf("precondition: Pilfer's ask has no card hidden from the chooser: %+v", d)
	}
	return out
}

func validatorProfile() validate.Profile {
	kinds := map[string]bool{}
	for k := range protocol.KindFields {
		kinds[k] = true
	}
	return validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{}}
}

// TestPilferDiscardFromRevealedHand: Pilfer's chooser sees the opponent's
// revealed hand, so the modes/discard candidates must reference those hand
// cards through the look, carry them as known "revealed" entries, and pass
// the Section 11.3 validator (V4 needs every candidate ref in the
// observation, V5 the order and ownership of the known entries).
func TestPilferDiscardFromRevealedHand(t *testing.T) {
	g := pilferGame(t, 41)
	d := g.E.Pending()
	hidden := hiddenOptions(t, g, d)
	env := envFor(t, g)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("pose: %v", err)
	}
	if !p.Look {
		t.Fatal("pose does not open the look over the revealed hand")
	}
	if len(p.Known) != len(hidden) {
		t.Fatalf("known entries %d, want one per hidden card (%d): %+v", len(p.Known), len(hidden), p.Known)
	}
	named := map[string]bool{}
	for _, k := range p.Known {
		if k.How != "revealed" || k.Zone != "hand" || k.ObjectID == nil {
			t.Fatalf("known entry %+v is not a revealed hand card with an id", k)
		}
		named[k.CardName] = true
	}
	var selects int
	for _, c := range p.Candidates {
		if c.Sem.Kind != "select_object" {
			continue
		}
		selects++
		sel, ok := c.Sem.Fields["choice"].(protocol.TargetRef)
		if !ok || sel.Object == nil || sel.Object.CardName == nil {
			t.Fatalf("candidate names no object: %+v", c.Sem)
		}
		if c.Sem.Fields["purpose"] != "discard" {
			t.Fatalf("candidate purpose %v, want discard", c.Sem.Fields["purpose"])
		}
		if !c.Hidden || c.SortName != *sel.Object.CardName || c.SortID != sel.Object.ObjectID {
			t.Fatalf("candidate %q is not a hidden-look card with its order key", *sel.Object.CardName)
		}
		if !named[*sel.Object.CardName] {
			t.Fatalf("candidate names %q, which is not among the known revealed cards", *sel.Object.CardName)
		}
	}
	if selects != len(hidden) {
		t.Fatalf("select_object candidates %d, want one per revealed card (%d)", selects, len(hidden))
	}
	obs, err := env.Obs.Observation(p.Seat, observe.State{Known: p.Known})
	if err != nil {
		t.Fatal(err)
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

// TestRevealedHandNotVisibleToThirdSeat: the reveal is scoped to the chooser.
// A viewer that was not handed the mapping's known entries — the other seat,
// and the chooser itself observed bare — must not get references to the
// revealed cards, and the look ids the chooser uses must not appear in the
// other seat's observation.
func TestRevealedHandNotVisibleToThirdSeat(t *testing.T) {
	g := pilferGame(t, 42)
	d := g.E.Pending()
	hidden := hiddenOptions(t, g, d)
	env := envFor(t, g)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Known) != len(hidden) {
		t.Fatalf("precondition: known entries %d, want %d", len(p.Known), len(hidden))
	}
	other := state.PlayerID(1) - p.Seat

	// The chooser's bare observation: the opponent's hand is not projected
	// and no reveal is recorded, so the cards are unreferenced.
	bare, err := env.Obs.Observation(p.Seat, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bare.Known) != 0 {
		t.Fatalf("bare observation already knows %+v without the reveal", bare.Known)
	}
	if bare.Players[other].Hand != nil {
		t.Fatalf("bare observation projects %s's hand", observe.Seat(other))
	}
	lookIDs := map[string]bool{}
	for _, k := range p.Known {
		lookIDs[*k.ObjectID] = true
	}
	for _, r := range observationRefs(bare) {
		if lookIDs[r.ObjectID] {
			t.Fatalf("the bare observation references look id %s", r.ObjectID)
		}
	}

	// The other seat's own observation lists its hand under its visible ids;
	// the chooser's look ids must never appear there.
	ownerObs, err := env.Obs.Observation(other, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerObs.Known) != 0 {
		t.Fatalf("the other seat's observation gained known entries: %+v", ownerObs.Known)
	}
	for _, r := range observationRefs(ownerObs) {
		if lookIDs[r.ObjectID] {
			t.Fatalf("the look id %s leaked into %s's observation", r.ObjectID, observe.Seat(other))
		}
	}

	// The same cards are referenced only when the reveal is handed over, and
	// then only as revealed hand entries of the chooser's decision.
	shown, err := env.Obs.Observation(p.Seat, observe.State{Known: p.Known})
	if err != nil {
		t.Fatal(err)
	}
	if len(shown.Known) != len(hidden) {
		t.Fatalf("shown observation has %d known entries, want %d", len(shown.Known), len(hidden))
	}
	for _, k := range shown.Known {
		if !lookIDs[*k.ObjectID] || k.How != "revealed" {
			t.Fatalf("shown known entry %+v is not a revealed look card", k)
		}
	}
}

// observationRefs is every object reference an observation carries: zone
// records, stack entries and known cards with an id.
func observationRefs(o protocol.Observation) []protocol.ObjectRef {
	var out []protocol.ObjectRef
	for _, p := range o.Players {
		for _, zone := range [][]protocol.ObjectRecord{p.Hand, p.Battlefield, p.Graveyard, p.Exile, p.Command} {
			for _, r := range zone {
				out = append(out, r.ObjectRef)
			}
		}
	}
	for _, st := range o.Stack {
		out = append(out, st.ObjectRef)
	}
	for _, k := range o.Known {
		if k.ObjectID != nil {
			out = append(out, protocol.ObjectRef{ObjectID: *k.ObjectID, OwnerSeat: k.OwnerSeat, Zone: k.Zone,
				CardName: &k.CardName})
		}
	}
	return out
}
