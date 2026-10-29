package mapping_test

// sbp-unmapped-choose-vote: a fixed-list Vote (Forge Choices$ on SP$ Vote /
// DB$ Vote) routes to the v2 choose_option vocabulary as choose/vote instead
// of unmapped:choose/vote. Selection is by Option.Index; the engine reads the
// chosen Option's Obj (index+1), never its Label (an internal SVar name).

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

func voteEnv(t *testing.T) *mapping.Env {
	t.Helper()
	reg := testcorpus.Registry(t)
	return envFor(t, testgame.New(t, reg, "Wildfire", "Wildfire", 1, "none"))
}

// checkVoteCandidates asserts the v2 projection of a fixed ballot: one
// choose_option per ballot entry, the vocabulary's vote purpose, selection by
// option_index, and the option's Kind (never its Label) as option_label.
func checkVoteCandidates(t *testing.T, p *mapping.Pose, d *decision.Decision) {
	t.Helper()
	if len(p.Candidates) != len(d.Options) {
		t.Fatalf("%d candidates, want %d", len(p.Candidates), len(d.Options))
	}
	for i, c := range p.Candidates {
		if c.Sem.Kind != "choose_option" {
			t.Fatalf("candidate %d is %s, want choose_option", i, c.Sem.Kind)
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("candidate %d fails the v2 validator: %v", i, err)
		}
		if got := c.Sem.Fields["purpose"]; got != "vote" {
			t.Fatalf("candidate %d purpose %v, want vote", i, got)
		}
		if got := c.Sem.Fields["option_index"]; got != uint32(i) {
			t.Fatalf("candidate %d option_index %v, want %d", i, got, i)
		}
		if got := c.Sem.Fields["option_count"]; got != uint32(len(d.Options)) {
			t.Fatalf("candidate %d option_count %v, want %d", i, got, len(d.Options))
		}
		label, ok := c.Sem.Fields["option_label"].(*string)
		if !ok || label == nil || *label != d.Options[i].Kind {
			t.Fatalf("candidate %d label %v, want the option kind %q (not its presentation label %q)",
				i, c.Sem.Fields["option_label"], d.Options[i].Kind, d.Options[i].Label)
		}
	}
}

func TestChooseVoteRoutesFixedBallotOptions(t *testing.T) {
	env := voteEnv(t)
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "vote", Label: "DBPlaneswalk", Obj: 1},
			{Index: 1, Kind: "vote", Label: "DBChaos", Obj: 2},
		}}
	// Precondition: the ballot entries differ in the two fields the route
	// must not confuse -- Obj is a distinct index, Label is an SVar name.
	for i, o := range d.Options {
		if o.Label == o.Kind {
			t.Fatalf("precondition: option %d's label equals its kind", i)
		}
		if o.Obj == 0 {
			t.Fatalf("precondition: option %d has no Obj", i)
		}
	}
	if d.Options[0].Obj == d.Options[1].Obj {
		t.Fatal("precondition: the ballot options share an Obj")
	}
	if got := mapping.Route(d); got != "choose/vote" {
		t.Fatalf("Route = %q, want choose/vote", got)
	}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	checkVoteCandidates(t, p, d)
	commit, done, err := tx.Answer(1)
	if err != nil || !done {
		t.Fatalf("answer: done %v, err %v", done, err)
	}
	if want := []decision.Intent{mapping.Intent(d, 1)}; !reflect.DeepEqual(commit, want) {
		t.Fatalf("commit %v, want %v", commit, want)
	}
	chosen := d.Chosen(commit[0])
	if len(chosen) != 1 || chosen[0].Obj != 2 {
		t.Fatalf("chosen %v, want the option carrying Obj 2", chosen)
	}
}

func TestChooseVoteAnswersTheEngineBallot(t *testing.T) {
	reg := testcorpus.Registry(t)
	deck := catalog.Deck{CatalogID: "choose-vote", Rows: []wire.DeckRow{
		{Name: "Path of the Enigma", Count: 8},
		{Name: "Island", Count: 26}, {Name: "Forest", Count: 26}}}
	cs, err := catalog.Resolve(reg, deck)
	if err != nil {
		t.Fatal(err)
	}
	g := repoGame(t, reg, cs, 1)
	if !testgame.RunUntil(t, g, testgame.Bots(1), func(e *rules.Engine) bool {
		d := e.Pending()
		return d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "vote"
	}, 30000) {
		t.Fatal("no game reached the fixed-list vote ballot at seed 1")
	}
	d := g.E.Pending()
	if len(d.Options) != 2 || d.Options[0].Obj != 1 || d.Options[1].Obj != 2 {
		t.Fatalf("precondition: ballot %v, want two options with Obj 1 and 2", d.Options)
	}
	for i, o := range d.Options {
		if o.Kind != "vote" || o.Label == o.Kind {
			t.Fatalf("precondition: option %d is kind %q label %q, want a vote with an SVar label", i, o.Kind, o.Label)
		}
	}
	if got := mapping.Route(d); got != "choose/vote" {
		t.Fatalf("Route = %q, want choose/vote", got)
	}
	env := envFor(t, g)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	checkVoteCandidates(t, p, d)
	in := mapping.Intent(d, 1)
	commit, done, err := tx.Answer(1)
	if err != nil || !done {
		t.Fatalf("answer: done %v, err %v", done, err)
	}
	chosen := d.Chosen(commit[0])
	if len(chosen) != 1 || chosen[0].Obj != 2 {
		t.Fatalf("chosen %v, want the option carrying Obj 2", chosen)
	}
	if _, err := g.Probe(in); err != nil {
		t.Fatalf("engine rejected the ballot answer: %v", err)
	}
}

// TestRouteKeepsTheMixedVoteFallback pins the !k["vote"] guard: a vote
// beside another kind must not reach chooseObjectRoute, whose Obj test would
// read the vote's ballot index+1 as an object id. twopiles no longer serves
// as the second kind because sbp-unmapped-dig-division gives it its own
// earlier route, so this uses an unclaimed kind with distinct Obj values.
func TestRouteKeepsTheMixedVoteFallback(t *testing.T) {
	d := &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "vote", Label: "DBPlaneswalk", Obj: 1},
			{Index: 1, Kind: "card", Label: "some card", Obj: 2},
		}}
	// Precondition: chooseObjectRoute would call this a select_object ask if
	// the guard were dropped -- every option has a distinct non-zero Obj.
	for i, o := range d.Options {
		if o.Obj == 0 {
			t.Fatalf("precondition: option %d has no Obj", i)
		}
	}
	if d.Options[0].Obj == d.Options[1].Obj {
		t.Fatal("precondition: the mixed options share an Obj")
	}
	if got, want := mapping.Route(d), "unmapped:choose/card,vote"; got != want {
		t.Fatalf("Route = %q, want %q", got, want)
	}
}
