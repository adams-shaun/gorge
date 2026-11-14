package kgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// changeZoneAllShuffle lowers ChangeZoneAll Shuffle$ True (gorge
// effChangeZoneAll's changeZoneAllShuffleLibraries: after the placement,
// each destination library that received a card shuffles, then every
// library of an EXPLICIT player scope that received nothing; a sweep into
// any zone but the library ignores the flag). Lowered for a graveyard ->
// library sweep of one owner: the ValidTgts$ Player form (the target's
// graveyard into the target's library: one Shuffle of the target, even
// when nothing moved) and the unscoped form whose ChangeType$ names only
// your own cards (Card.YouOwn / YouCtrl: your graveyard; the shuffle is
// skipped when nothing matches, as gorge shuffles only receiving
// libraries). Every other Shuffle$ shape refuses.
func (lc *lowerCtx) changeZoneAllShuffle(ps *params, in lInstr, origin, dest int) error {
	if ps.is("Shuffle", "False") {
		lc.emit(in) // gorge cza.ShuffleTrue is false: no shuffle
		return nil
	}
	if !ps.is("Shuffle", "True") {
		return refuse("ChangeZoneAll Shuffle$ %q", ps.str("Shuffle"))
	}
	if dest != zLibrary {
		lc.emit(in) // gorge: the flag only acts on library destinations
		return nil
	}
	if _, ok := ps.get("LibraryPosition"); ok {
		return refuse("ChangeZoneAll Shuffle$ with LibraryPosition$")
	}
	if origin == zBattlefield {
		// Teferi, Temporal Pilgrim's "shuffle each nonland permanent they
		// control into its owner's library": the target player's permanents
		// (ChangeType$ ...TargetedPlayerCtrl), to their owners' libraries,
		// each receiving library shuffled (ExtLibSweep)
		if in.Obj.Kind != rNone || in.F == nil || len(in.F.Clauses) == 0 || in.F.Players != 0 || in.Note != "" {
			return refuse("ChangeZoneAll Shuffle$ of %s from the battlefield", in.Obj)
		}
		for _, c := range in.F.Clauses {
			if c.Ctrl != whoTgtPlayer {
				return refuse("ChangeZoneAll Shuffle$ from the battlefield beyond the target player's permanents")
			}
		}
		lc.emit(lInstr{Op: "LibSweep", F: in.F})
		return nil
	}
	if origin != zGraveyard {
		return refuse("ChangeZoneAll Shuffle$ from %s", zoneLabel[origin])
	}
	if in.Obj.Kind == rTarget {
		lc.emit(in)
		lc.emit(lInstr{Op: "Shuffle", Obj: in.Obj})
		return nil
	}
	if in.Obj.Kind != rNone || in.F == nil || len(in.F.Clauses) == 0 || in.F.Players != 0 {
		return refuse("ChangeZoneAll Shuffle$ of %s", in.Obj)
	}
	for _, c := range in.F.Clauses {
		if c.Owner != whoYou && c.Ctrl != whoYou {
			return refuse("ChangeZoneAll Shuffle$ over several owners' graveyards")
		}
	}
	if in.Note == noteAllOrigin {
		in.Note = ""
		count := lCond{Kind: cValue, A: lValue{Kind: vCountGraveyard, F: in.F}, Op: opGE, B: cst(1)}
		lc.czallAllShuffle(in, count) // UseAllOriginZones$ (lower_changezoneall.go)
		return nil
	}
	lc.emit(in) // gorge sweepsYourOwn: you shuffle whatever the graveyard holds
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rYou}})
	return nil
}

