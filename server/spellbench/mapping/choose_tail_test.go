package mapping_test

// sbp-unmapped-choose-tail: the remaining named `choose` asks that the
// one-pick catch-alls cannot express. Each subtest drives a real game to one
// live decision of the kind, maps it, and requires every candidate to pass
// the protocol shape and the Section 11.3 stream validator. The old code
// refuses proliferate, pair and convoke before it can pose anything.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
)

type tailCard struct {
	name  string
	count int
}

// tailDeck builds a 40-card deck of the named cards, padded with basic land.
func tailDeck(t *testing.T, reg *cards.Registry, land string, entries ...tailCard) []*cards.Card {
	t.Helper()
	out := make([]*cards.Card, 0, 40)
	for _, e := range entries {
		c := lookupCard(t, reg, e.name)
		for i := 0; i < e.count; i++ {
			out = append(out, c)
		}
	}
	fill := lookupCard(t, reg, land)
	for len(out) < 40 {
		out = append(out, fill)
	}
	if len(out) != 40 {
		t.Fatalf("deck has %d cards, want 40", len(out))
	}
	return out
}

// tailUntil plays bots over the same deck until a decision satisfies pred.
func tailUntil(t *testing.T, reg *cards.Registry, deck []*cards.Card, secretByte byte, pred func(*decision.Decision) bool) *gamecfg.Game {
	t.Helper()
	for s := int(secretByte); s < int(secretByte)+40; s++ {
		buf := make([]byte, 32)
		buf[0] = byte(s)
		g, err := gamecfg.New(reg, secrets.NewGame(buf), [2][]*cards.Card{deck, deck},
			gamecfg.Rules{Mulligan: "none"})
		if err != nil {
			t.Fatal(err)
		}
		if testgame.RunUntil(t, g, testgame.Bots(uint64(s)), func(e *rules.Engine) bool {
			d := e.Pending()
			return d != nil && pred(d)
		}, 30000) {
			return g
		}
	}
	t.Fatal("no game reached the wanted decision")
	return nil
}

// checkTailPose maps d, requires every candidate to pass the protocol shape
// and the stream validator, and requires the first candidate to commit an
// answer Decision.Validate accepts.
func checkTailPose(t *testing.T, g *gamecfg.Game, d *decision.Decision, wantKind string) *mapping.Pose {
	t.Helper()
	env := envFor(t, g)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("the %s ask is unmapped: %v", wantKind, err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("%s pose: %v", wantKind, err)
	}
	if len(p.Candidates) == 0 {
		t.Fatalf("%s pose has no candidates", wantKind)
	}
	seen := false
	for _, c := range p.Candidates {
		if c.Sem.Kind == wantKind {
			seen = true
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("%s candidate %s fails the protocol shape: %v", wantKind, semJSON(c.Sem), err)
		}
	}
	if !seen {
		t.Fatalf("%s has no %s candidate: %s", wantKind, wantKind, semJSON(p.Candidates))
	}
	checkValidatorClean(t, env, p)
	commit := answerAll(t, tx, func(*mapping.Pose) int { return 0 })
	if len(commit) != 1 {
		t.Fatalf("%s committed %d intents, want one: %v", wantKind, len(commit), commit)
	}
	if err := d.Validate(commit[0]); err != nil {
		t.Fatalf("%s first candidate commits %v, which Validate rejects: %v", wantKind, commit[0].Choices, err)
	}
	return p
}

// envRef resolves obj as the environment's observation names it.
func envRef(t *testing.T, g *gamecfg.Game, seat state.PlayerID, obj state.ObjID) (protocol.ObjectRef, error) {
	t.Helper()
	env := envFor(t, g)
	r, err := env.Obs.Ref(seat, obj)
	if err != nil || r == nil {
		return protocol.ObjectRef{}, fmt.Errorf("object %d has no visible ref", obj)
	}
	return *r, nil
}

