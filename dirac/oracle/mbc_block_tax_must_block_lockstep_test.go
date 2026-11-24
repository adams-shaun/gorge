package oracle

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Cowed by Wisdom's block tax beside Trumpeting Armodon's "target creature
// blocks it this turn if able" (ExtMustBlock): with the duty withholding Done
// and the tax pricing out every pick, the kernel stalled `blktax` mid-game
// (decision 574 of tail-14-c000 pair 83). The pair is refused when the matchup
// is built, as every other block floor or duty beside a block tax is, so the
// recorded game must no longer build.
func TestMBCTail14ReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "block_tax_beside_must_block_duty.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := globSet(t, r.Pool, r)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	for _, n := range []string{"Cowed by Wisdom", "Trumpeting Armodon"} {
		if _, ok := set.Lookup(n); !ok {
			t.Fatalf("precondition: %s is not in the pool %s", n, r.Pool)
		}
	}
	_, _, _, err = set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	var ref *cardset.InteractionRefusal
	if !errors.As(err, &ref) {
		t.Fatalf("recorded pair: err %v, want an InteractionRefusal", err)
	}
	if ref.Reason != "blktax" || !slices.Contains(ref.Cards, "Cowed by Wisdom") || !slices.Contains(ref.Cards, "Trumpeting Armodon") {
		t.Fatalf("refused %s naming %v, want blktax naming Cowed by Wisdom and Trumpeting Armodon", ref.Reason, ref.Cards)
	}
}
