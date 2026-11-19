package oracle

import "testing"

// CR 702.66a with CR 601.2h: Reverse Engineer ({3}{U}{U}, Improvise) beside
// four Forests, an Island and an A-Find the Path aura granting the enchanted
// land "{T}: Add two mana of any one color" is payable: the artifact credits
// a generic and the granted ability pays the blue pips. Gorge's potential
// planner once withheld the cast as "cost:credit" because its printed census
// cannot see the granted ability, while the kernel offered it (tail-62 pair
// 55). The potential prefix fallback now prices the granted source.
func TestMBCReverseEngineerImproviseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reverse_engineer_improvise")
}