// changeZoneAllRiders reads a ChangeZoneAll's entry riders onto in: the
// counters every entrant carries (WithCountersType$/WithCountersAmount$,
// CR 122.6) and the per-entrant StaticEffect$ (CR 603.6b), both applied by
// gorge inside the move loop before the entrants' enters checks fire. They
// are folded into the sweep instruction (kop.MvEntryRiders), not emitted as
// a separate OpOverride: that targets one object and would run after the
// sweep. (Ghost Vacuum).
func (lc *lowerCtx) changeZoneAllRiders(ps *params, in *lInstr, origin, dest int, dual bool) error {
	ct, hasCt := ps.get("WithCountersType")
	se, hasSe := ps.get("StaticEffect")
	if !hasCt && !hasSe {
		return nil
	}
	if dest != zBattlefield {
		return refuse("ChangeZoneAll entry riders onto %s", zoneLabel[dest])
	}
	if dual {
		return refuse("ChangeZoneAll entry riders with Origin$ Graveyard,Exile")
	}
	if hasCt && strings.Contains(ct, ",") {
		return refuse("ChangeZoneAll WithCountersType$ %q: a list of counter kinds", ct)
	}
	var n lValue
	var err error
	if hasCt {
		if n, err = lc.amount(ps, "WithCountersAmount", 1); err != nil {
			return err
		}
		if !n.isConst() || n.N < 1 || n.N > 255 {
			return refuse("ChangeZoneAll WithCountersAmount$ %q: only a constant 1..255 is read", ps.str("WithCountersAmount"))
		}
	}
	var o *lOverride
	if hasSe {
		// read before the origin check so a StaticEffect$ the override
		// lowering cannot read (a basic land type, an unknown type) names that
		if o, err = lc.staticEffectOverride(se); err != nil {
			return err
		}
	}
	if origin != zExile && origin != zGraveyard {
		// the lockstep pool (exw) proves the rider path for a sweep out of
		// exile (Ghost Vacuum) and out of a graveyard (Storm of Souls)
		return refuse("ChangeZoneAll entry riders from %s: only Exile and Graveyard sweeps are lockstep-checked", zoneLabel[origin])
	}
	if hasCt {
		in.A = n
		in.CS += "|counters=" + ct
	}
	if o != nil {
		lc.out.overrides = append(lc.out.overrides, o)
		in.CS += "|static=" + strconv.Itoa(len(lc.out.overrides)-1)
		in.Note = strings.TrimSpace(in.Note + " each entrant: " + overrideString(o))
	}
	return nil
}

// playerTargetRange reads TargetMin$/TargetMax$ on a ChangeZoneAll over
// target players. Each targeted player's zone moves in target order (the
// kernel runs the op once per target of the slot). A TargetMin$ of 0 moves
// nothing with none targeted (Thraben Charm; gorge swept every player's zone
// before cr-changezoneall-zero-targets, 56ba2e185).
func (lc *lowerCtx) playerTargetRange(ps *params, lt *lTarget) error {
	for k, dst := range map[string]*lValue{"TargetMin": &lt.Min, "TargetMax": &lt.Max} {
		v, ok := ps.get(k)
		if !ok {
			continue
		}
		x, err := lc.value(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		*dst = x
	}
	return nil
}

// noteAllOrigin marks a ChangeZoneAll instruction that read UseAllOriginZones$
// True; changeZoneAllShuffle (lower_effectemblem.go) turns it into the gorge shuffle
// order (czallAllShuffle).
const noteAllOrigin = "use all origin zones"

// czallParams3 reads the three ChangeZoneAll keys lowers.
// Every other value or combination refuses by name.
//
//   - UseAllOriginZones$ True (Finale of Revelation; gorge changeZoneAllPlayers
//     scopes the sweep to every alive player): only beside Shuffle$ True,
//     graveyard to library, no target player. The shuffle order is item
//     czallAllShuffle's.
//   - Duration$ UntilHostLeavesPlay (Pinnacle Starcage): a battlefield to
//     exile sweep run by a permanent's ability; each exiled non-token gets an
//     until-host-leaves link (kernel czall3Leave, "UntilLeaves" -> OpChangeZoneAllEx
//     B bit kop.CzUntilLeaves).
//   - ForgetOtherRemembered$ True beside RememberChanged$ True (Random
//     Encounter): the matched set is snapshotted, the remembered set cleared,
//     then the snapshot moves ("ForgetOther" -> kop.MvForgetOther). Kept to
//     the non-battlefield, non-library sweeps the kernel's cold walk copies.
func (lc *lowerCtx) czallParams3(ps *params, in *lInstr, origin, dest int) error {
	if v, ok := ps.get("UseAllOriginZones"); ok {
		switch {
		case v != "True":
			return refuse("ChangeZoneAll UseAllOriginZones$ %q", v)
		case !ps.is("Shuffle", "True"):
			return refuse("ChangeZoneAll UseAllOriginZones$ without Shuffle$ True")
		case origin != zGraveyard || dest != zLibrary:
			return refuse("ChangeZoneAll UseAllOriginZones$ beyond graveyard to library")
		case in.Obj.Kind != rNone:
			return refuse("ChangeZoneAll UseAllOriginZones$ with a target player")
		}
		in.Note = noteAllOrigin
	}
	if v, ok := ps.get("ForgetOtherRemembered"); ok {
		switch {
		case v != "True":
			return refuse("ChangeZoneAll ForgetOtherRemembered$ %q", v)
		case !ps.is("RememberChanged", "True"):
			return refuse("ChangeZoneAll ForgetOtherRemembered$ without RememberChanged$ True")
		case origin == zBattlefield:
			return refuse("ChangeZoneAll ForgetOtherRemembered$ from the battlefield")
		case dest == zLibrary:
			return refuse("ChangeZoneAll ForgetOtherRemembered$ to the library")
		case in.Obj.Kind != rNone:
			return refuse("ChangeZoneAll ForgetOtherRemembered$ with a target player")
		case ps.m["Shuffle"] != "" || ps.m["LibraryPosition"] != "":
			return refuse("ChangeZoneAll ForgetOtherRemembered$ with Shuffle$")
		}
		in.CS += "|ForgetOther"
		lc.czForgetSet = true
	}
	if v, ok := ps.get("Duration"); ok {
		switch {
		case v != "UntilHostLeavesPlay":
			return refuse("ChangeZoneAll Duration$ %q", v)
		case origin != zBattlefield || dest != zExile:
			return refuse("ChangeZoneAll Duration$ %s beyond battlefield to exile", v)
		case !lc.face.IsPermanent():
			return refuse("ChangeZoneAll Duration$ %s on a spell", v)
		case in.Obj.Kind != rNone:
			return refuse("ChangeZoneAll Duration$ %s with a target player", v)
		case strings.Contains(in.CS, "|Remember") || ps.m["Shuffle"] != "":
			return refuse("ChangeZoneAll Duration$ %s with RememberChanged$ or Shuffle$", v)
		}
		in.CS += "|UntilLeaves"
	}
	return nil
}

// czallAllShuffle emits a graveyard-to-library sweep of your own cards that
// reads UseAllOriginZones$ (gorge changeZoneAllShuffleLibraries, scoped):
// the receiving library (yours, only when a card moved) is shuffled first,
// then every scoped player not yet shuffled, in seat order:
//
//	moved:   sweep; Shuffle You; Shuffle Opponent   (you, then the one not yet shuffled)
//	nothing: Shuffle each player in seat order       (both, seat 0 first)
//
// The second arm runs behind an always-false SkipIf that jumps the first.
func (lc *lowerCtx) czallAllShuffle(in lInstr, count lCond) {
	lc.emit(lInstr{Op: "SkipIf", Cond: count, Skip: 4,
		Note: "gorge shuffles the receiving library first, then each scoped player not yet shuffled"})
	lc.emit(in)
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rYou}})
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rEachOpp}})
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: cst(0), Op: opGE, B: cst(1)}, Skip: 2, Note: "jump over the nothing-moved arm"})
	lc.emit(lInstr{Op: "RepeatEach", Obj: lRef{Kind: rEachPlayer}, C: 1 | kop.RepeatEachSeatOrder, Skip: 1})
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rEach}})
}

