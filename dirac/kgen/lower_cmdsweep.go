package kgen

// originCommandGrave lowers Campfire's first ChangeZoneAll: "put all
// commanders you own from the command zone and from your graveyard into your
// hand" (Origin$ Command,Graveyard, ChangeType$ Card.IsCommander+YouOwn,
// Destination$ Hand). It is lowered to nothing on the same assumption as
// Command Beacon's pick (kop.W7IsCommander): no seat has a commander, so the
// command zone holds nothing and no graveyard card is a commander, and
// gorge's sweep moves nothing and poses no decision. The rest of the chain
// (the SubAbility's graveyard shuffle) lowers on its own. The cmd5 pool guard
// reads the counter, so a Commander game's card reaching this fails there.
func (lc *lowerCtx) originCommandGrave(ps *params) error {
	if ps.str("Destination") != "Hand" || ps.str("ChangeType") != "Card.IsCommander+YouOwn" {
		return refuse("ChangeZoneAll Origin$ Command,Graveyard outside Destination$ Hand, ChangeType$ Card.IsCommander+YouOwn")
	}
	ps.consume("Origin", "SubAbility")
	if left := ps.unread(); len(left) > 0 {
		return refuse("ChangeZoneAll Origin$ Command,Graveyard with %v", left)
	}
	noteCmdShortcut(cmdShortcutCommandSweep)
	return nil
}
