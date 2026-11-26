package oracle

import "testing"

// Pariah redirects combat damage from its controller to the enchanted creature
// (Diffusion Sliver wearing Sword of Feast and Famine, protection from black);
// the attacker, Shadowheart, Dark Justiciar, is black. CR 702.16e prevents the
// redirected damage. gorge's protection guard read only the original recipient
// (the player), so gorge killed the Sliver and dropped Claustrophobia and
// Pariah into the graveyard while the kernel (and Forge) prevented it.
// Recorded as tail-06 pair 169 (surface autopay).
func TestRedirectedDamageProtectionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "redirected_damage_protection")
}