// czallBuild3 checks the built OpChangeZoneAllEx against what the kernel's
// cold walk (kernel czall3) copies and sets Duration$'s B bit. Entry riders
// (counters, StaticEffect$, AtEOT$) refuse: the cold walk does not apply them.
func czallBuild3(ki *kop.Instr, dest kop.Zone, forget, untilLeaves bool) error {
	if ki.C>>3&kop.MvEntryRiders != 0 {
		return fmt.Errorf("ChangeZoneAll ForgetOtherRemembered$/Duration$ with entry riders")
	}
	if ki.C>>3&kop.MvGraveExile != 0 && untilLeaves {
		return fmt.Errorf("ChangeZoneAll Duration$ from Graveyard,Exile")
	}
	if untilLeaves {
		if dest != kop.ZoneExile {
			return fmt.Errorf("ChangeZoneAll Duration$ to zone %d", dest)
		}
		ki.B = kop.Const(kop.CzUntilLeaves)
	}
	return nil
}

// originGraveHand admits two Graveyard/Hand shapes into the Library, each
// with Shuffle$ True and no other rules key; every other one refuses by name.
//
//   - Origin$ Graveyard,Hand, ChangeType$ Card.YouOwn (Weftwalking, Midnight
//     Clock): sweeps your graveyard then your hand into your library
//     (kop.MvGraveExile with the second zone Hand in B) and shuffles you.
//   - Origin$ Hand,Graveyard or Graveyard,Hand, ChangeType$ Card with
//     UseAllOriginZones$ True (Echo of Eons): every player's cards, the hands
//     first for Hand,Graveyard (kop.CzHandGrave: the pre-shuffle library order
//     follows the move order). The kernel shuffles both libraries itself, in
//     gorge's order (kop.CzShuffleEach), so no Shuffle is emitted.
func (lc *lowerCtx) originGraveHand(ps *params) error {
	origin := strings.TrimSpace(ps.str("Origin"))
	for _, k := range sortedParamKeys(ps.m) {
		switch k {
		case "Origin", "Destination", "ChangeType", "Shuffle", "SubAbility", "UseAllOriginZones":
		case "Cost", "Planeswalker":
			// an activated ability's cost and loyalty mark are the ability's,
			// lowered with it (Arzakon's loyalty cost beside the Echo of Eons sweep)
		default:
			if !ignoredParams[k] {
				return refuse("ChangeZoneAll Origin$ %s with %s$", origin, k)
			}
		}
	}
	if ps.str("Destination") != "Library" || !ps.is("Shuffle", "True") {
		return refuse("ChangeZoneAll Origin$ %s outside Destination$ Library, Shuffle$ True", origin)
	}
	if v, ok := ps.get("UseAllOriginZones"); ok {
		if v != "True" || ps.str("ChangeType") != "Card" {
			return refuse("ChangeZoneAll Origin$ %s UseAllOriginZones$ %q outside ChangeType$ Card", origin, v)
		}
		f, err := lc.filter("Card")
		if err != nil {
			return err
		}
		if len(f.Clauses) == 0 || f.Players != 0 {
			return refuse("ChangeZoneAll Origin$ %s filter shape", origin)
		}
		lc.emit(lInstr{Op: "ChangeZoneAll", F: f, C: zLibrary, CS: "from=" + origin + "|ShuffleEach"})
		return nil
	}
	if origin != "Graveyard,Hand" || ps.str("ChangeType") != "Card.YouOwn" {
		return refuse("ChangeZoneAll Origin$ %s outside Graveyard,Hand, ChangeType$ Card.YouOwn", origin)
	}
	f, err := lc.filter("Card.YouOwn")
	if err != nil {
		return err
	}
	if len(f.Clauses) == 0 || f.Players != 0 {
		return refuse("ChangeZoneAll Origin$ Graveyard,Hand filter shape")
	}
	for _, c := range f.Clauses {
		if c.Owner != whoYou && c.Ctrl != whoYou {
			return refuse("ChangeZoneAll Origin$ Graveyard,Hand over several owners")
		}
	}
	lc.emit(lInstr{Op: "ChangeZoneAll", F: f, C: zLibrary, CS: "from=Graveyard,Hand"})
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rYou}})
	return nil
}

