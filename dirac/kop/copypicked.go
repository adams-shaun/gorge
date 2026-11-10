package kop

// ExtCopyPicked (CopyPermanent Choices$, Calamity, Galloping Inferno): the
// resolving controller creates one token copy of the object the preceding
// OpChooseCard picked (Obj, RefChosen). Its B.N and C are OpCopyAsTokenEx's
// (the CopyFlags and the copy exceptions' override index+1); the pump and
// NonRemCtrl bits are never set. Unlike OpCopyAsTokenEx on an empty Obj it
// still registers the AtEOT$ riders' delayed trigger when nothing was
// picked (CpSacAtEndDly, CpExileAtEnd: gorge scheduleAtEOTNone, CR 603.7).
// An Aura or a card with an as-enters choice or a clone election stalls
// (StallCopyAllAsk) rather than ask. dirac/copy_picked.go.
const ExtCopyPicked ExtOp = 315

// CopyAttacking: ExtCopyPicked B.N bit 14 (the bit CopyPumpEOT is beside a
// pump keyword, which this op never carries): the token enters tapped or
// untapped attacking the controller's opponent (TokenAttacking$ True, CR
// 508.4; gorge copyAttackRider: players only, so no planeswalker or battle
// defender and no ask). The kernel reads it from the OpCopyAsTokenEx it
// builds for the op.
const CopyAttacking = 1 << 14

func init() { extOpNames[ExtCopyPicked] = "CopyPicked" }
