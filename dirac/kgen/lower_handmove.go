package kgen

import (
	"strings"
)

// lineReadsX is gorge abilityReferencesX's token test on one param value.
func lineReadsX(v string) bool {
	for _, tok := range strings.FieldsFunc(v, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) {
		if tok == "X" {
			return true
		}
	}
	return false
}

// handMove lowers a hidden-origin ChangeZone Origin$ Hand -> Battlefield,
// Exile or Library: gorge effChangeZoneHand / effChangeZoneHandOwners (both
// handMoveOwnersWalk): the hand's owner is the controller, or the player a
// ValidTgts$ names (DefinedPlayer$ Targeted beside it is that target); the
// chooser is the owner unless Chooser$ You (the controller); ChangeNum$ (a
// constant) cards matching ChangeType$ move. Optional$ True/You poses the
// yes/no confirmation first and makes the pick optional (Min 0); Mandatory$
// True makes it required, a required move with no more eligible cards than
// the bound taking them all unasked; a markerless line's optionality is
// gorge's reading of its SpellDescription$ and Oracle text
// (handMoveTextOptional, copied in handTextOptional), refused when that
// reading is unknown. A library destination with the Bottom flag goes to the
// bottom (LibraryPosition$ -1), else the top. Battlefield entries are the
// owner's (tapped with Tapped$); RememberChanged$ remembers the moved cards.
// Refused: a filter that may admit an Aura onto the battlefield in a move of more than one card, counters / StaticEffect$,
// other selectors and every other parameter.
func (lc *lowerCtx) handMove(ps *params, f *lFilter, n lValue, flags []string, counterKind string, override *lOverride, dest int) error {
	if counterKind != "" || override != nil {
		return refuse("hand move with counters/override")
	}
	if v, ok := ps.get("AlreadyRevealed"); ok {
		// presentation only: Forge skips re-revealing a hand just revealed;
		// gorge has no reader (its own unread census,
		// engine/rules/testdata/paramcensus-sets/DFT.tsv:1) and the hand
		// pick moves the same card either way.
		if v != "True" {
			return refuse("hand move AlreadyRevealed$ %q", v)
		}
	}
	counted := n.Kind == vCountHand // "any number" over a Count$ValidHand SVar (gorge handMoveCountOf: resolved once, at resolution)
	if !counted && (!n.isConst() || n.N < 1) {
		return refuse("hand move ChangeNum$ %v", n)
	}
	for _, fl := range flags {
		if (fl == "Bottom" || fl == "PosAsk") && dest != zLibrary {
			return refuse("hand move Bottom flag to %s", zoneLabel[dest])
		}
		if fl != "Tapped" && fl != "Mandatory" && fl != "Remember" && fl != "Bottom" && fl != "PosAsk" {
			return refuse("hand move flag %s", fl)
		}
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("hand move filter %s", f)
	}
	if v, ok := ps.get("Attacking"); ok {
		// the moved card enters attacking (gorge classifyAttackingEntryText;
		// kernel SrAttacking): a battlefield entry only
		if v != "True" || dest != zBattlefield {
			return refuse("hand move Attacking$ %q to %s", v, zoneLabel[dest])
		}
		flags = append(flags, "Attacking")
	}
	if dest == zLibrary && ps.is("Shuffle", "True") {
		// gorge handLibraryTail shuffles instead of placing
		return refuse("hand move of %d cards shuffled into the library", n.N)
	}
	// gorge handLibraryTail puts the answered cards on top in answer order
	// (the first answered on top; kernel moveFound), or on the bottom in
	// answer order with LibraryPosition$ -1
	if dest == zBattlefield {
		for _, c := range f.Clauses {
			if c.Types == 0 || c.Types&tEnchantment != 0 {
				// CR 303.4f/g: an Aura put onto the battlefield enchants what its
				// controller chooses as it enters, or stays in the hand when
				// nothing is legal (gorge auraEntryGate); the kernel's parked
				// move is one card
				if counted || !n.isConst() || n.N != 1 || f.Each {
					return refuse("hand move filter %s may admit an Aura", f)
				}
				flags = append(flags, "AuraEntry")
				break
			}
		}
	}
	owner := lRef{Kind: rYou}
	dp, _ := ps.get("DefinedPlayer")
	eachOpp := lc.repeatPl == 0 && handOwnersEachOpp(dp)       // lower_handmoveeach.go
	if o, ok, err := lc.handMoveDefinedOwner(ps); err != nil { // lower_handmove.go
		return err
	} else if ok {
		owner = o
	}
	if vt, ok := ps.get("ValidTgts"); ok {
		pf, err := lc.filter(vt)
		if err != nil {
			return err
		}
		if len(pf.Clauses) != 0 || pf.Players == 0 {
			return refuse("hand move ValidTgts$ %q", vt)
		}
		owner = lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf})}
		ps.consume("TgtPrompt")
		if dp, ok := ps.get("DefinedPlayer"); ok && dp != "Targeted" && dp != "TargetedPlayer" {
			return refuse("hand move DefinedPlayer$ %q beside ValidTgts$", dp)
		}
	}
	chooserYou := false
	if ch, ok := ps.get("Chooser"); ok {
		switch {
		case ch == "You":
			chooserYou = owner.Kind != rYou
		case ch == "Targeted" && owner.Kind == rTarget:
		default:
			return refuse("hand move Chooser$ %q", ch)
		}
	}
	// the imprint mox's pick is gorge zone_change.go's imprint ask, not the
	// generic hand mover: ChangeNum$ cards, unasked when no more qualify (the
	// trigger's OptionalDecider$ is the "you may"; lower_imprintmox.go)
	mandatory := ps.is("Mandatory", "True") || lc.imprintMox && dest == zExile
	optional := false
	if o, ok := ps.get("Optional"); ok {
		if mandatory {
			return refuse("hand move both Optional$ and Mandatory$")
		}
		switch strings.TrimSpace(o) {
		case "True", "You":
			optional = true
			flags = append(flags, "Confirm")
		default:
			return refuse("hand move Optional$ %q", o)
		}
	}
	if f.Each && !optional {
		// gorge takes a required EACH move whole when every group fits
		// (zone_hand.go); only the may-form asks per type
		return refuse("required hand move of one card per type")
	}
	if !mandatory && !optional {
		verb := "put"
		if dest == zExile {
			verb = "exile"
		}
		opt, known := handTextOptional(ps.str("SpellDescription")+"\n"+lc.face.Oracle, verb)
		if !known {
			return refuse("markerless hand move of unknown optionality")
		}
		if !opt {
			mandatory = true
		}
	}
	ps.consume("SpellDescription", "StackDescription")
	cs := []string{"from=Hand"}
	for _, fl := range flags {
		if fl != "Mandatory" {
			cs = append(cs, fl)
		}
	}
	if mandatory {
		cs = append(cs, "Mandatory")
	}
	if chooserYou {
		cs = append(cs, "ChooserYou")
	}
	if untilLeaves, err := handMoveUntilLeaves(ps, dest); err != nil { // lower_handmove.go
		return err
	} else if untilLeaves {
		cs = append(cs, "UntilLeaves")
	}
	if v, ok := ps.get("ForgetOtherRemembered"); ok {
		// gorge effChangeZoneHand clears the resolution's remembered set
		// before the move (and when the pool is empty); RememberChanged$
		// re-adds the moved cards. The kernel clears SrForgetOther's set
		// after the picks and before moveFound (dirac/interp_ask.go
		// finishSearchNow), so a pick filter reading Card.IsRemembered stays
		// exact.
		if v != "True" {
			return refuse("hand move ForgetOtherRemembered$ %q", v)
		}
		cs = append(cs, "ForgetOther")
	}
	in := lInstr{Op: "ChooseMove", Obj: owner, A: n, F: f, C: dest, CS: strings.Join(cs, "|")}
	if eachOpp && owner.Kind == rEachOpp {
		return lc.handMoveEachOpp(in, counted) // lower_handmoveeach.go
	}
	lc.emit(in)
	return nil
}

