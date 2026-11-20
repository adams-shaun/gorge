package oracle

import "testing"

// CR 601.2f with CR 601.2g: Thorn of Amethyst raises a noncreature spell's
// cost by {1} even when the spell is cast without paying its mana cost (118.9d),
// and the caster may activate mana abilities while paying. Urza, Lord High
// Artificer's {5} holds Lash Out and Vibrating Sphere in exile under a free
// may-play grant; with an empty floating pool gorge's potential planner still
// prices both casts (the 0/0 Construct token's "tap an untapped artifact you
// control: Add {U}" pays the raise), so both are in its option list. The
// kernel's autopay surface gated a cost-waived cast on the floating pool alone
// and withheld both (tail-64 pair 75, decision 848).
func TestMBCLashOutReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_mayplay_raise_potential")
}
