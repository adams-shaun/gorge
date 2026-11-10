package kgen

import (
	"strings"
)

// publicPick lowers a Hidden$ True ChangeZone over a public origin with no
// object selector (gorge effHiddenPick): Origin$ Battlefield -> Hand or
// Exile, or Origin$ Graveyard -> Hand or Exile, or Origin$ Exile ->
// Graveyard. The pool is the matching
// cards of every player's zone (seat order; game-wide), of that zone's
// player with ValidTgts$ a player and DefinedPlayer$ Targeted (SrOwnZone on
// a battlefield origin; a graveyard walk is that player's zone anyway), or,
// with a DefinedPlayer$ whose spelling the kernel holds as a Ref, of the
// player that Ref names (gorge gameWide is false: the pick loops over the
// resolved players' zones). The chooser is the zone's player, or the
// controller with Chooser$ You; ChangeNum$ a constant; Mandatory$ True Min
// = the bound, else Min 0; Optional$ True/You poses the yes/no confirmation
// first; RememberChanged$ remembers the moved cards. A game-wide graveyard
// pick (no ValidTgts$, no DefinedPlayer$) is lowered only over an Exile
// destination, where both engines enter the card under its owner; over a
// hand destination gorge would enter an opponent's card into its owner's
// hand, which the kernel's per-player walk cannot pose. An Exile pick is
// game-wide only: gorge effHiddenPick's gameWide is true for the public
// origin with no fetch player, and the kernel's exwPickCands always walks
// every seat's exile, so a scoping parameter is refused rather than posed
// against a pool the kernel would not narrow. Without Hidden$ gorge takes
// the object path, not lowered here.
func (lc *lowerCtx) publicPick(ps *params, f *lFilter, n lValue, flags []string, counterKind string, override *lOverride, origin, dest int) error {
	if !ps.is("Hidden", "True") {
		return refuse("ChangeType$ move %s->%s without Hidden$", zoneLabel[origin], zoneLabel[dest])
	}
	if origin == zExile {
		for _, k := range []string{"ValidTgts", "DefinedPlayer", "Chooser"} {
			if _, ok := ps.get(k); ok {
				return refuse("exile pick %s$: the kernel's exile walk is game-wide", k)
			}
		}
	}
	if counterKind != "" || override != nil {
		return refuse("public pick with counters/override")
	}
	// a constant must be >= 1; the one dynamic bound admitted is the plain
	// Count$Valid battlefield count (Expose the Culprit's any-number pick,
	// the kernel evaluates it at resolution)
	plainCount := n.Kind == vCount && n.Mul == 0 && n.Div == 0 && n.Add == 0 && !n.Neg
	if n.isConst() {
		if n.N < 1 {
			return refuse("public pick ChangeNum$ %v", n)
		}
	} else if !plainCount {
		return refuse("public pick ChangeNum$ %v", n)
	}
	if _, ok := ps.get("ExileFaceDown"); ok {
		// the intermediate face-down exile is unobservable when the chained
		// Cloak moves every remembered card to the battlefield face down
		if origin != zBattlefield || dest != zExile || !ps.is("ExileFaceDown", "True") || !ps.is("RememberChanged", "True") || ps.is("WithMayLook", "True") {
			return refuse("public pick ExileFaceDown$ shape")
		}
		ps.consume("ExileFaceDown")
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("public pick filter %s", f)
	}
	cs := []string{"from=" + zoneLabel[origin]}
	for _, fl := range flags {
		switch fl {
		case "Mandatory", "Remember":
			cs = append(cs, fl)
		default:
			return refuse("public pick flag %s", fl)
		}
	}
	owner := lRef{Kind: rYou}
	if vt, ok := ps.get("ValidTgts"); ok {
		pf, err := lc.filter(vt)
		if err != nil {
			return err
		}
		scoped := ps.str("DefinedPlayer") == "Targeted"
		if !scoped && origin == zBattlefield && ps.str("DefinedPlayer") == "" && ps.str("Chooser") == "Targeted" && onlyTargetedCtrl(f) {
			// Teferi, Temporal Pilgrim's "target opponent chooses a
			// permanent they control": no DefinedPlayer$, so gorge's pool is
			// game-wide and ChangeType$ ...TargetedPlayerCtrl scopes it to
			// the target (Chooser$ Targeted)
		} else if len(pf.Clauses) != 0 || pf.Players == 0 || !scoped {
			return refuse("public pick ValidTgts$ %q DefinedPlayer$ %q", vt, ps.str("DefinedPlayer"))
		}
		owner = lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf})}
		ps.consume("TgtPrompt")
		if scoped {
			cs = append(cs, "OwnZone")
		}
	} else if origin == zGraveyard {
		// No target player: gorge's pool is game-wide when no fetch player
		// was named (gameWide), else the DefinedPlayer$ spellings' zones --
		// the same player a Ref already names, so the pool lowers through
		// lc.definedPlayer and no target slot is declared here.
		if dp := ps.str("DefinedPlayer"); dp != "" {
			o, err := lc.definedPlayer(dp)
			if err != nil {
				return refuse("public pick DefinedPlayer$ %q: %v", dp, err)
			}
			switch o.Kind {
			case rTarget:
				owner = o
			case rYou:
				// gorge's fetch player is the controller: the pool is its own
				// graveyard, the kernel's default
			default:
				return refuse("public pick DefinedPlayer$ %q is not a player Ref", dp)
			}
		} else {
			if dest != zExile {
				return refuse("game-wide graveyard pick to %s", zoneLabel[dest])
			}
			cs = append(cs, "GameGrave")
		}
	}
	if ch, ok := ps.get("Chooser"); ok {
		switch {
		case ch == "You" && owner.Kind == rTarget:
			cs = append(cs, "ChooserYou")
		case ch == "Targeted" && owner.Kind == rTarget, ch == "You" && owner.Kind == rYou:
		default:
			return refuse("public pick Chooser$ %q", ch)
		}
	}
	if o, ok := ps.get("Optional"); ok {
		if ps.is("Mandatory", "True") || (strings.TrimSpace(o) != "True" && strings.TrimSpace(o) != "You") {
			return refuse("public pick Optional$ %q", o)
		}
		cs = append(cs, "Confirm")
	}
	ps.consume("SelectPrompt")
	lc.emit(lInstr{Op: "ChooseMove", Obj: owner, A: n, F: f, C: dest, CS: strings.Join(cs, "|")})
	return nil
}

