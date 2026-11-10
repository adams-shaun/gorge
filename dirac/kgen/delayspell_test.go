package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDelayspellRefusals: a Mode$ SpellCast DelayedTrigger lowers only as
// the ThisTurn$ True, ValidActivatingPlayer$ You, non-Static$ registration
// with a characteristic-only filter (or a source-power bound) and a body
// naming no SVar, outside a granted ability; the latent keeps its SpellCast
// trigger.
// Flamehold Grappler (ValidCard$ Card) is accepted now that lowers a
// permanent-spell copy (C&2).
func TestDelayspellRefusals(t *testing.T) {
	checkCarrier(t,
		[]string{"Teach by Example", "Twinferno", "Najal, the Storm Runner", "Mercurial Spelldancer",
			"Doublecast", "Summon: G.F. Cerberus", "Pigment Wrangler", "Rimefire Torque",
			"Flamehold Grappler", "Loki Laufeyson", "Mistrise Village", "Theorist's Proxy"},
		map[string]string{
			"Narset Transcendent":   "delayed SpellCast without ThisTurn$",
			"Way of the Cryomancer": "delayed SpellCast in a granted ability",
			"Storm King's Thunder":  `delayed SpellCast body reads "X"`,
			"Sea Gate Stormcaller":  `delayed SpellCast body reads "X"`,
		},
		[]string{
			"DelayedTrigger C=1 CS=NextSpellCast",
			"Latent DelayedSpellCast",
			"trigger SpellCast F={types=0x20 | types=0x40} F2=nil whose=1 zone=0",
			"cmp(2 2 0 X )",                   // Loki: mana value <= the source's power
			"CopySpell obj=Triggered A=2 C=1", // Cerberus III
			"CopySpell obj=Triggered A=1 C=3", // Flamehold Grappler: MayChooseTarget + a permanent copy
		})
}

// TestDelayspellRefusesOtherCaster: ValidActivatingPlayer$ other than You
// is refused (no corpus carrier: a synthetic copy of Teach by Example).
func TestDelayspellRefusesOtherCaster(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Teach by Example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	c := *p.byName["Teach by Example"]
	f := *c.Faces[0]
	sa := *f.Abilities[0]
	sa.Params = map[string]string{}
	for k, v := range f.Abilities[0].Params {
		sa.Params[k] = v
	}
	sa.Params["ValidActivatingPlayer"] = "Opponent"
	f.Abilities = append(f.Abilities[:0:0], &sa)
	c.Faces = append(c.Faces[:0:0], &f)
	lc := lowerCard(p, "Teach by Example", &c, false)
	var got []string
	for _, r := range lc.Refusals {
		got = append(got, r.Reason)
	}
	if all := strings.Join(got, " | "); !strings.Contains(all, `delayed SpellCast ValidActivatingPlayer "Opponent"`) {
		t.Fatalf("refusals %q, want the ValidActivatingPlayer refusal", all)
	}
}
