package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The CastRemovalDamage contract at the read boundary: a held shape's literal
// NumDmg$ comes back as a positive int32 -- through DamageAmount, NumDmg$'s
// one reader, so a DealDamage spell reads its COMPILED figure exactly as the
// resolution deals it -- and every unknown answers 0 (absent, dynamic,
// non-positive, and an ability whose CastRemovalKind is empty, the pairing
// rule that never carries a damage figure next to an unheld shape).
func TestCastRemovalDamageReadsTheCompiledDealDamage(t *testing.T) {
	cases := []struct {
		name string
		sa   *cards.SA
		want int32
	}{
		{
			name: "literal DealDamage bolt",
			sa:   &cards.SA{API: "DealDamage", Params: map[string]string{"NumDmg": "3", "Defined": "Targeted"}},
			want: 3,
		},
		{
			name: "dynamic X is unknown",
			sa:   &cards.SA{API: "DealDamage", Params: map[string]string{"NumDmg": "X", "Defined": "Targeted"}},
			want: 0,
		},
		{
			name: "absent NumDmg is unknown",
			sa:   &cards.SA{API: "DealDamage", Params: map[string]string{"Defined": "Targeted"}},
			want: 0,
		},
		{
			name: "negative clamps to unknown",
			sa:   &cards.SA{API: "DealDamage", Params: map[string]string{"NumDmg": "-2", "Defined": "Targeted"}},
			want: 0,
		},
		{
			name: "Destroy with no NumDmg is not a damage figure",
			sa:   &cards.SA{API: "Destroy", Params: map[string]string{"Defined": "Targeted"}},
			want: 0,
		},
		{
			name: "unheld API pairs with nothing",
			sa:   &cards.SA{API: "Counter", Params: map[string]string{"NumDmg": "3"}},
			want: 0,
		},
	}
	for _, c := range cases {
		if got := CastRemovalDamage(c.sa); got != c.want {
			t.Errorf("%s: CastRemovalDamage = %d, want %d", c.name, got, c.want)
		}
	}
}
