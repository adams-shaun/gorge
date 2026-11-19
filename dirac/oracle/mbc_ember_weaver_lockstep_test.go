package oracle

import "testing"

// Ember Weaver castability at priority: gorge and the kernel part on whether
// {2}{G} is payable (dr-mbc-aaaa pair 10, decision 221).
func TestMBCEmberWeaverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ember_weaver_cast_offer")
}
