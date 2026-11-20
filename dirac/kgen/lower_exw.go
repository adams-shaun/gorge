package kgen

import "strings"

// Defined$ ExiledWith (gorge definedSpec / exiledWithSet), the
// cards exiled with the resolving source, as a ChangeZoneAll sweep over the
// exile zones. The sweep walks seat 0's exile then seat 1's in zone order,
// which is exiledWithSet's order, and the filter is the exiled-with predicate
// with PredN 1 (a resolving trigger's fire ver counts too, as exiledWithSet
// passes it).

// exiledWithMove lowers a ChangeZone that moves the cards exiled with the source out of exile.
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
				if in.C != zExile || strings.Contains(in.CS, "from=Hand") && (lc.svarsReadExiledWith() || lc.imprintMox) {
					// (the imprint mox reads the reverse record alone, which
					// gorge sets because its text names ExiledWithSource)
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

// exwOwnerCheck refuses a card that lowered the RefExiledWithOwner player
// ref (DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1,
// lower_dpsearch.go) whose exile can hold more than one linked card. gorge's
// shared player filter matches EVERY seat whose exile holds a card exiled
// with the source and acts on each (playerHasCardsInZoneMatches over
// definedPlayerIDs); the kernel's RefExiledWithOwner is a scalar that asks
// only the first seat in seat order (zone_exiledwith.go exiledWithOwnerSeat).
// The two agree only while at most one seat can match -- the card's program
// holds one exile mover, bound to one card. A second mover (two resolutions
// can stack two linked cards into two seats) or a mover whose per-resolution
// count is not bounded to one (a two-target ETB exile) refuses the card; the
// one-card shape, Unidentified Hovership's TargetMax$ 1 ETB, still lowers.
func (lc *lowerCtx) exwOwnerCheck() {
	if !lc.exwOwnerRef {
		return
	}
	movers := 0
	for _, ab := range lc.out.Abilities {
		for i := range ab.Prog {
			ok, err := exwOwnerMover(&ab.Prog[i], ab)
			if err != nil {
				lc.refuseNode("card", "%s", err.Error())
				return
			}
			if ok {
				movers++
			}
		}
	}
	if movers > 1 {
		lc.refuseNode("card", "ExiledWithSource player ref over %d exile movers: two linked cards can sit in two seats' exiles, where gorge acts on every matching seat and the scalar ref asks only the first", movers)
	}
}

// exwOwnerMover classifies one instruction for exwOwnerCheck: ok when it is
// an exile mover gorge's link recording follows (the exwForwardOp movers,
// zone_exiledwith.go), err when it can move more than one card into exile in
// one resolution. The count is the mover's target slot or count operand.
func exwOwnerMover(in *lInstr, ab *lAbility) (bool, error) {
	switch in.Op {
	case "Exile", "ExileUntilLeaves":
	case "Search":
		if !exwCountAtMostOne(in.A) {
			return false, refuse("ExiledWithSource player ref over a Search of %s", in.A)
		}
	case "Dig":
		if !exwCountAtMostOne(in.A) {
			return false, refuse("ExiledWithSource player ref over a Dig of %s", in.A)
		}
	case "ChooseMove":
		if in.C != zExile {
			return false, nil
		}
		if !exwCountAtMostOne(in.A) {
			return false, refuse("ExiledWithSource player ref over a ChooseMove of %s", in.A)
		}
	case "ChangeZoneAll":
		if in.C != zExile {
			return false, nil
		}
		return false, refuse("ExiledWithSource player ref over a ChangeZoneAll to exile: its count is unbounded")
	default:
		return false, nil
	}
	switch in.Obj.Kind {
	case rTarget:
		if in.Obj.N >= len(ab.Targets) {
			return false, refuse("ExiledWithSource player ref over an exile of missing target slot %d", in.Obj.N)
		}
		t := ab.Targets[in.Obj.N]
		if !exwCountAtMostOne(t.Min) || !exwCountAtMostOne(t.Max) {
			return false, refuse("ExiledWithSource player ref over an exile target slot of %s..%s", t.Min, t.Max)
		}
	case rSelf, rTriggered, rTriggeredNew:
	default:
		return false, refuse("ExiledWithSource player ref over an exile of %s", in.Obj)
	}
	return true, nil
}

// exwCountAtMostOne is an operand or target bound of at most one card (an
// omitted one is the default single pick).
func exwCountAtMostOne(v lValue) bool {
	return v.Kind == vNone || v.isConst() && v.N <= 1
}
