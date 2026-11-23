package oracle

import "testing"

// Demonmail Hauberk's cast and Sword of Fire and Ice's equip were offered by gorge but
// not by the kernel at p1's priority decision 346 (tail-55-c000 pair 65): p1's lands
// were tapped and only an adapted Incubation Druid, whose counters make its mana
// ability produce three, could fund them. The reflected-amount source's board search
// priced that tap differently in the two engines (CR 106.1, 605.1a).
func TestMBCDemonmailHauberkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "incubation_druid_adapted_mana_funds_equipment_offer")
}
