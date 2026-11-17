package kgen

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// mayPlayAltManaCostForms: a MayPlay$ static's MayPlayAltManaCost$ is an
// alternative cost on casts made through that permission (CR 118.9). The
// kernel carries the life alternative (kop.StMayPayLife) on a library-top or
// exile permission; a fixed mana cost has no field on the static and stays
// refused. Valgavoth and Glimpse also carry a Moved replacement that is
// refused apart from the static, so those two cards stay refused as a whole.
var mayPlayAltManaCostForms = []formCase{
	{card: "Bolas's Citadel", form: "library-top PayLife<ConvertedManaCost> permission", lir: "static MayPlayFrom F={types=0x0 ctrl=1} F2=nil cond=true P=- T=- kw=[] addsub=[] amount=- zone=4"},
	{card: "Valgavoth, Terror Eater", form: "exile ExiledWithSource PayLife<ConvertedManaCost> on your turn", lir: "static MayPlayFrom F={types=0x0 ExiledWith} F2=nil cond=YourTurn()", refused: "repl-hard: dies-to-exile from any zone"},
	{card: "Glimpse the Cosmos", form: "fixed mana alternative {U} has no kernel field", refused: `MayPlayAltManaCost "U"`},
}

func TestMayPlayAltManaCostForms(t *testing.T) {
	checkForms(t, mayPlayAltManaCostForms)

	// the dump does not print a MayPlayFrom static's flags: the life
	// alternative is read off the lowered static
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join([]string{"Bolas's Citadel", "Valgavoth, Terror Eater", "Glimpse the Cosmos"}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range lowerPool(p).cards {
		var flags []string
		n := 0
		for _, st := range c.Statics {
			if st.Kind == "MayPlayFrom" {
				n++
				flags = st.Flags
			}
		}
		want := c.Name != "Glimpse the Cosmos"
		if n != b2i(want) || slices.Contains(flags, "MayPayLife") != want {
			t.Errorf("%s: %d MayPlayFrom statics, flags %v, want MayPayLife=%v", c.Name, n, flags, want)
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
