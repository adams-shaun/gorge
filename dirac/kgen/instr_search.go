package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for library search and the chosen move (instr.go).
func init() {
	instrOp("Search", (*instrCtx).opSearch)
	instrOp("ChooseMove", (*instrCtx).opSearch)
}

func (ic *instrCtx) opSearch() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	var err error
	ki.Op = kop.OpSearchEx
	zone := kop.ZoneLibrary
	sf := 0
	if in.Op == "ChooseMove" {
		ki.Op = kop.OpChooseMove
		zone, sf = chooseMoveZone(fl["from"])
	}
	for k, b := range map[string]int{"Tapped": kop.SrTapped, "ThenTop": kop.SrThenTop, "Mandatory": kop.SrMandatory, "Confirm": kop.SrConfirm, "ChooserYou": kop.SrChooserYou, "Reveal": kop.SrReveal, "OwnZone": kop.SrOwnZone, "ForgetOther": kop.SrForgetOther, "UnderYourControl": kop.SrUnderYourControl} {
		if _, ok := fl[k]; ok {
			sf |= b
		}
	}
	if _, ok := fl["Remember"]; ok {
		// RememberChanged$: the moved cards join the remembered set
		// (gorge zone_search eventRemember / Ctx.Remembered; kernel
		// moveFound). moveFound remembers a card put onto the
		// battlefield, into a hand or graveyard, or into a library from
		// a hand (gorge handLibraryTail: the bottom move); a library
		// SEARCH's moveFound did not remember, and neither does the
		// ThenTop path: refuse those.
		_, noShuf := fl["NoShuffle"]
		handToLib := in.Op == "ChooseMove" && zone == kop.ZoneHand && zoneMap[in.C] == kop.ZoneLibrary
		if zoneMap[in.C] == kop.ZoneLibrary && !noShuf && !handToLib || sf&kop.SrThenTop != 0 {
			return fmt.Errorf("RememberChanged$ on a search into a library")
		}
		sf |= kop.SrRemember
	}
	ki.C = int16(int(zoneMap[in.C]) | sf<<3)
	// SrForgetChanged rides in B too; SrAttacking / SrDifferentNames ride in B (C>>3 is full); B.N bit 0
	// is the UntilLeaves hand-exile link
	sb := 0
	if _, ok := fl["EachMin"]; ok {
		sb |= kop.SrEachMin
	}
	if _, ok := fl["Attacking"]; ok {
		sb |= kop.SrAttacking
	}
	if _, ok := fl["DifferentNames"]; ok {
		sb |= kop.SrDifferentNames
	}
	if _, ok := fl["NoShuffle"]; ok {
		if in.Op != "Search" && zone != kop.ZoneLibrary {
			return fmt.Errorf("NoShuffle on a %s", in.Op)
		}
		sb |= kop.SrNoShuffle
	}
	if _, ok := fl["Imprint"]; ok {
		libExile := zone == kop.ZoneLibrary && zoneMap[in.C] == kop.ZoneExile
		handPut := zone == kop.ZoneHand && zoneMap[in.C] == kop.ZoneBattlefield // Eureka (lower_imprintzone.go handMoveImprint)
		if in.Op != "ChooseMove" || !libExile && !handPut {
			return fmt.Errorf("Imprint on a %s from %s", in.Op, fl["from"])
		}
		sb |= kop.SrImprint
	}
	if _, ok := fl["Random"]; ok {
		if in.Op != "ChooseMove" || zone != kop.ZoneGraveyard && zone != kop.ZoneExile {
			return fmt.Errorf("Random on a %s", in.Op)
		}
		sb |= kop.SrRandom // Tersa Lightshatter
	}
	if _, ok := fl["GameGrave"]; ok {
		// A game-wide graveyard pick (gorge effHiddenPick gameWide: no
		// ValidTgts$, no DefinedPlayer$): the pool is every player's
		// graveyard in seat order; picked cards enter their owner's zone.
		// Only a graveyard origin poses it; the pick's destination rides
		// in C. Counters never co-occur (a counter rider is refused
		// before the pick lowers).
		if in.Op != "ChooseMove" || zone != kop.ZoneGraveyard {
			return fmt.Errorf("GameGrave on a %s from %s", in.Op, fl["from"])
		}
		sb |= kop.SrGameGrave
	}
	if _, ok := fl["ChooserOpp"]; ok && in.Op == "ChooseMove" {
		// Chooser$ Opponent on a pick (Tasigur's "of an opponent's
		// choice"): the resolving controller's first living opponent
		// answers (gorge hiddenPickChooser: chooserPlayer's first of the
		// alive opponents); the kernel reads SrChooserOpp off the
		// ChooseMove's B beside its other readers. B bit 0 is the
		// UntilLeaves hand-exile link, which never co-occurs: the link
		// is a host-gone arm with no Chooser$, and a search's ChooserOpp
		// rides sideSearchFlags instead.
		if _, leaves := fl["UntilLeaves"]; leaves {
			return fmt.Errorf("ChooserOpp beside UntilLeaves")
		}
		sb |= kop.SrChooserOpp
	}
	if _, ok := fl["Bottom"]; ok {
		// LibraryPosition$ -1 on a hand move: the picked cards go to the
		// bottom of the library (gorge handLibraryTail's
		// libraryOrderPlacement bottom=true). Only a hand-to-library
		// ChooseMove carries it.
		if in.Op != "ChooseMove" || zone != kop.ZoneHand || zoneMap[in.C] != kop.ZoneLibrary {
			return fmt.Errorf("Bottom on a %s from %s", in.Op, fl["from"])
		}
		sb |= kop.SrHandBottom
	}
	if _, ok := fl["PosAsk"]; ok {
		// the controller's top-or-bottom ask after a hand put-back
		// (Dream Cache; lower_destalt.go)
		if in.Op != "ChooseMove" || zone != kop.ZoneHand || zoneMap[in.C] != kop.ZoneLibrary {
			return fmt.Errorf("PosAsk on a %s from %s", in.Op, fl["from"])
		}
		sb |= kop.SrHandPosAsk
	}
	if _, ok := fl["DestAltGrave"]; ok {
		// the searcher's hand-or-graveyard ask after the pick (Dina's
		// Guidance; lower_destalt.go)
		if in.Op != "Search" || zoneMap[in.C] != kop.ZoneHand {
			return fmt.Errorf("DestAltGrave on a %s", in.Op)
		}
		sb |= kop.SrDestAltGrave
	}
	if _, ok := fl["ShareLandType"]; ok {
		if in.Op != "Search" {
			return fmt.Errorf("ShareLandType on a %s", in.Op)
		}
		sb |= kop.SrShareLandType
	}
	if _, ok := fl["ForgetChanged"]; ok {
		if in.Op != "Search" {
			return fmt.Errorf("ForgetChanged on a %s", in.Op)
		}
		sb |= kop.SrForgetChanged
	}
	if az, ok := fl["alt"]; ok {
		// OriginAlternative$: the library
		// search's zone election; the alternative zone rides B bits 5..7
		_, altAll := fl["altAll"]
		_, noShuf := fl["NoShuffle"]
		libPut := altAll && noShuf && zoneMap[in.C] == kop.ZoneLibrary // Doomsday (lower_libgrave.go)
		if in.Op != "Search" || zoneMap[in.C] != kop.ZoneHand && zoneMap[in.C] != kop.ZoneBattlefield && !libPut {
			return fmt.Errorf("alt origin on a %s to zone %d", in.Op, zoneMap[in.C])
		}
		var altZ kop.Zone
		switch az {
		case "Graveyard":
			altZ = kop.ZoneGraveyard
		case "Hand":
			altZ = kop.ZoneHand
		case "Graveyard,Hand":
			// the pair occupies the zone code the library can never use:
			// the election offers the library, graveyard and hand.
			altZ = kop.Zone(kop.SrAltGraveHand)
		default:
			return fmt.Errorf("alt origin %q", az)
		}
		sb |= kop.SrAltOrigin | int(altZ)<<kop.SrAltZoneShift
		if _, ok := fl["altAll"]; ok {
			// Origin$ Library,Graveyard (lower_libgrave.go): every zone, no election
			if altZ != kop.ZoneGraveyard {
				return fmt.Errorf("altAll with alt origin %q", az)
			}
			sb |= kop.SrAltAll
		}
		zone = kop.ZoneNone // the filter matches cards of every elected zone
	}
	if sb, zone, err = sideSearchFlags(in.Op, fl, sb, zone); err != nil {
		return err
	}
	if _, ok := fl["AuraEntry"]; ok {
		// the hand move of one card whose filter may
		// admit an Aura (CR 303.4f/g, kernel zone_handmoveaura.go)
		if in.Op != "ChooseMove" || zone != kop.ZoneHand || zoneMap[in.C] != kop.ZoneBattlefield {
			return fmt.Errorf("AuraEntry on a %s", in.Op)
		}
		sb |= kop.SrAuraEntry
	}
	if _, ok := fl["AttachTo"]; ok {
		// AttachedTo$ on a library search
		// (lower_zone.go's search path): the found card enters attached
		// to the bearer the W Ref names (kop.SrSearchAttach); the named
		// bearer replaces the plain CR 303.4f enchant ask
		if in.Op != "Search" || zoneMap[in.C] != kop.ZoneBattlefield {
			return fmt.Errorf("AttachTo on a %s to zone %d", in.Op, zoneMap[in.C])
		}
		var r kop.Ref
		if r, err = cb.ref(in.Obj2); err != nil {
			return err
		}
		if r > 7 {
			return fmt.Errorf("attach bearer ref %d", r)
		}
		sb |= kop.SrSearchAttach
		ki.W = r
	}
	if _, ok := fl["UntilLeaves"]; ok {
		var bit int
		bit, err = untilLeavesBit(in, zone)
		if err != nil {
			return err
		}
		sb |= bit
	}
	if in.Op == "ChooseMove" && fl["from"] == "Hand,Graveyard" {
		for _, bad := range []string{"Random", "UntilLeaves", "NoShuffle", "ForgetChanged"} {
			if _, ok := fl[bad]; ok {
				return fmt.Errorf("%s on a Hand,Graveyard pick", bad)
			}
		}
		sb |= kop.SrHandGrave
		if k, ok := fl["counters"]; ok {
			ck, err := counterKind(k)
			if err != nil {
				return err
			}
			if int(ck)+1 > 0xFF {
				return fmt.Errorf("Hand,Graveyard pick counter kind %d", ck)
			}
			sb |= (int(ck) + 1) << 4
		}
	}
	if in.Op == "ChooseMove" && fl["from"] == "Graveyard" && fl["counters"] != "" {
		// WithCountersType$ on a plain graveyard pick
		// (Scavenger's Talent): the kind rides B bits 4..11, the same
		// field as Hand,Graveyard (SrHandGrave stays clear; bit 2
		// distinguishes the two pool shapes). The kernel arms exactly
		// one entry counter (handGraveArm).
		for _, bad := range []string{"Random", "UntilLeaves", "ForgetChanged"} {
			if _, ok := fl[bad]; ok {
				return fmt.Errorf("%s on a Graveyard pick", bad)
			}
		}
		ck, err := counterKind(fl["counters"])
		if err != nil {
			return err
		}
		if int(ck)+1 > 0xFF {
			return fmt.Errorf("Graveyard pick counter kind %d", ck)
		}
		sb |= (int(ck) + 1) << 4
	}
	if sb != 0 {
		ki.B = kop.Const(sb)
	}
	if err := ic.objFilter(zone); err != nil {
		return err
	}
	return nil
}

