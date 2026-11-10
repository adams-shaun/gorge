package kgen

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// emblemPlayFrom returns the MayPlayFrom statics of c's lowered emblems.
func emblemPlayFrom(c *lCard) []lStatic {
	var out []lStatic
	for _, e := range c.Emblems {
		for _, st := range e.Statics {
			if st.Kind == "MayPlayFrom" {
				out = append(out, st)
			}
		}
	}
	return out
}

// TestEffectMayLookTop: an Effect static that lets its controller look at and
// play the top card of their library for the turn lowers to an end-of-turn
// emblem holding one library MayPlayFrom static (the look is information
// only and lowers to nothing); Gwenom's life-for-mana alternative rides as
// MayPayLife. Every other spelling refuses by name.
func TestEffectMayLookTop(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("The Belligerent\nGwenom, Remorseless\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	for _, c := range lw.cards {
		if got := mzReasons(c); got != "" {
			t.Fatalf("%s refused: %s", c.Name, got)
		}
		sts := emblemPlayFrom(c)
		if len(sts) != 1 || sts[0].Zone != zLibrary {
			t.Fatalf("%s: %d MayPlayFrom emblem statics (zone %v), want one over the library", c.Name, len(sts), sts)
		}
		if len(c.Emblems) != 1 || !c.Emblems[0].UntilEOT {
			t.Fatalf("%s: emblems %+v, want one end-of-turn emblem", c.Name, c.Emblems)
		}
		pay := slices.Contains(sts[0].Flags, "MayPayLife")
		if pay != (c.Name == "Gwenom, Remorseless") {
			t.Errorf("%s: MayPayLife flag %v", c.Name, pay)
		}
	}

	const head = "Name:Probe\nManaCost:1 U\nTypes:Creature Bear\nPT:1/1\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ Go | TriggerDescription$ Probe.\n" +
		"SVar:Go:DB$ Effect | StaticAbilities$ Play\n" +
		"SVar:Play:Mode$ Continuous | Affected$ %s | AffectedZone$ Library | %s | Description$ Probe.\n"
	const ok = "MayLookAt$ You | MayPlay$ True"
	if r := mzReasons(mzLower(t, fmt.Sprintf(head, "Card.TopLibrary+YouCtrl", ok))); r != "" {
		t.Fatalf("the probe shape refused: %s", r)
	}
	for _, tc := range []struct{ name, aff, rider, want string }{
		{"revealed to all", "Card.TopLibrary+YouCtrl", "MayLookAt$ Player | MayPlay$ True", "MayLookAt"},
		{"deeper cards", "Card.YouCtrl", ok, "MayLookAt"},
		{"opponent's top", "Card.TopLibrary+OppCtrl", ok, "MayLookAt"},
		{"other alternative cost", "Card.TopLibrary+YouCtrl", ok + " | MayPlayAltManaCost$ PayLife<2>", "MayLookAt"},
		{"free beside it", "Card.TopLibrary+YouCtrl", ok + " | MayPlayWithoutManaCost$ True", "MayLookAt"},
		{"capped", "Card.TopLibrary+YouCtrl", ok + " | MayPlayLimit$ 1", "MayLookAt"},
	} {
		r := mzReasons(mzLower(t, fmt.Sprintf(head, tc.aff, tc.rider)))
		if !strings.Contains(r, tc.want) {
			t.Errorf("%s: refusal %q, want it to name %s", tc.name, r, tc.want)
		}
	}
}
