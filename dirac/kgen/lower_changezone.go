package kgen

// changeZone lowers api:ChangeZone (changeZoneMove). ForgetChanged$ True
// rides as a move flag through many helper lowerings; the
// kernel honors it on exactly four emitted ops, so any other op that carries
// it is refused by name here, whichever helper emitted it.
func (lc *lowerCtx) changeZone(ps *params) error {
	atEOT, hasAtEOT, err := lc.changeZoneAtEOT(ps) // lower_ateot.go
	if err != nil {
		return err
	}
	start := len(lc.ab.Prog)
	if err := lc.changeZoneMove(ps); err != nil {
		return err
	}
	if hasAtEOT {
		if err := lc.emitChangeZoneAtEOT(start, atEOT); err != nil {
			return err
		}
	}
	for i := start; i < len(lc.ab.Prog); i++ {
		in := &lc.ab.Prog[i]
		fl := flagsOf(in.CS)
		if _, ok := fl["ForgetChanged"]; !ok {
			continue
		}
		switch in.Op {
		case "Return", "Bounce", "Exile", "Search":
		default:
			return refuse("ForgetChanged$ on a %s", in.Op)
		}
		if _, ok := fl["ForgetOther"]; ok {
			return refuse("ForgetChanged$ with ForgetOtherRemembered$")
		}
		if _, ok := fl["AlsoSelf"]; ok {
			return refuse("ForgetChanged$ with ThisDefinedAndTgts$")
		}
	}
	return nil
}
