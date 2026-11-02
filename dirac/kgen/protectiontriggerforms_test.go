package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// protectiontriggerformsCarriers is one card per form:
// each must lower with no refusal, and its lowered IR must contain the
// fragment.
var protectiontriggerformsCarriers = map[string]struct{ form, lir string }{
	"White Knight":      {"K:Protection from black (CardDef.ProtColors)", "kw=[First Strike] flash=false protection=B"},
	"Brushwagg":         {"Mode$ Blocks (TrigBlocks)", "trigger Blocks F={types=0x0 Self}"},
	"Baloth Prime":      {"Mode$ Sacrificed with a ValidCard$ filter", "trigger Sacrificed F={types=0x1 ctrl=1}"},
	"Ceremonial Knife":  {"static AddTrigger$ to the equipped creature", "ab[1] Triggered Granted"},
	"Glimpse of Nature": {"an Effect's SpellCast trigger without a Duration$ (EOT emblem)", "emblem[0] trigger=1 statics=0 until=EOT"},
	"Tel-Jilad Chosen":  {"K:Protection:Artifact (CardDef.ProtFrom)", "protfrom={types=0x4}"},
	"Fallowsage":        {"Mode$ Taps of the card itself (TrigTaps)", "trigger Taps F={types=0x0 Self}"},
	"Thragtusk":         {"ChangesZone Battlefield->Any (TrigLeaves)", "trigger Leaves F={types=0x0 Self}"},
}

func TestThisTurnCastProtection(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Emrakul, the World Anew\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(lw.cards))
	}
	c := lw.cards[0]
	for _, r := range c.Refusals {
		if strings.Contains(r.Reason, "ThisTurnCast") {
			t.Errorf("ThisTurnCast still refuses: %s: %s", r.Node, r.Reason)
		}
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	if !strings.Contains(sb.String(), "protfrom={types=0x9f CastByYou EnteredThisTurn}") {
		t.Errorf("ProtFrom lacks the cast-this-turn flags:\n%s", sb.String())
	}
	for _, want := range []string{`a Spell head outside a ReduceCost ValidTarget$`, "unread params [AllValid]"} {
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("expected unrelated refusal %q to remain; refusals: %v", want, c.Refusals)
		}
	}
}

func TestProtectionAndTriggerModeFormsAccepted(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for n := range protectiontriggerformsCarriers {
		names = append(names, n)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	seen := 0
	for _, c := range lw.cards {
		want, ok := protectiontriggerformsCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s (%s): refused: %s", c.Name, want.form, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", c.Name, want.form, want.lir, sb.String())
		}
	}
	if seen != len(protectiontriggerformsCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(protectiontriggerformsCarriers))
	}
}