// handTextOptional is gorge effects.handMoveTextOptional (zone_hand.go):
// the may-forms of the hand move's own sentence in the lowered card text,
// for a move whose verb is action ("put" for the battlefield).
func handTextOptional(text, action string) (optional, known bool) {
	text = strings.ToLower(text)
	if handPhraseMentionsHand(text, "may "+action) || handPhraseFollowsHandReveal(text, "may "+action) ||
		(handPhraseMentionsHand(text, "any number") && handPhraseMentionsHand(text, action)) {
		return true, true
	}
	if handPhraseSupportsRequiredMove(text, action) || handPhraseFollowsHandChoice(text, action) {
		return false, true
	}
	return false, false
}

// The four helpers are gorge's handMovePhrase* (effects/zone_hand.go).

func handPhraseSupportsRequiredMove(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		if strings.Contains(text[begin:end], "hand") || strings.Contains(text[begin:end], "choose") {
			return true
		}
		start = i + len(phrase)
	}
}

func handPhraseFollowsHandChoice(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		contextStart := begin
		for n := 0; n < 3 && contextStart > 0; n++ {
			prior := strings.TrimRight(text[:contextStart], ".; ")
			if j := strings.LastIndexAny(prior, ".;"); j >= 0 {
				contextStart = j + 1
			} else {
				contextStart = 0
			}
		}
		context := text[contextStart:end]
		if (strings.Contains(text[begin:end], "that card") || strings.Contains(text[begin:end], " it")) &&
			strings.Contains(context, "hand") && strings.Contains(context, "choose") && strings.Contains(context, "it") {
			return true
		}
		start = i + len(phrase)
	}
}

