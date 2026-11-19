package oracle

import "testing"

// Auron, Venerated Guardian's attack trigger captures Auron and its
// RememberAmount$ +1/+1 counter is remembered as an Auron entry: gorge's
// ImmediateTrigger excluded the capture twice (once for the parent set, once
// inside Count$RememberedNumber), ate the entry and never put the "when you
// do" exile on the stack; the kernel did (decision 329 of tail-67 pair 5).
func TestMBCAuronVeneratedGuardianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "auron_attack_reflexive_remember_amount")
}

// Denry Klin, Editor in Chief enters with a chosen counter and its own
// "whenever a nontoken creature you control enters, if Denry has counters"
// triggers on itself: gorge puts the counter before the enters triggers are
// checked; the kernel fired them before the as-enters pick was answered, so
// the intervening if saw no counter (decision 243 of tail-18 pair 6).
func TestMBCDenryKlinEditorInChiefReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "denry_enters_counter_pick_self_trigger")
}

// Nekusar, the Mindrazer's draw-step trigger and its "whenever an opponent
// draws" trigger wait together: gorge queues the step's trigger first (its
// Step event precedes the turn draw), the kernel queued the draw's first, so
// the same trigger-order answer stacked them the other way (decision 186 of
// tail-06 pair 5).
func TestMBCNekusarTheMindrazerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "nekusar_draw_step_trigger_order")
}

// Feed the Pack's "you may sacrifice a nontoken creature. If you do, create
// X Wolves" checks ConditionCheckSVar$ X (the sacrificed toughness) after the
// trigger's own cost is paid; kgen put the condition before the payment, so
// the kernel skipped the ask gorge poses (decision 364 of tail-05 pair 7).
func TestMBCFeedThePackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "feed_the_pack_condition_after_trigger_cost")
}

// Blur flickers Bone Shredder in response to its echo trigger: the permanent
// that returns is a new object (CR 400.7), so the echo trigger asks nothing
// and sacrifices nothing. gorge kept the object id and asked the echo
// payment; the kernel skipped it (decision 183 of dr-mbc-aaab pair 20).
func TestMBCBlurReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blur_flickered_echo_source_new_object")
}

// White Orchid Phantom's "destroy up to one target nonbasic land. Its
// controller may search" with no land chosen: gorge's TargetedController names
// no one and nothing is searched; the kernel fell back to the trigger's
// controller and posed a search (decision 586 of tail-72 pair 86).
func TestMBCWhiteOrchidPhantomReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "white_orchid_empty_target_controller_search")
}
