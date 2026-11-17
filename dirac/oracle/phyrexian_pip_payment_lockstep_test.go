package oracle

import "testing"

// A Phyrexian pip's colour face is offered by the announcement's feasibility
// and paid by the activation's payment. Three shapes the pip ask, the offer
// gate and the payment must agree on:
//   - a colour face the offer walk can reach (a paid filter) but the payment
//     cannot: gorge reverses the activation it cannot pay (CR 733.1), so the
//     kernel must not panic (the focus run's pair 11);
//   - a colour face a restricted source or a floating restricted batch pays
//     (Gwenna's creature-only {R}): the pip ask reads the same restricted
//     mana the payment spends (CR 106.6), so both faces stay offered (pair 106);
//   - a colour face a storage battery's live counters pay (Subterranean
//     Hangar's announced X): the offer gate prices the counters the planner
//     does, so a second activation stays offered (pair 149).
func TestPhyrexianPipPaymentLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "repro/phyrexian_pip_unfunded_colour_face", "tail-42", autopay)
	synthReproLockstep(t, "repro/phyrexian_pip_restricted_colour_face", "tail-42", autopay)
	synthReproLockstep(t, "repro/phyrexian_pip_storage_battery", "tail-42", autopay)
}