// onlyTargetedCtrl reports whether every clause of f is controlled by the
// resolution's player target (ChangeType$ ...TargetedPlayerCtrl).
func onlyTargetedCtrl(f *lFilter) bool {
	if f == nil || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoTgtPlayer {
			return false
		}
	}
	return true
}

// graveToBattlefieldFlags admits a graveyard pick onto the battlefield
// (gorge effHiddenPick: Mandatory$ True compulsory, else Min 0 "you may";
// Tapped$ True enters it tapped; GainControl$ True enters it under the
// resolving controller) with no other rider: the move flags Mandatory,
// Tapped, Confirm and UnderYourControl only.
func graveToBattlefieldFlags(flags []string) bool {
	for _, f := range flags {
		if f != "Mandatory" && f != "Tapped" && f != "Confirm" && f != "UnderYourControl" {
			return false
		}
	}
	return true
}

// graveExilePick lowers the hidden pick over Origin$ Graveyard,Exile (graveexile):
// gorge effHiddenPick's game-wide pool walks Graveyard then Exile, each
// player in seat order (zone_hidden.go:33), and the fetch player is the
// controller. Only ChangeType$ with a card filter, Hidden$ True, and a Hand,
// Battlefield or top-of-library destination are read; a constant ChangeNum$ or
// the trigger's amount (the up-to count is evaluated when the pick starts);
// Mandatory$, Optional$ True/You, RememberChanged$, ForgetOtherRemembered$.
func (lc *lowerCtx) graveExilePick(ps *params, f *lFilter, n lValue, flags []string, counterKind string, override *lOverride, dest int) error {
	if counterKind != "" || override != nil {
		return refuse("Graveyard,Exile pick with counters/override")
	}
	if n.Kind != vTriggerAmount && (!n.isConst() || n.N < 1 || n.N > 255) {
		return refuse("Graveyard,Exile pick ChangeNum$ %v", n)
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("Graveyard,Exile pick filter %s", f)
	}
	if !everyRememberedOrImprinted(ps.str("ChangeType")) {
		return refuse("Graveyard,Exile pick ChangeType$ %q without IsRemembered", ps.str("ChangeType"))
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("Graveyard,Exile pick with ValidTgts$")
	}
	if _, ok := ps.get("DefinedPlayer"); ok {
		return refuse("Graveyard,Exile pick with DefinedPlayer$")
	}
	if _, ok := ps.get("Chooser"); ok {
		return refuse("Graveyard,Exile pick with Chooser$")
	}
	if lp, ok := ps.get("LibraryPosition"); ok {
		// the caller admits only the top (0) for a library destination; the
		// kernel's pick into a library places on top without a shuffle
		if dest != zLibrary || strings.TrimSpace(lp) != "0" {
			return refuse("Graveyard,Exile pick with LibraryPosition$ %q", lp)
		}
	} else if dest == zLibrary {
		return refuse("Graveyard,Exile pick to Library without LibraryPosition$")
	}
	if _, ok := ps.get("AnyNumber"); ok {
		return refuse("Graveyard,Exile pick with AnyNumber$")
	}
	cs := []string{"from=Graveyard,Exile"}
	for _, fl := range flags {
		switch fl {
		case "Mandatory", "Remember":
			cs = append(cs, fl)
		case "Tapped":
			cs = append(cs, fl)
		case "UnderYourControl":
			cs = append(cs, fl) // GainControl$: enters under the fetching controller, else its owner
		default:
			return refuse("Graveyard,Exile pick flag %s", fl)
		}
	}
	if ps.is("Mandatory", "True") {
		cs = append(cs, "Mandatory")
	}
	if o, ok := ps.get("Optional"); ok {
		if ps.is("Mandatory", "True") || (strings.TrimSpace(o) != "True" && strings.TrimSpace(o) != "You") {
			return refuse("Graveyard,Exile pick Optional$ %q", o)
		}
		cs = append(cs, "Confirm")
	}
	if ps.is("ForgetOtherRemembered", "True") {
		cs = append(cs, "ForgetOther")
	}
	ps.consume("SelectPrompt", "ShuffleNonMandatory", "ChangeTypeDesc", "ChangeNumDesc")
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: n, F: f, C: dest, CS: strings.Join(cs, "|")})
	return nil
}

// everyRememberedOrImprinted is everyRemembered where an alternative may name
// IsImprinted instead (Patient Naturalist's Land.YouOwn+IsImprinted): the pool
// is the resolution's imprinted cards. lc.filter admits the word only inside
// a chain that wrote the set.
func everyRememberedOrImprinted(ct string) bool {
	for _, alt := range strings.Split(ct, ",") {
		rem := strings.Contains(alt, "IsRemembered") && !strings.Contains(alt, "!IsRemembered")
		imp := strings.Contains(alt, "IsImprinted") && !strings.Contains(alt, "!IsImprinted")
		if !rem && !imp {
			return false
		}
	}
	return strings.TrimSpace(ct) != ""
}

// everyRemembered reports a ChangeType$ whose every comma alternative is
// restricted to the remembered set (Card.IsRemembered). The Graveyard,Exile
// pools are read against the effect's remembered cards only; an owner-free
// filter would pick from every graveyard and exile card (graveexile).
func everyRemembered(ct string) bool {
	for _, alt := range strings.Split(ct, ",") {
		if !strings.Contains(alt, "IsRemembered") || strings.Contains(alt, "!IsRemembered") {
			return false
		}
	}
	return strings.TrimSpace(ct) != ""
}
