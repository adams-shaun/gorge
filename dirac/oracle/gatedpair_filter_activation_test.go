package oracle

import "testing"

// A gate-on same-colour pair (Whisperer of the Wilds: {T}: Add {G}, then a
// ferocious {T}: Add {G}{G}) funds a filter's fee and the rest of the play:
// Stallion of Ashmouth's {1}{B} Delirium pump with no other source untapped is
// payable from the Whisperer's {G}{G} and Viridian Acolyte's {1},{T}: any
// colour ({G} pays the fee, the {B} the pip, the other {G} the {1}). gorge's
// ability ask lists both of the Whisperer's abilities and its prefix search
// takes each in turn; the kernel's filter search tapped only ability 0 and
// withheld the activation (lockstep pair 93 of the Lychguard focus pool, decision
// 621: gorge offered activate(Stallion), the kernel only pass).
func TestGatedPairFilterActivationLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gated_pair_filter_funds_activation")
}