// untilLeavesBit is the ChooseMove B bit of Duration$ UntilHostLeavesPlay: on a
// hand exile (Deep-Cavern Bat) B.N bit 0, the kernel's hand-return link; on a
// battlefield exile pick (Glorious Protector) kop.SrBattleLink, the link that
// returns the picked set to the battlefield. Any other pick refuses.
func untilLeavesBit(in *lInstr, zone kop.Zone) (int, error) {
	if in.Op != "ChooseMove" || zoneMap[in.C] != kop.ZoneExile || zone != kop.ZoneHand && zone != kop.ZoneBattlefield {
		return 0, fmt.Errorf("UntilLeaves on a %s", in.Op)
	}
	if zone == kop.ZoneHand {
		return 1, nil
	}
	return kop.SrBattleLink, nil
}

// chooseMoveZone is a ChooseMove's From$ zone (the graveyard by default) and
// the search flags it carries (opSearch).
func chooseMoveZone(from string) (zone kop.Zone, sf int) {
	zone, sf = kop.ZoneGraveyard, 0
	switch from {
	case "Hand":
		zone = kop.ZoneHand
	case "Battlefield":
		zone = kop.ZoneBattlefield
	case "Exile":
		zone = kop.ZoneExile // every seat's exile (zone_exiledwithpick.go)
	case "Command":
		zone = kop.ZoneCommand // Command Beacon's pick (dirac/zone_cmdpick.go)
	case "Library":
		zone = kop.ZoneLibrary // a hidden pick, not a search (Sanar's per-colour picks)
	case "Graveyard,Exile":
		// graveexile: a game-wide pool over both zones; the walk in
		// dirac/value_tappower.go restricts the zones itself.
		zone = kop.ZoneNone
		sf |= kop.SrGraveExile
	case "Hand,Graveyard":
		// the fetching player's hand then
		// graveyard; SrHandGrave rides in B (below)
		zone = kop.ZoneNone
	}
	return zone, sf
}
