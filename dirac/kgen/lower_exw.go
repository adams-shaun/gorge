package kgen

import "strings"

// Defined$ ExiledWith (gorge definedSpec / exiledWithSet), the
// cards exiled with the resolving source, as a ChangeZoneAll sweep over the
// exile zones. The sweep walks seat 0's exile then seat 1's in zone order,
// which is exiledWithSet's order, and the filter is the exiled-with predicate
// with PredN 1 (a resolving trigger's fire ver counts too, as exiledWithSet
// passes it).

// exiledWithMove lowers `DB$ ChangeZone | Defined$ ExiledWith | Origin$ Exile`.
func (lc *lowerCtx) exiledWithMove(ps *params, origin, dest int, flags []string, counterKind string, override *lOverride) error {
	if origin != zExile || dest != zBattlefield && dest != zHand {
		return refuse("Defined$ ExiledWith %s->%s", zoneLabel[origin], zoneLabel[dest])
	}
	if counterKind != "" || override != nil {
		return refuse("Defined$ ExiledWith with counters/override")
	}
	for _, k := range []string{"ChangeType", "ValidTgts", "Hidden", "Mandatory", "Optional", "Duration", "Reveal", "Shuffle", "AtRandom"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Defined$ ExiledWith with %s$", k)
		}
	}
	f, err := lc.exiledWithFilter()
	if err != nil {
		return err
	}
	ps.consume("Defined")
	cs := "from=Exile"
	if len(flags) != 0 {
		cs += "|" + strings.Join(flags, "|")
	}
	lc.emit(lInstr{Op: "ChangeZoneAll", F: f, C: dest, CS: cs, Note: "Defined$ ExiledWith: the cards exiled with this source"})
	return nil
}

// exiledWithFilter builds the exiled-with set as a filter in the predicate
// form (PredN 1: a resolving trigger's fire ver counts too, as exiledWithSet
// passes it), not the flag, and marks the card as a reader so its own exile
// records the link (genPrep.exwRec; zone_exiledwith.go). Shared by the
// ChangeZone sweep and CopyPermanent's copy of the set.
func (lc *lowerCtx) exiledWithFilter() (*lFilter, error) {
	lc.exwOK, lc.exwSynth = true, true
	f, err := lc.filter("Card.ExiledWithSource")
	lc.exwOK, lc.exwSynth = false, false
	if err != nil {
		return nil, err
	}
	if f.Players != 0 || len(f.Clauses) != 1 {
		return nil, refuse("Defined$ ExiledWith: not one card filter")
	}
	f.Clauses[0].Flags &^= fExiledWith
	f.Clauses[0].CmpStat, f.Clauses[0].CmpN = cmpExiledWith, 1
	return f, nil
}

