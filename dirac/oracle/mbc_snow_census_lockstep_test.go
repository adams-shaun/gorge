package oracle

import "testing"

// Scrying Sheets' {1}{S} dig is offered by both engines while the floating
// pool holds its snow unit and one more: the pool alone pays the cost
// (CR 107.4h, the {S} pip rides a snow unit of the pool). The kernel once
// withheld the option: its grant budget's census read (affordsUngrantedAv)
// prices the cost through avail.affords, which refuses every {S} cost, so the
// play read as grant-funded and the grant search (which prices no {S} cost
// either) dropped it from the offer. gorge's census prices the {S} pips from
// the pool's snow tally and the rest from the bound, so it keeps the play.
// Recorded as tail-71 pair 82 (surface full), the evidence loop's own game.
func TestMBCScryingSheetsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "snow_census")
}