func handPhraseFollowsHandReveal(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		prior := strings.TrimRight(text[:begin], ".; ")
		if j := strings.LastIndexAny(prior, ".;"); j >= 0 {
			prior = prior[j+1:]
		}
		if strings.Contains(prior, "hand") && strings.Contains(text[begin:end], "it") {
			return true
		}
		start = i + len(phrase)
	}
}

func handPhraseMentionsHand(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		if strings.Contains(text[i:end], "hand") {
			return true
		}
		start = i + len(phrase)
	}
}

// handPutBack lowers Forge's hidden-origin hand put-back with no object
// selector (Brainstorm's "put two cards from your hand on top of your
// library"; gorge zone_change.go routes an Origin$ Hand ChangeZone without
// ChangeType$/Defined$/DefinedPlayer$/ValidTgts$ to effChangeZoneHand,
// whose pool is the whole hand). It is the hand move over ChangeType$
// Card. LibraryPosition$ 0 puts the cards on top and -1 on the bottom
// (gorge handLibraryTail's libraryOrderPlacement). Reorder$ is read only by
// gorge's library-search placement (zone_library.go placeLibraryObjects),
// never on this path: consumed.
func (lc *lowerCtx) handPutBack(ps *params, origin, dest, pos int, flags []string, counterKind string, override *lOverride) (bool, error) {
	if origin != zHand || dest != zLibrary || pos != 0 && pos != -1 {
		return false, nil
	}
	for _, k := range []string{"ChangeType", "Defined", "DefinedPlayer", "ValidTgts", "Hidden"} {
		if _, ok := ps.get(k); ok {
			return false, nil
		}
	}
	n, err := lc.amount(ps, "ChangeNum", 1)
	if err != nil {
		return true, err
	}
	if pos == -1 {
		flags = append(flags, "Bottom")
	}
	if handPosAsk(ps, pos) { // lower_destalt.go
		flags = append(flags, "PosAsk")
	}
	if ps.is("Mandatory", "True") {
		flags = append(flags, "Mandatory")
	}
	if v, ok := ps.get("Reorder"); ok && strings.TrimSpace(v) != "True" {
		return true, refuse("Reorder$ %q", v)
	}
	ps.consume("Reorder")
	f, err := lc.filter("Card")
	if err != nil {
		return true, err
	}
	return true, lc.handMove(ps, f, n, flags, counterKind, override, dest)
}

