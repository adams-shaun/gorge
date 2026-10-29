package mapping_test

import (
	"errors"
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

// repeatableCharmGame builds a 60-card deck of four copies of cardName and 56
// basic lands in both seats and plays until that card's modal ask.
func repeatableCharmGame(t *testing.T, cardName, basicLand string, secretByte byte) *gamecfg.Game {
	t.Helper()
	reg := testcorpus.Registry(t)
	card, ok := reg.Lookup(cardName)
	if !ok {
		t.Fatalf("no card named %q", cardName)
	}
	land, ok := reg.Lookup(basicLand)
	if !ok {
		t.Fatalf("no basic land named %q", basicLand)
	}
	deck := make([]*cards.Card, 0, 60)
	for i := 0; i < 4; i++ {
		deck = append(deck, card)
	}
	for i := 0; i < 56; i++ {
		deck = append(deck, land)
	}
	s := make([]byte, 32)
	s[0] = secretByte
	g, err := gamecfg.New(reg, secrets.NewGame(s), [2][]*cards.Card{deck, deck},
		gamecfg.Rules{Mulligan: "none", StartingSeat: state.PlayerID(0)})
	if err != nil {
		t.Fatal(err)
	}
	if !testgame.RunUntil(t, g, testgame.Bots(uint64(secretByte)), func(e *rules.Engine) bool {
		d := e.Pending()
		if d == nil || d.Kind != decision.KModes {
			return false
		}
		o := e.G.Obj(d.Source)
		return o != nil && o.Face().Name == cardName
	}, 30000) {
		t.Fatalf("no %s game reached its modal ask", cardName)
	}
	return g
}

// Season of Loss is CharmNum$ 5 over three printed modes: the ask wants five
// picks and v2's choose_spell_mode caps maximum at mode_count, so the mapping
// must decline it rather than emit candidates the validator rejects.
func TestRepeatableModesBeyondPrintedCountAreUnmapped(t *testing.T) {
	t.Run("declines beyond", func(t *testing.T) {
		g := repeatableCharmGame(t, "Season of Loss", "Swamp", 41)
		d := g.E.Pending()
		if d.Kind != decision.KModes || !d.Repeatable {
			t.Fatalf("ask %+v, want a repeatable modal decision", d)
		}
		if d.Min != 0 || d.Max != 5 || len(d.Options) != 3 {
			t.Fatalf("ask min %d max %d options %d, want 0/5/3", d.Min, d.Max, len(d.Options))
		}
		if d.ResumeSA == nil {
			t.Fatal("ask carries no ResumeSA")
		}
		if got := len(strings.Split(d.ResumeSA.Params["Choices"], ",")); got != 3 {
			t.Fatalf("printed modes %d, want 3 (%q)", got, d.ResumeSA.Params["Choices"])
		}
		env := envFor(t, g)
		src, err := mapping.MustSource(env, d)
		if err != nil {
			t.Fatal(err)
		}
		if src.Zone != "stack" {
			t.Fatalf("source zone %q, want stack", src.Zone)
		}
		// Pin the protocol side: the 5-over-3 pair the ask would forward is
		// exactly what choose_spell_mode's constraint rejects.
		if err := protocol.ChooseSpellMode(src, 0, 3, 0, 0, 5).Check(); err == nil ||
			!strings.Contains(err.Error(), "maximum 5") {
			t.Fatalf("ChooseSpellMode(0 of 3, max 5).Check() = %v, want a maximum 5 rejection", err)
		}
		_, err = mapping.Begin(env, d)
		if !errors.Is(err, mapping.ErrUnmapped) {
			t.Fatalf("Begin = %v, want ErrUnmapped", err)
		}
		if !strings.Contains(err.Error(), "printed=3") || !strings.Contains(err.Error(), "max=5") {
			t.Fatalf("Begin error %q, want printed=3 and max=5", err)
		}
	})

	// The over-broadening control: a repeatable ask whose picks fit the printed
	// count must keep mapping and pose valid candidates.
	t.Run("maps within", func(t *testing.T) {
		g := repeatableCharmGame(t, "Fiery Confluence", "Mountain", 42)
		d := g.E.Pending()
		if !d.Repeatable || d.Min != 3 || d.Max != 3 || len(d.Options) != 2 {
			t.Fatalf("ask min %d max %d options %d repeatable %t, want 3/3/2/true",
				d.Min, d.Max, len(d.Options), d.Repeatable)
		}
		env := envFor(t, g)
		tx, err := mapping.Begin(env, d)
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		p, err := tx.Pose()
		if err != nil {
			t.Fatalf("Pose: %v", err)
		}
		modes := 0
		for _, c := range p.Candidates {
			if err := c.Sem.Check(); err != nil {
				t.Fatalf("candidate %+v: %v", c.Sem.Fields, err)
			}
			if c.Sem.Kind != "choose_spell_mode" {
				continue
			}
			modes++
			if c.Sem.Fields["mode_count"] != uint32(3) || c.Sem.Fields["maximum"] != uint32(3) {
				t.Fatalf("candidate %+v, want mode_count 3 and maximum 3", c.Sem.Fields)
			}
		}
		if modes == 0 {
			t.Fatal("no choose_spell_mode candidate posed")
		}
	})
}
