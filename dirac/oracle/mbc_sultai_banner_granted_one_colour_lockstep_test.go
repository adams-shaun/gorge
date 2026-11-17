package oracle

import (
	"path/filepath"
	"testing"
)

// CR 106.4: Nature's Embrace grants the enchanted land "{T}: Add two mana of
// any one color". Sultai Banner's "{B}{G}{U}, {T}, Sacrifice Sultai Banner:
// Draw a card" needs three different colours, so those two mana (one colour)
// cannot fund it, and the Banner's own tap cannot either (CR 602.2b, 118.3).
// The kernel counted the granted pair as two independent any-colour units and
// offered the activation; at decision 806 of the tail-19 focus game gorge
// withheld it.
func TestMBCSultaiBannerReprosLockstep(t *testing.T) {
	t.Parallel()
	// Load the recording's pool first: cardsettest skips on a tree with no corpus, and needs no git
	// checkout, where synthReg (inside stdReproNamed) fails "could not resolve git repo root" in a
	// fresh export with no .git (pr-precheck's fresh-clone run).
	r, err := ReadRepro(filepath.Join("testdata", "repro", "sultai_banner_granted_one_colour.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	globSet(t, r.Pool, r)
	stdReproNamed(t, "sultai_banner_granted_one_colour")
}
