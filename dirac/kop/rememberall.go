package kop

// ExtRememberAll: every battlefield permanent matching F joins the
// resolution's remembered set, seat 0's first, each seat's in timestamp
// order (PumpAll RememberPumped$ True: gorge effPumpAll remembers each
// permanent its walk pumps). kgen emits it right before the PumpAll it
// belongs to, over the same filter. dirac/act_rememberall.go.
const ExtRememberAll ExtOp = 306

func init() { extOpNames[ExtRememberAll] = "RememberAll" }
