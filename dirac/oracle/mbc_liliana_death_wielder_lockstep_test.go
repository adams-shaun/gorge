package oracle

import "testing"

// CR 601.2f, 118.9d: Urza, Lord High Artificer exiles the top card and lets
// its controller play it without paying its mana cost, but Thorn of Amethyst
// still raises the cost. On the autopay surface the kernel offered only pass
// where gorge offered the free may-play cast of Liliana, Death Wielder from
// exile (options options, priority decision 875 for p0, tail-64 pair 36,
// seed 77888, surface autopay, mulligans 0). The planned-zone cast the census
// prices is one of gorge's potential plays, so the offer's raise is paid by
// the planner's sources and freeCastRaisePayable (dirac/legal_gen.go) admits
// it on the autopay surface.
func TestMBCLilianaDeathWielderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "liliana_death_wielder")
}