// handMoveDefinedOwner reads a hand move's DefinedPlayer$ Targeted with no
// ValidTgts$ of its own (Deep-Cavern Bat's ExileCard under a RevealHand
// that targets the opponent): gorge effChangeZoneHandOwners takes the hand
// owner from the chain's player target. Exactly one player target slot
// must exist on the ability.
func (lc *lowerCtx) handMoveDefinedOwner(ps *params) (lRef, bool, error) {
	dp, ok := ps.get("DefinedPlayer")
	if !ok {
		return lRef{}, false, nil
	}
	if _, has := ps.get("ValidTgts"); has {
		return lRef{}, false, nil // the ValidTgts$ arm reads it
	}
	if r, err := lc.defined("Spawner>TriggeredTarget"); strings.TrimSpace(dp) == "Spawner>TriggeredTarget" {
		// an ImmediateTrigger spawned by the card's combat damage to a
		// player: the damaged player, the one opponent (Biting-Palm Ninja)
		if err != nil {
			return lRef{}, false, err
		}
		ps.consume("DefinedPlayer")
		return r, true, nil
	}
	if v := strings.TrimSpace(dp); lc.repeatPl > 0 && (v == "Remembered" || v == "Player.IsRemembered") {
		// the repeated player's hand (Worlds Within Worlds; gorge
		// effChangeZoneHandOwners over cc.Remembered, the iteration's
		// player): repeatEachPlayers rewrites rRemembered to RefEach
		ps.consume("DefinedPlayer")
		return lRef{Kind: rRemembered}, true, nil
	}
	if lc.repeatPl == 0 && handOwnersEachOpp(dp) {
		// "each other player may put a creature card from their hand onto
		// the battlefield" (Hunted Wumpus): the owners are the opponents
		// (gorge definedSpecOpponent: AliveFrom(controller) without the
		// controller), handMove walks them (lower_handmoveeach.go)
		ps.consume("DefinedPlayer")
		return lRef{Kind: rEachOpp}, true, nil
	}
	if strings.TrimSpace(dp) != "Targeted" {
		return lRef{}, false, refuse("hand move DefinedPlayer$ %q", dp)
	}
	// a charm's ability holds every mode's targets: only the chosen mode's
	// own slots count (the slot number stays the ability-wide one)
	slot, nTgts := -1, 0
	for i, t := range lc.ab.Targets {
		if lc.mode >= 0 && t.Mode != lc.mode {
			continue
		}
		nTgts++
		if t.F != nil && t.F.Players != 0 && len(t.F.Clauses) == 0 {
			if slot >= 0 {
				return lRef{}, false, refuse("hand move DefinedPlayer$ Targeted over two player targets")
			}
			slot = i
		}
	}
	if slot < 0 || nTgts != 1 {
		return lRef{}, false, refuse("hand move DefinedPlayer$ Targeted without one player target")
	}
	ps.consume("DefinedPlayer")
	return lRef{Kind: rTarget, N: slot}, true, nil
}

// handMoveUntilLeaves reads Duration$ UntilHostLeavesPlay on a hand exile
// (gorge zone_change.go: no move at all once the host has left
// (exileHostGoneFor); the exiled card returns to the zone it came from, the
// hand, when the host leaves (rules emit.go sweepExileReturn)). IsCurse$ is
// an AI hint and is consumed with it. Any other duration refuses.
func handMoveUntilLeaves(ps *params, dest int) (bool, error) {
	dur, ok := ps.get("Duration")
	if !ok {
		return false, nil
	}
	if strings.TrimSpace(dur) != "UntilHostLeavesPlay" || dest != zExile {
		return false, refuse("hand move Duration$ %q", dur)
	}
	ps.consume("IsCurse")
	return true, nil
}