func TestChooseTailKindsMap(t *testing.T) {
	reg := testcorpus.Registry(t)

	t.Run("card", func(t *testing.T) {
		deck := tailDeck(t, reg, "Swamp", tailCard{"Phyrexian Dreadnought", 8})
		g := tailUntil(t, reg, deck, 1, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && d.Min == 0 && d.ResumeKind == "choice" &&
				len(d.Options) > 0 && d.Options[0].Kind == "card"
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/card" {
			t.Fatalf("Route = %q, want choose/card", got)
		}
		checkTailPose(t, g, d, "select_object")
	})

	t.Run("pair", func(t *testing.T) {
		deck := tailDeck(t, reg, "Mountain", tailCard{"Hanweir Lancer", 16})
		g := tailUntil(t, reg, deck, 3, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "pair"
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/pair" {
			t.Fatalf("Route = %q, want choose/pair", got)
		}
		checkTailPose(t, g, d, "select_object")
	})

	t.Run("reveal", func(t *testing.T) {
		deck := tailDeck(t, reg, "Forest",
			tailCard{"Scent of Ivy", 8}, tailCard{"Grizzly Bears", 8})
		g := tailUntil(t, reg, deck, 5, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && d.Min == 0 && len(d.Options) > 0 && d.Options[0].Kind == "reveal"
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/reveal" {
			t.Fatalf("Route = %q, want choose/reveal", got)
		}
		checkTailPose(t, g, d, "select_object")
	})

	t.Run("move_counter", func(t *testing.T) {
		deck := tailDeck(t, reg, "Forest",
			tailCard{"Bioshift", 8}, tailCard{"Servant of the Scale", 8},
			tailCard{"Island", 8})
		g := tailUntil(t, reg, deck, 7, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "move_counter"
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/number" {
			t.Fatalf("Route = %q, want choose/number", got)
		}
		checkTailPose(t, g, d, "choose_number")
	})

	t.Run("enlist", func(t *testing.T) {
		deck := tailDeck(t, reg, "Plains", tailCard{"Argivian Cavalier", 16})
		g := tailUntil(t, reg, deck, 9, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "enlist"
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/enlist" {
			t.Fatalf("Route = %q, want choose/enlist", got)
		}
		checkTailPose(t, g, d, "select_object")
	})

	t.Run("proliferate", func(t *testing.T) {
		deck := tailDeck(t, reg, "Forest",
			tailCard{"Contentious Plan", 8}, tailCard{"Servant of the Scale", 8},
			tailCard{"Island", 8})
		g := tailUntil(t, reg, deck, 11, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "proliferate"
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/proliferate" {
			t.Fatalf("Route = %q, want choose/proliferate", got)
		}
		checkTailPose(t, g, d, "select_object")
	})

	t.Run("convoke", func(t *testing.T) {
		deck := tailDeck(t, reg, "Plains",
			tailCard{"Venerated Loxodon", 4}, tailCard{"Savannah Lions", 16})
		g := tailUntil(t, reg, deck, 13, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && strings.HasPrefix(d.Options[0].Kind, "convoke_")
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/payment" {
			t.Fatalf("Route = %q, want choose/payment", got)
		}
		// Precondition: at least one creature carries both a coloured and a
		// generic contribution, the shape a per-option candidate list would
		// duplicate. The candidate is per object, so the committed option must
		// name that same object.
		perObject := map[state.ObjID]int{}
		for _, o := range d.Options {
			perObject[o.Obj]++
		}
		both := false
		for _, n := range perObject {
			both = both || n > 1
		}
		if !both {
			t.Fatalf("precondition: the convoke ask offers no object with both contributions: %+v", d.Options)
		}
		p := checkTailPose(t, g, d, "select_object")
		for _, c := range p.Candidates {
			if c.Sem.Kind != "select_object" {
				continue
			}
			tgt, _ := c.Sem.Fields["choice"].(protocol.TargetRef)
			if tgt.Object == nil || c.Op.Option < 0 || c.Op.Option >= len(d.Options) ||
				d.Options[c.Op.Option].Obj == 0 {
				t.Fatalf("convoke candidate %s commits option %d", semJSON(c.Sem), c.Op.Option)
			}
			if r, err := envRef(t, g, d.Player, d.Options[c.Op.Option].Obj); err != nil || r.ObjectID != tgt.Object.ObjectID {
				t.Fatalf("convoke candidate names %s but commits option %d (%s)", semJSON(c.Sem), c.Op.Option, d.Options[c.Op.Option].Label)
			}
		}
	})

	t.Run("activation_creature", func(t *testing.T) {
		deck := tailDeck(t, reg, "Plains",
			tailCard{"Heirloom Epic", 8}, tailCard{"Ornithopter", 16})
		g := tailUntil(t, reg, deck, 15, func(d *decision.Decision) bool {
			return d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "activation_creature"
		})
		d := g.E.Pending()
		if got := mapping.Route(d); got != "choose/payment" {
			t.Fatalf("Route = %q, want choose/payment", got)
		}
		checkTailPose(t, g, d, "select_object")
	})
}
