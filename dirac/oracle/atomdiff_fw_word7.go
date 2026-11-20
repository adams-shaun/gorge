package oracle

import "github.com/adams-shaun/gorge/dirac"

// The PredWord7 filter words (kop.W7*) that read no target, trigger object or
// imprint: each is answered by a probe's source and controller alone. The
// words that need a referent (greatestPowerControlledByTargeted,
// sharesCreatureTypeWith TriggeredCard, blockingTriggeredAttacker, IsImprinted)
// are the Defined$ referent ticket's.
func init() {
	for _, w := range []string{
		"Card.faceUp", "Card.noName", "Card.Snow", "Card.nonSnow", "Card.ManaCostPartialBlue",
		"Card.IsCommander", "Card.!IsCommander", "Card.BottomLibrary", "Card.!BottomLibrary", "Card.IsNotChosenType",
		"Card.inZoneGraveyard", "Card.inZoneHand", "Card.inZoneLibrary", "Card.inZoneExile", "Card.inZoneStack",
	} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone})
	}
	// kgen refuses these on a permanent (the word reads the zone the object
	// came from or its printed abilities); gorge answers them everywhere
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.NoAbilities", GorgeSpec: "Card.NoAbilities", Ctx: AtomAnyZone &^ AtomBattlefield})
	// the entered ledger is kept only for a table that reads it
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.ThisTurnEnteredFrom_Battlefield", GorgeSpec: "Card.ThisTurnEnteredFrom_Battlefield", Ctx: AtomAnyZone &^ AtomBattlefield, Needs: dirac.BankEntered})
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.ThisTurnEnteredFrom_Library", GorgeSpec: "Card.ThisTurnEnteredFrom_Library", Ctx: AtomAnyZone &^ AtomBattlefield, Needs: dirac.BankEntered})
	for _, w := range []string{
		"Card.inZoneBattlefield", // kgen lowers it to no predicate: implied by the battlefield context it is read in (filter.go propWords)
		"Creature.greatestPower", "Creature.attackingAlone", "Creature.attackingSame", "Card.attacking Opponent",
		"Card.!blocking", "Card.blockingSource", "Card.blockedBySource", "Card.EnchantedPlayerCtrl",
		"Card.EnchantedBy Aura.YouCtrl", "Card.doesNotShareNameWith OtherYourBattlefield", "Card.sharesNameWith Valid Permanent.Legendary",
		"Card.AttachedTo Card", "Card.AttachedTo Creature", "Card.AttachedTo Creature.YouCtrl", "Card.AttachedTo Permanent.Other",
	} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomBattlefield})
	}
}
