package oracle

import "testing"

// Court of Garenbrig (1GG) was offered by dirac at a priority decision where gorge
// did not offer it (decision 206, dr-mbc-aaaa pair 10).
func TestMBCCourtOfGarenbrigReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "court_of_garenbrig_cast_offer")
}