// exwCheck refuses a card that reads the link (lc.out.ExiledWith) and exiles
// through a mover whose gorge association is unproven. gorge writes the
// forward ExiledCards list only from the ChangeZone object loop, a library
// search and ChangeZoneAll; the reverse scalar only when the card's text names
// ExiledWithSource (exileProvenanceNeeded) or for a Hideaway. The hidden
// ChooseMove pick (Wormfang Newt/Turtle: `Hidden$ True`, no ExiledWithSource
// in the text) therefore records nothing in gorge and the leaves trigger
// returns nothing, which is the gorge finding in the report; a
// duration exile is unproven against the link. A Dig is the
// reverse-only association (the kernel's exwForwardOp lacks OpDig): it is
// proven only for a reader whose own text names ExiledWithSource
// (lc.out.ExwWord: Maralen, Fae Ascendant), where gorge sets the reverse
// scalar; a Dig beside an ExiledWith$Amount reader stays refused
func (lc *lowerCtx) exwCheck() {
	if !lc.out.ExiledWith {
		return
	}
	for _, a := range lc.out.Abilities {
		for _, in := range a.Prog {
			switch in.Op {
			case "ChooseMove":
				// a pick out of exile writes no link (the card is already
				// linked); a pick out of the hand into exile links it when
				// the card's SVars read the set (gorge zone_hand.go
				// svarsReadExiledWith, Forge's handleExiledWith: All Good
				// Things . . .); any other pick into exile is Wormfang's
				// unproven shape
				if in.C != zExile || strings.Contains(in.CS, "from=Hand") && lc.svarsReadExiledWith() {
					continue
				}
			case "Search":
				// gorge writes the forward entry in applyLibrarySearch (an
				// events.Imprint) and the reverse scalar in moveZoneEvent when
				// the text names the word; the kernel records OpSearchEx
				continue
			case "Exile":
				// a dies-trigger's exile of the triggering card (Promise,
				// Purgatory): with two sources of the card triggering on one
				// death the link must stay with the first. A graveyard-origin
				// exile carries MvFromGraveyard, so the kernel skips the card
				// the first source already moved (CR 400.7), as gorge does. A
				// discard trigger's (Veteran Survivor) has no such race.
				if a.Trigger == nil || a.Trigger.Event != "Dies" || in.Obj.Kind != rTriggeredNew && in.Obj.Kind != rTriggered {
					continue
				}
				if strings.Contains(in.CS, "from=Graveyard") {
					continue
				}
			case "Dig":
				// the reverse-only association (the kernel's exwForwardOp lacks
				// OpDig): proven only for a reader whose own text names
				// ExiledWithSource (lc.out.ExwWord: Maralen, Fae Ascendant),
				// where gorge sets the reverse scalar; a Dig beside an
				// ExiledWith$Amount reader stays refused
				if lc.out.ExwWord {
					continue
				}
			case "Foretell", "ExileUntilLeaves":
			default:
				continue
			}
			lc.refuseNode("card", "ExiledWith reader with a %s exile: gorge's exiled-with association for that mover is unproven", in.Op)
			return
		}
	}
}

// svarsReadExiledWith is gorge's svarsReadExiledWith over the face's SVar
// table: some SVar body names ExiledWith.
func (lc *lowerCtx) svarsReadExiledWith() bool {
	for _, body := range lc.face.SVars {
		if strings.Contains(body, "ExiledWith") {
			return true
		}
	}
	return false
}

// exilePick lowers `ChangeZone | Hidden$ True | Origin$ Exile | Destination$
// Battlefield | ChangeType$ ...` (Omenpath Journey's AtRandom$ pick, Northampton
// Farm's mandatory GainControl$ pick): gorge effHiddenPick
// over every player's exile in seat order (gameWide: no DefinedPlayer$, not a
// hand or library origin). AtRandom$ True takes the engine's seeded pick
// (randomObjIDs) and asks nobody; otherwise the controller chooses. The pick
// writes no exiled-with link (the card is already linked). Kernel OpChooseMove
// from the Exile zone (zone_exiledwithpick.go).
func (lc *lowerCtx) exilePick(ps *params, f *lFilter, n lValue, flags []string, counterKind string, override *lOverride, dest int) error {
	if counterKind != "" || override != nil {
		return refuse("exile pick with counters/override")
	}
	if dest != zBattlefield {
		return refuse("exile pick to %s", zoneLabel[dest])
	}
	for _, k := range []string{"ValidTgts", "DefinedPlayer", "Chooser", "Optional", "RememberLKI", "RememberChanged", "LibraryPosition", "Duration", "Reveal", "Shuffle", "Transformed", "Attacking", "ForgetOtherRemembered", "StaticEffect", "Defined"} {
		if _, ok := ps.m[k]; ok {
			return refuse("exile pick with %s$", k)
		}
	}
	if !n.isConst() || n.N != 1 {
		return refuse("exile pick ChangeNum$ %v", n)
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("exile pick filter %s", f)
	}
	cs := []string{"from=Exile"}
	for _, fl := range flags {
		switch fl {
		case "Tapped", "Mandatory", "UnderYourControl":
			cs = append(cs, fl)
		default:
			return refuse("exile pick flag %s", fl)
		}
	}
	if v, ok := ps.get("AtRandom"); ok {
		if v != "True" {
			return refuse("exile pick AtRandom$ %q", v)
		}
		cs = append(cs, "Random")
	}
	lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: n, F: f, C: dest, CS: strings.Join(cs, "|")})
	return nil
}
