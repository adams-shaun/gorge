package oracle

import "testing"

// CR 707.9b with CR 613.1: a copy that carries its template's Continuous
// static grants the static's ability from the template's card, and an
// as-enters copy grant of the same ability index (Protean Thaumaturge's
// "except it has this ability", CR 707.9a) does not take the grant over.
// Vigilant Sentry's threshold pump on a Protean copy read its target spec
// from the Protean card's own ability 0 (the constellation trigger's
// "another target creature"), so the kernel offered an Archpriest of Iona
// that is neither attacking nor blocking beside the two attackers the
// "target attacking or blocking creature" slot allows (CR 601.2c).
func TestMBCArchpriestOfIonaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_static_grant_beside_same_index_copy_grant")
}