// originBattleGrave admits ONLY Origin$ Battlefield,Graveyard, Destination$
// Exile, DefinedPlayer$ Targeted, and a ChangeType$ the kernel can scan both
// zones with (Strategic Betrayal). Every owner/controller role clause must
// carry the graveyard zone pin: the build lowers the filter with
// kop.ZoneNone, so ownerRoleF2 no longer refuses the battlefield role there,
// and the pin is what keeps F2TgtPlayerCtrl from reading a permanent's
// controller during the battlefield scan. No other rules key (RememberChanged$,
// ForgetOtherRemembered$, Duration$, Shuffle$, AtEOT$, entry riders, ValidTgts$
// or a Permanent base) has a carrier here.
func (lc *lowerCtx) originBattleGrave(ps *params) error {
	for _, k := range sortedParamKeys(ps.m) {
		switch k {
		case "Origin", "Destination", "ChangeType", "DefinedPlayer", "SubAbility":
		default:
			if !ignoredParams[k] {
				return refuse("ChangeZoneAll Origin$ Battlefield,Graveyard with %s$", k)
			}
		}
	}
	dest, err := zoneOf(ps, "Destination", zNone)
	if err != nil {
		return err
	}
	if dest != zExile {
		return refuse("ChangeZoneAll Origin$ Battlefield,Graveyard to %s", zoneLabel[dest])
	}
	if dp := ps.str("DefinedPlayer"); dp != "Targeted" {
		return refuse("ChangeZoneAll Origin$ Battlefield,Graveyard DefinedPlayer$ %s", dp)
	}
	// gorge's ChangeZoneAll has no DefinedPlayer$ reader (not in
	// changeZoneAllKnownKeys): it sweeps every player and the filter confines
	// the sweep. The kernel instruction keeps Obj = none, meaning every seat.
	ps.consume("DefinedPlayer")
	lc.exwOK = true // a ChangeZoneAll sweep is a resolution-time mover
	f, err := lc.filter(ps.str("ChangeType"))
	lc.exwOK = false
	if err != nil {
		return err
	}
	if len(f.Clauses) == 0 || f.Players != 0 {
		return refuse("ChangeZoneAll Origin$ Battlefield,Graveyard filter shape")
	}
	for _, c := range f.Clauses {
		if c.BasePermanent {
			return refuse("ChangeZoneAll Origin$ Battlefield,Graveyard over Permanent")
		}
		if c.Owner != whoTgtPlayer && c.Owner != whoDefending && c.Owner != whoEachPl && c.Owner != whoHitBySrc &&
			c.Ctrl != whoTgtPlayer && c.Ctrl != whoDefending && c.Ctrl != whoEachPl && c.Ctrl != whoHitBySrc {
			continue
		}
		if c.CmpStat != cmpWord7 || c.CmpN != int(kop.W7InZone)|int(kop.ZoneGraveyard)<<8 {
			return refuse("ChangeZoneAll Origin$ Battlefield,Graveyard role clause without inZoneGraveyard")
		}
	}
	lc.emit(lInstr{Op: "ChangeZoneAll", F: f, C: zExile, CS: "from=Battlefield,Graveyard"})
	return nil
}
