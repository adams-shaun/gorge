package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectSourceSynthetic lower a synthetic creature whose only ability is
// root, with svars supplied for its StaticAbilities$/Triggers$ bodies, and
// returns the refusal reasons.
func effectSourceSynthetic(t *testing.T, root string, svars map[string]string) []string {
	t.Helper()
	r := cards.ResolveSVar(map[string]string{"R": root}, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	face := &cards.Face{
		Name: "Synthetic", ManaCost: "1 G", Types: []string{"Creature"}, PT: "2/2",
		SVars: svars, Abilities: []*cards.SA{r},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, rf := range lowerCard(p, "Synthetic", card, false).Refusals {
		out = append(out, rf.Reason)
	}
	return out
}

// TestEffectSourceHostBound: an activated Effect with no Duration$ whose
// CantBlockBy reads ValidAttacker$ Card.EffectSource is the host-bound
// emblem. Giant Koi and Verdant Outrider carry that shape (gorge's
// EffectSource is an alias of the registered effect's source object,
// effects/filter.go; a card that leaves and returns is a new object,
// CR 400.7, so the this-turn and host-leaves boundaries coincide). The
// shapes the widening must not admit stay refused.
func TestEffectSourceHostBound(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Giant Koi\nVerdant Outrider\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, name := range []string{"Giant Koi", "Verdant Outrider"} {
		c := byName[name]
		if c == nil {
			t.Fatalf("%s: not lowered", name)
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", name, c.Refusals)
			continue
		}
		if len(c.Emblems) != 1 {
			t.Errorf("%s: %d emblems, want 1", name, len(c.Emblems))
			continue
		}
		ke := c.Emblems[0]
		if !ke.WhileHost || !ke.UntilEOT || ke.UntilNextTurn {
			t.Errorf("%s emblem = %+v, want WhileHost one-turn", name, ke)
		}
		if len(ke.Statics) != 1 || ke.Statics[0].Kind != "CantBlockBy" {
			t.Errorf("%s statics = %+v, want one CantBlockBy", name, ke.Statics)
		}
	}

	// A Duration$ Permanent Effect is not this-turn: the host-bound arm must
	// not take it (its lifetime is not the source's).
	if got := effectSourceSynthetic(t, "AB$ Effect | Cost$ 1 G | Duration$ Permanent | StaticAbilities$ KW",
		map[string]string{"KW": "Mode$ CantBlockBy | ValidAttacker$ Card.EffectSource"}); len(got) == 0 {
		t.Errorf("Duration$ Permanent Effect lowered, want a refusal")
	}
	// Only the attacker side of CantBlockBy reads the source; a blocker-side
	// EffectSource has no host-bound reading.
	got := effectSourceSynthetic(t, "AB$ Effect | Cost$ 1 G | StaticAbilities$ KW",
		map[string]string{"KW": "Mode$ CantBlockBy | ValidAttacker$ Card.EffectSource | ValidBlocker$ Card.EffectSource"})
	if len(got) == 0 || !strings.Contains(strings.Join(got, "; "), "ValidBlocker") {
		t.Errorf("ValidBlocker$ Card.EffectSource refusals = %q, want a ValidBlocker refusal", got)
	}
	// An Effect TRIGGER reads EffectSource only as a DamageDone source (the
	// remembered-source emblem hook, TestEffectSourceTrigger); any other key
	// or mode still has no host-bound reading.
	for _, body := range []string{
		"Mode$ DamageDone | ValidSource$ Player | ValidTarget$ Card.EffectSource | Execute$ TrigDraw",
		"Mode$ ChangesZone | ValidCard$ Card.EffectSource | Execute$ TrigDraw",
	} {
		if got := effectSourceSynthetic(t, "AB$ Effect | Cost$ 3 U U | Triggers$ DamageTrig",
			map[string]string{"DamageTrig": body, "TrigDraw": "DB$ Draw | NumCards$ 2"}); len(got) == 0 {
			t.Errorf("Effect trigger %q lowered, want a refusal", body)
		}
	}
}
