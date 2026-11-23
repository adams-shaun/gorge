package oracle

import "testing"

// CR 601.2g, 605.3b: Grim Backwoods' {2}{B}{G} draw activation beside a
// hybrid filter (Flooded Grove, {G/U}, {T}: add 2 of one colour among G/U)
// and granted/printed U and any-colour sources (the tail-15 fx-wh pool,
// pair 69). gorge's plain prefix search orders the Grove's flattened
// "choose a mana ability" ask so the answers naming a needed colour come
// first (scriptOrderAnswers, need={B,G}): it walks the filter's {G/U} fee
// paid with U before the printed {T}: Add {C}, so the pool ends 1U 2G 1B and
// the Forest is never tapped. The kernel's plain run walked the printed
// fixed {C} line first, floated a useless {C}, and had to fund the play's
// {G} pip from the Forest (pool 2U 1G 1C 1B, Forest tapped) — a projection
// divergence right after the kernel accepted the same activate(...,1) offer.
func TestMBCActFeeOrderPrintedColourOverNeededFilterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_fee_order_plain_run_walks_printed_colour_over_needed_filter")
}
