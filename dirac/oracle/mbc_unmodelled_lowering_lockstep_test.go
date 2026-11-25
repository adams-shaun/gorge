package oracle

import "testing"

// The pool tail-40 game 15 (focus_00376_UR vs focus_00255_RG, seed
// 7897695009980561635) once stalled on the kernel's `lowering` class: a second
// Aggravated Assault activation in the additional main phase the first grant
// opened was a grant shape the kernel did not model. The recording pins the
// game's whole decision path.
func TestMBCTail40ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lowering")
	// CR 701.34a with CR 603.2: seat 1's proliferate puts three kinds on seat
	// 0's Rimebound Dead while seat 0's Generous Patron ("whenever you put
	// counters on a creature you don't control") is the only counter-added
	// listener. Patron hears only its own controller's puts, so the per-kind
	// order reaches no stack; the kernel stalled on it (StallProlif, pair 68).
	stdReproNamedServed(t, "prolif_order_foreign_putter_listener")
	// CR 701.34a: seat 1's Cankerbloom sacrifices to proliferate in its
	// additional main phase; gorge asks which recipients get a counter and the
	// kernel stalled on the ask (StallProlif, pair 47).
	stdReproNamedServed(t, "prolif_choice")
	// CR 707.9b: seat 0's Venser, Fervent Forger copies a permanent twice with
	// "they gain haste" in a pool whose Snow Fortress puts the unmodeled Snow
	// supertype into every override record's KeepSub; the kernel did not find
	// the haste-only record and stalled `lowering` (pair 82).
	stdReproNamed(t, "copy_pump_keyword_record_with_snow_keepsub")
	// CR 701.34a with CR 603.8: seat 1's proliferate puts three kinds on its
	// own Merchant Ship, whose "when you control no Islands" state trigger
	// rides the counter-put event as its only listener. One instance at a
	// time, whatever kind came first, so the per-kind order reaches no stack;
	// the kernel stalled on it (StallProlif, tail-40 pair 163).
	stdReproNamed(t, "prolif_state_trigger_self_listener")
}
