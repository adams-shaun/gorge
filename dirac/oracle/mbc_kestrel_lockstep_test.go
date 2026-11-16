package oracle

import "testing"

// Cloudchaser Kestrel's {W}: target permanent becomes white ability is offered
// by the kernel at a priority decision where gorge offers only pass (decision
// 301 of tail-44 pair 1, autopay, seed 1).
//
// The recording replays on pool custom, the table its own decks build: land's
// kgen has since lowered more cards, which shifted the tail buckets and moved
// A-Heartfire Hero (an innocent bystander in the dealt decks) out of tail-44.
func TestMBCCloudchaserKestrelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kestrel_activate_priority")
}
