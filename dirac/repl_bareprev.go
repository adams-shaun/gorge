package dirac

// barePrevOnly reports whether a damage event's n replacement candidates are
// all bare full preventions: every one prevents (prev), none is the SHIELD
// counter's (spent when it applies), a PreventNext shield (spent by the
// amount it prevents) or a plus-N addition. Whichever applies first prevents
// the whole event and the rest have nothing left to apply to (CR 616.1e), so
// gorge applies them in scan order without the CR 616.1 order ask
// (engine/rules allBarePreventions), and neither does the kernel: two
// Prismatic Strands colours, or two Cephalid Illusionist grants (one keyword
// bit here, two Effects in gorge), on one event.
func barePrevOnly(n int, prev, shld, pnx, plus uint8) bool {
	return n >= 2 && shld == 0 && pnx == 0 && plus == 0 && prev == uint8(1<<n-1)
}
