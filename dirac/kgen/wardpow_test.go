package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kwardpowBuild lowers and builds one card from its script text, returning
// its CardDef and refusals.
func kwardpowBuild(t *testing.T, name, src string) (kop.CardDef, []refusal) {
	t.Helper()
	card, diags := cards.ParseBytes("kwardpow_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	return bs[0].def, lw.cards[0].Refusals
}

// TestWardLifePower: Raubahn's Ward—Pay life equal to its power lowers to
// WardLifePower (no {N}, no fixed life); any other X in a PayLife<X/…> ward
// cost refuses.
func TestWardLifePower(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(tableCorpus(t), "cardsfolder", "r", "raubahn_bull_of_ala_mhigo.txt"))
	if err != nil {
		t.Skip("no gorge corpus: ", err)
	}
	// the attack trigger's Equipment filter needs the whole pool's subtype
	// table; the ward lines are what this test reads
	var lines []string
	for _, l := range strings.Split(string(src), "\n") {
		if !strings.HasPrefix(l, "T:") && !strings.HasPrefix(l, "SVar:Trig") && !strings.HasPrefix(l, "SVar:DB") {
			lines = append(lines, l)
		}
	}
	d, rs := kwardpowBuild(t, "Raubahn, Bull of Ala Mhigo", strings.Join(lines, "\n"))
	if len(rs) != 0 {
		t.Fatalf("Raubahn refused: %v", rs)
	}
	if !d.WardLifePower || d.Keywords&kop.KwWard == 0 || d.WardN != 0 || d.WardLife != 0 {
		t.Errorf("Raubahn: WardLifePower %v KwWard %v WardN %d WardLife %d", d.WardLifePower, d.Keywords&kop.KwWard != 0, d.WardN, d.WardLife)
	}
	syn := "Name:Ward Probe\nManaCost:1 R\nTypes:Creature Human\nPT:2/3\nK:Ward:PayLife<X/life equal to CARDNAME's toughness>\nSVar:X:Count$CardToughness\n"
	_, rs = kwardpowBuild(t, "Ward Probe", syn)
	var got []string
	for _, r := range rs {
		got = append(got, r.Reason)
	}
	if all := strings.Join(got, " | "); !strings.Contains(all, `ward life cost X "Count$CardToughness"`) {
		t.Errorf("toughness ward: refusals %q, want the ward life cost X refusal", all)
	}
}
