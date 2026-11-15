package oracle

import (
	"testing"
)

// CR 602.2b/602.2c: Shinka, the Bloodsoaked Keep's "{R}, {T}: Target legendary
// creature gains first strike until end of turn" is only activatable when a
// legal target exists. Pair 128 of tail-44 at land 3ce538a59c: at decision 464
// the kernel offered activate(Shinka,-,1) where gorge offered only pass and
// Jamuraan Lion's activate. The loop filed it as the `options options` class
// holding back 172 cards.
func TestMBCShinkaTheBloodsoakedKeepReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "shinka_pump_offer")
}
