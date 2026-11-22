package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:MonarchJail", "OpExt ExtMonarchJail (Obj the target)", "Palace Jailer's ChangeZone to exile with its Effect SubAbility$ (CR 610.3): the target creature is exiled, and a monarch link (genState.links, src the exiled card, ts the Jailer controller's seat + 1) is recorded; becomeMonarch returns the card to the battlefield under its owner's control when a player other than that controller becomes the monarch, and ends the link when the card has left exile. CR 610.3b: when an opponent became the monarch since the exile ability was put on the stack (sfOppCrowned, stamped on every stack ability not controlled by the new monarch), nothing is exiled. Palace Jailer. dirac/act_monarchjail.go."})
}
