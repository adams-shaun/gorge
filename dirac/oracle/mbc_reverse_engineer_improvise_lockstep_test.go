package oracle

import "testing"

// CR 702.126a with CR 601.2h: Reverse Engineer ({3}{U}{U}, Improvise) beside
// an untapped artifact, four Forests, an Island and an A-Find the Path aura
// granting the enchanted land "{T}: Add two mana of any one color". Gorge's
// planImprovise withholds the credited artifact and prices the rest from its
// printed census, which holds no granted mana ability, so the credited cost
// is its "cost:credit" decline and the cast is not offered. The kernel's
// planner counted the grant and offered it (tail-62 pair 55); it now prices
// the credited cost on the ungranted census too (improviseSourcesPay).
func TestMBCReverseEngineerImproviseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reverse_engineer_improvise")
}
