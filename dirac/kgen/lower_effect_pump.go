package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"strconv"
	"strings"
)

func (lc *lowerCtx) pump(ps *params) error {
	zone := zBattlefield
	if z, ok := ps.get("PumpZone"); ok {
		zone = zoneNames[z]
	}
	obj, err := lc.objOrTarget(ps, "Self", zone)
	if err != nil {
		return err
	}
	if kwc, ok := ps.get("KWChoice"); ok {
		return lc.pumpChoice(ps, obj, zone, kwc)
	}
	if _, ok := ps.get("ReplaceDyingDefined"); ok {
		// the mark follows the pump's own events (lower_exileexmark.go); a
		// Duration$ grant is not a pump of the target's characteristics
		if _, d := ps.get("Duration"); d || zone != zBattlefield {
			return refuse("ReplaceDyingDefined$ beside Duration$ or PumpZone$")
		}
	}
	if err := lc.pumpBody(ps, obj, zone, nil); err != nil {
		return err
	}
	return lc.replaceDyingMark(ps, obj, lRef{})
}

// pumpChoice lowers a Pump with KWChoice$ ("gains your choice of X or Y":
// gorge pump.go asks the pump's controller KModes first, then registers the
// pump with the chosen keyword). OpChooseBranch's chooser is the
// controller, not the pumped target; each branch is one candidate's full
// pump body. A fixed KW$ beside KWChoice$ and a duration other than the
// end-of-turn default refuse (the kernel's grant is until end of turn).
func (lc *lowerCtx) pumpChoice(ps *params, obj lRef, zone int, kwc string) error {
	if _, ok := ps.get("KW"); ok {
		return refuse("Pump with KW$ and KWChoice$")
	}
	if d, ok := ps.get("Duration"); ok && d != "" && d != "EndOfTurn" {
		return refuse("Pump KWChoice Duration %q", d)
	}
	choices := strings.Split(kwc, ",")
	if len(choices) < 1 {
		return refuse("Pump KWChoice with %d choices", len(choices))
	}
	kws := make([][]string, len(choices))
	for i, name := range choices {
		ks, err := pumpKeywords(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		kws[i] = ks
	}
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rYou}, A: cst(len(choices))})
	for i, ks := range kws {
		bat := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: i})
		if err := lc.pumpBody(ps, obj, zone, ks); err != nil {
			return err
		}
		lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
	}
	return nil
}

// pumpObjectRef reports whether a Pump's subject is a battlefield object
// rather than a player. The keyword override record applies to a permanent;
// a player-scoped grant (`Override obj=You`, The One Ring and The Stasis
// Coffin's "you gain protection from everything") has no kernel record and
// must refuse instead of lowering to a no-op. A target slot must be an
// object filter; a multi-target union is not a single override subject.
func (lc *lowerCtx) pumpObjectRef(obj lRef) bool {
	switch obj.Kind {
	case rSelf, rTriggered, rTriggeredNew, rTriggeredSource, rEquipped, rRemembered,
		rDelayedRemembered, rReplaced, rChosen, rEach, rSacrificed, rHost, rDiscarded:
		return true
	case rTarget:
		t := lc.ab.Targets[obj.N]
		return t.F != nil && t.F.Players == 0 && len(t.F.Clauses) != 0
	}
	return false
}

// pumpPlayerRef reports whether a Pump's subject is the resolving
// controller: the one player seat a player grant can name exactly, because
// the kernel's grant emblem is created for that seat (OpCreateEmblem Obj
// rYou) and gorge registers the same grant as a Player.You static
// (registerPlayerPumpGrant). A target player slot or another player
// selector cannot own the emblem -- the player grammar has no absolute seat
// spelling -- so it refuses by name (Eon Frolicker, Noble Heritage).
func (lc *lowerCtx) pumpPlayerRef(obj lRef) bool {
	return obj.Kind == rYou
}

// pumpPlayerGrant lowers a player-scoped Pump keyword grant (The One Ring,
// The Stasis Coffin, Teferi's Protection's "you gain protection from
// everything") as a runtime emblem of the resolving controller, the same
// instruction a Duration$ UntilYourNextTurn Effect static lowers to
// (lower_effectemblem.go): the UntilNextTurn emblem ends at the cleanup
// before the controller's next turn, gorge's UntilTurn = next - 1 boundary,
// and static_plprot.go's plProtAll damage and targeting readers judge the
// emblem's statics. Any other duration, a P/T part, a zone scope or a
// keyword the kernel has no emblem player read for refuses by name.
func (lc *lowerCtx) pumpPlayerGrant(ps *params, obj lRef, zone int, d string, na, nd bool, kw string, hasKW bool) error {
	if na || nd {
		return refuse("player-scoped Pump beside NumAtt$/NumDef$")
	}
	if !hasKW {
		return refuse("player-scoped Pump without KW$")
	}
	if zone != zBattlefield {
		return refuse("PumpZone$ beside a player grant")
	}
	if d != "UntilYourNextTurn" {
		return refuse("player-scoped Pump grant Duration %q", d)
	}
	sts, err := lc.playerGrantStatics(kw)
	if err != nil {
		return err
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Statics: sts, UntilNextTurn: true})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return nil
}

// playerGrantStatics lowers the keywords of a player-scoped Pump grant to
// the emblem player statics the kernel reads: "Protection from everything"
// to StatPlayerProtAll (CR 702.16j; static_plprot.go's plProtAll walks the
// player's emblems), and "Hexproof" (Blossoming Calm, CR 702.11c) to a plain
// StatPlayerHexproof with no Amount (A == 0: hexproof alone; A == 1 would add
// protection from each opponent), which playerHexproof reads off the
// player's emblems (static_emblemphex.go's embPlHex). Anything else refuses.
// A shroud has no symmetric kernel
// player read (playerHexproof keeps hexproof's own-controller exemption, CR
// 702.11c, which shroud must not: CR 702.18a), and a Protection:<spec>
// quality (Eon Frolicker's Player.PlayerUID_ChosenPlayerUID) names a player
// the grant is not for.
func (lc *lowerCtx) playerGrantStatics(kw string) ([]lStatic, error) {
	var sts []lStatic
	for _, k := range strings.Split(kw, "&") {
		switch k = strings.TrimSpace(k); k {
		case "Protection from everything":
			sts = append(sts, lStatic{Kind: "PlayerProtAll"})
		case "Hexproof":
			sts = append(sts, lStatic{Kind: "PlayerHexproof"})
		default:
			return nil, refuse("player-scoped Pump grant keyword %q", k)
		}
	}
	return sts, nil
}

// pumpPlayerEOT lowers the keywords of an until-end-of-turn Pump grant to the
// resolving controller (The Needs of the Many, Earthshape: "you gain
// hexproof until end of turn"). gorge registers the grant as a Player.You
// keyword static (registerPlayerPumpGrant) that its playerKeywords gates
// read, so hexproof stops an opponent's targeting until cleanup: the
// kernel's TurnReplPlayerHex record, the player half pumpYouAndValid emits
// for Dawn's Truce. Any other keyword refuses by name: a kernel keyword
// grant lands on a permanent and never on a player, and the kernel has no
// player read for shroud (CR 702.18a) or a player's protection.
func (lc *lowerCtx) pumpPlayerEOT(ks []string) error {
	for _, k := range ks {
		if k != "Hexproof" {
			return refuse("player-scoped Pump grant keyword %q until end of turn", k)
		}
	}
	lc.emit(lInstr{Op: "TurnRepl", C: kop.TurnReplPlayerHex, Note: "you have hexproof this turn"})
	return nil
}

// protKwInto splits a granted-keyword list's protection terms into the
// override record's ProtFrom (splitProtFromKW) and lowers the rest through
// pumpKeywords into AddKw.
func (lc *lowerCtx) protKwInto(o *lOverride, kw string) error {
	rest, err := lc.splitProtFromKW(kw, o)
	if err != nil {
		return err
	}
	if rest == "" {
		return nil
	}
	if o.AddKw, err = pumpKeywords(rest); err != nil {
		return err
	}
	takeBanding(o)
	return nil
}

// pumpKwGrant lowers a battlefield object's keyword-only Pump Duration$
// whose grant is a layer-6 override record: Permanent (gorge AddContinuous) or
// UntilYourNextUpkeep (the resolving controller's next upkeep, CR 611.2b), or
// AsLongAsControl on an object the host controls (pumpAsLongAsControl). Any
// other duration refuses, as the Duration arm in pumpBody always has.
func (lc *lowerCtx) pumpKwGrant(d, kw string, obj lRef) error {
	switch d {
	case "Permanent":
		return lc.pumpPermanentKwGrant(kw, obj)
	case "UntilYourNextUpkeep":
		return lc.pumpUpkeepKwGrant(kw, obj)
	case "AsLongAsControl":
		if lc.pumpObjectRef(obj) {
			return lc.pumpAsLongAsControl(kw, obj)
		}
	}
	return refuse("Pump Duration %q", d)
}

// pumpPermanentKwGrant lowers a battlefield object's Pump Duration$
// Permanent keyword grant: the protection terms and the rest of the keyword
// list ride one layer-6 override record (pumpDurationBody).
func (lc *lowerCtx) pumpPermanentKwGrant(kw string, obj lRef) error {
	kw, horse := takePermanentHorsemanship(kw)
	o := &lOverride{Horsemanship: horse}
	if kw != "" {
		if err := lc.protKwInto(o, kw); err != nil {
			return err
		}
	}
	lc.emitOverride(obj, o, "Permanent")
	return nil
}

// pumpUpkeepKwGrant lowers a battlefield object's Pump Duration$
// UntilYourNextUpkeep keyword grant: one keyword-only override record whose
// waiting controller is the applied record's p (kop.Override.UntilUpkeep;
// dirac/static_untilnextupkeep.go). The grant lasts through the controller's
// untap step and ends as their upkeep begins (gorge dropUntilYourNextUpkeep).
func (lc *lowerCtx) pumpUpkeepKwGrant(kw string, obj lRef) error {
	o := &lOverride{UntilUpkeep: true}
	rest, landwalk, err := lc.splitLandwalkPump(kw) // lower_landwalkgrant.go
	if err != nil {
		return err
	}
	o.Landwalk = landwalk
	if rest != "" {
		if err := lc.protKwInto(o, rest); err != nil {
			return err
		}
	}
	lc.emitOverride(obj, o, "UntilYourNextUpkeep")
	return nil
}

// pumpAsLongAsControl lowers a battlefield object's Pump Duration$
// AsLongAsControl keyword grant: one host-anchored override record
// (WhileHost + AsLongAsControl) that gorge starts only while the activator
// controls the host and ends at the host's control change or phase-out. The
// can't-attack-or-block sentence is CantAttack beside AddKw CantBlock, so both
// halves ride the one record.
func (lc *lowerCtx) pumpAsLongAsControl(kw string, obj lRef) error {
	o := &lOverride{WhileHost: true, AsLongAsControl: true}
	rest, both := splitKWSentence(kw, cantAttackOrBlockKW)
	rest, atk := splitKWSentence(rest, cantAttackKW)
	o.CantAttack = both || atk
	if err := lc.protKwInto(o, rest); err != nil {
		return err
	}
	if both {
		o.AddKw = append(o.AddKw, "CantBlock")
	}
	lc.emitOverride(obj, o, "WhileHost")
	return nil
}

// pumpObjectDuration lowers a battlefield object's Pump with a Duration$ the
// kernel keeps as an override record: UntilYourNextTurn (gorge's durationTiming
// UntilTurn boundary, CR 611.2b; The Undercity's Throne), UntilUntaps (for as
// long as the activating host remains tapped, registerPumpEffects; the Animate
// model in animateOverride) and Permanent (a never-expiring layer 7 modify,
// durationTiming). A keyword grant rides the record's keyword bits; a
// NumAtt$/NumDef$ pump the same record's PTDelta (static_ptdelta.go: the +N/+M
// it adds with the counters), and both parts one record together as
// pumpAllDuration's does. A host-bound record (UntilUntaps) keeps the host's
// Obj and version in the record's P/T fields (applyWhileHost), so its delta
// rides the record's table SetP/SetT constants (ptDeltaRecs reads those while
// the override is WhileHost; basePTGen still skips it, HasPT clear) and its
// duration is WhileHost (instr_effects maps that to DurPermanent beside the
// table flag). A computed amount refuses: a record's P/T is read once at
// resolution, which a Count$ amount is not.
func (lc *lowerCtx) pumpObjectDuration(ps *params, obj lRef, na, nd bool, kw string, hasKW bool, dur string) error {
	// "HIDDEN CARDNAME can't attack." (Twisted Caverns): the override
	// record's CantAttack, no keyword bit; it is a keyword part of the grant
	cantAtk := false
	if hasKW {
		kw, cantAtk = splitKWSentence(kw, cantAttackKW)
		hasKW = kw != "" || cantAtk
	}
	// a permanent horsemanship grant rides the record (lower_kwmark.go)
	horse := false
	if dur == "Permanent" && hasKW {
		kw, horse = takePermanentHorsemanship(kw)
		hasKW = kw != "" || cantAtk || horse
	}
	host := dur == "UntilUntaps"
	if !na && !nd {
		if !hasKW {
			return refuse("Pump Duration %q without NumAtt$/NumDef$/KW$", dur)
		}
		o := &lOverride{CantAttack: cantAtk, Horsemanship: horse}
		if kw != "" {
			if err := lc.protKwInto(o, kw); err != nil {
				return err
			}
		}
		if host {
			o.WhileHost, o.WhileTapped, dur = true, true, "WhileHost"
		}
		lc.emitOverride(obj, o, dur)
		return nil
	}
	o := &lOverride{CantAttack: cantAtk, Horsemanship: horse}
	if kw != "" {
		if err := lc.protKwInto(o, kw); err != nil {
			return err
		}
	}
	var err error
	att, def := cst(0), cst(0)
	if v, ok := ps.get("NumAtt"); ok {
		if att, err = lc.value(v); err != nil {
			return err
		}
	}
	if v, ok := ps.get("NumDef"); ok {
		if def, err = lc.value(v); err != nil {
			return err
		}
	}
	if !att.isConst() || !def.isConst() {
		return refuse("Pump Duration %s with a computed P/T pump", dur)
	}
	o.PTDelta, o.PVal, o.TVal = true, att, def
	if host {
		// the kernel's host-bound record keeps its host in the record's P/T
		// fields (applyWhileHost), so the pump rides the record's table
		// constants beside PTDelta/PVal/TVal: ptDeltaRecs reads SetP/SetT of
		// a WhileHost record, and basePTGen still skips it (HasPT clear)
		o.SetP, o.SetT = att.N, def.N
		o.WhileHost, o.WhileTapped, dur = true, true, "WhileHost"
	}
	lc.emitOverride(obj, o, dur)
	return nil
}

// pumpDurationBody lowers a Pump with a Duration$ onto the kernel's override
// records: a keyword-only permanent grant (gorge AddContinuous Permanent), one
// lasting until the controller's next turn (the UntilTurn boundary, CR 611.2b),
// one lasting while the activating host stays tapped (registerPumpEffects
// UntilUntaps, the Animate model in animateOverride), or a permanent grant with
// a P/T part (a never-expiring layer 7 PTDelta record), or UntilYourNextUpkeep
// (pumpKwGrant). Any other duration, and
// any of these off the battlefield, refuses.
func (lc *lowerCtx) pumpDurationBody(ps *params, obj lRef, zone int) error {
	d, _ := ps.get("Duration")
	_, na := ps.m["NumAtt"]
	_, nd := ps.m["NumDef"]
	kw, hasKW := ps.get("KW")
	if lc.pumpPlayerRef(obj) {
		return lc.pumpPlayerGrant(ps, obj, zone, d, na, nd, kw, hasKW)
	}
	if done, err := lc.pumpTimedObject(ps, obj, zone, d, na, nd, kw, hasKW); done {
		return err // lower_pumpendcombat.go: UntilYourNextTurn, UntilEndOfCombat
	}
	if d == "UntilUntaps" && lc.pumpObjectRef(obj) && zone == zBattlefield {
		return lc.pumpObjectDuration(ps, obj, na, nd, kw, hasKW, d)
	}
	if d == "Permanent" && (na || nd) {
		// a P/T part makes the Permanent grant a layer 7 PTDelta record
		// too (Riding the Dilu Horse, Subversive Acolyte); the keyword-only
		// Permanent grant keeps pumpPermanentKwGrant's shape
		if lc.pumpObjectRef(obj) && zone == zBattlefield {
			return lc.pumpObjectDuration(ps, obj, na, nd, kw, hasKW, d)
		}
		return refuse("Pump Duration %q", d)
	}
	if na || nd || !hasKW || zone != zBattlefield {
		return refuse("Pump Duration %q", d)
	}
	return lc.pumpKwGrant(d, kw, obj)
}

// pumpBody emits one Pump's effects for the already-resolved subject obj.
// extra is the keyword a KWChoice$ branch chose (nil for a plain Pump).
func (lc *lowerCtx) pumpBody(ps *params, obj lRef, zone int, extra []string) error {
	if v, ok := ps.get("AtEOT"); ok {
		// gorge pump.go -> scheduleAtEOT over the pumped battlefield objects,
		// after the pump
		act, err := lc.atEOTActHere(v)
		if err != nil {
			return err
		}
		if atEOTGroupedAct(act) && lc.atEOTMaySpanSet(obj) {
			return refuse("Pump AtEOT %q over a set is one grouped trigger", v)
		}
		defer lc.emit(lInstr{Op: "AtEOT", Obj: obj, C: act})
	}
	if done, err := lc.skipNextUntap(ps, obj, zone); done || err != nil {
		return err
	}
	if _, ok := ps.get("Duration"); ok {
		return lc.pumpDurationBody(ps, obj, zone)
	}
	att, def := lValue{}, lValue{}
	if a, ok := ps.get("NumAtt"); ok {
		if a == "Double" {
			att = lValue{Kind: vPowerOf, Ref: obj}
		} else if a == "Triple" {
			// gorge statAddMultiplier: Triple adds twice the object's
			// current power (final 3x)
			att = lValue{Kind: vPowerOf, Ref: obj, Mul: 2}
		} else if v, err := lc.value(a); err != nil {
			return err
		} else {
			att = v
		}
	}
	if d, ok := ps.get("NumDef"); ok {
		if d == "Double" {
			// gorge NumForObject: the object's current toughness
			def = lValue{Kind: vToughnessOf, Ref: obj}
		} else if d == "Triple" {
			def = lValue{Kind: vToughnessOf, Ref: obj, Mul: 2}
		} else if v, err := lc.value(d); err != nil {
			return err
		} else {
			def = v
		}
	}
	if att.Kind != vNone || def.Kind != vNone {
		if att.Kind == vNone {
			att = cst(0)
		}
		if def.Kind == vNone {
			def = cst(0)
		}
		lc.emit(lInstr{Op: "PumpUntilEOT", Obj: obj, A: att, B: def})
	}
	if zone == zGraveyard && att.Kind == vNone && def.Kind == vNone {
		// Araumi of the Dead Tide: a Pump whose whole body is a graveyard
		// keyword grant. gorge's pump registers the derived keyword
		// [Encore:CardManaCost] on the card until end of turn and offers no
		// activation of it (kw_granted.go synthesizes no Encore body; the
		// grant lands and nothing is offered), so ExtGrantEncore encodes the
		// grant alone. Any other keyword on a graveyard card stays refused
		// below.
		if done, err := lc.pumpGraveyardKwGrant(ps, obj); done {
			return err // the Encore grant above; Cursecloth Wrappings' embalm (lower_embalmgrant.go)
		}
	}
	kw, hasKW := ps.get("KW")
	mustBlock, noCombat, lure, marks := false, false, false, 0
	if hasKW {
		kw, mustBlock = splitKWSentence(kw, mustBeBlockedKW)
		kw, noCombat = splitKWSentence(kw, cantAttackOrBlockKW)
		kw, lure = splitKWSentence(kw, lureKW)
		kw, marks = splitKwMarks(kw) // lower_kwmark.go
	}
	if hasKW && kw != "" {
		if err := lc.pumpGrantKw(kw, obj, zone); err != nil {
			return err
		}
	}
	if len(extra) > 0 {
		if lc.pumpPlayerRef(obj) {
			return refuse("player-scoped Pump KWChoice$ grant")
		}
		lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: strings.Join(extra, "&")})
	}
	if mustBlock {
		if zone != zBattlefield || !lc.pumpObjectRef(obj) {
			return refuse("Pump KW$ must be blocked if able on a non-permanent subject")
		}
		lc.emit(lInstr{Op: "MustBeBlocked", Obj: obj})
	}
	if marks != 0 {
		if zone != zBattlefield || !lc.pumpObjectRef(obj) {
			return refuse("Pump KW$ %s on a non-permanent subject", kwMarkNames(marks))
		}
		lc.emit(lInstr{Op: "KwMark", Obj: obj, C: marks, Note: kwMarkNames(marks) + " until end of turn"})
	}
	if lure {
		if zone != zBattlefield || !lc.pumpObjectRef(obj) {
			return refuse("Pump KW$ all must block on a non-permanent subject")
		}
		lc.emit(lInstr{Op: "MustBeBlocked", Obj: obj, C: kop.MustBeBlockedLure})
	}
	if noCombat {
		if zone != zBattlefield || !lc.pumpObjectRef(obj) {
			return refuse("Pump KW$ can't attack or block on a non-permanent subject")
		}
		// the Animate HiddenKeywords$ pair (animateHiddenKeywords): the
		// kernel's until-end-of-turn KwCantBlock grant, and the override
		// record's CantAttack (there is no kernel keyword bit for the attack half)
		lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: "CantBlock"})
		lc.emitOverride(obj, &lOverride{CantAttack: true}, "EOT")
	}
	return nil
}

// pumpGrantKw emits one Pump's granted-keyword sentence (the KW$ value split
// into rest keywords, Firebending, Myriad, Landwalk and protection grants,
// each until end of turn as the rules grant them) for the already-resolved
// subject obj.
func (lc *lowerCtx) pumpGrantKw(kw string, obj lRef, zone int) error {
	rest, fbs, err := splitFirebendingGrant(kw)
	if err != nil {
		return err
	}
	if len(fbs) > 0 && zone != zBattlefield {
		return refuse("granted Firebending to a non-battlefield object")
	}
	rest, myriads := splitMyriadGrant(rest)
	if myriads > 0 && (zone != zBattlefield || lc.pumpPlayerRef(obj)) {
		return refuse("granted Myriad to a non-battlefield object")
	}
	rest, attackKws, err := splitAttackKwGrant(rest) // lower_attackkwgrant.go
	if err != nil {
		return err
	}
	if err := refuseEffectReturnKw(attackKws); err != nil {
		return err
	}
	if len(attackKws) > 0 && (zone != zBattlefield || lc.pumpPlayerRef(obj)) {
		return refuse("granted %s to a non-battlefield object", attackKws[0])
	}
	prot := &lOverride{}
	if !lc.pumpPlayerRef(obj) {
		// the protection terms lower to the grant's quality filter
		// (kop.Override.ProtFrom); a player subject's protection grant still
		// refuses below, as pumpPlayerEOT always has
		rest, err = lc.splitProtFromKW(rest, prot)
		if err != nil {
			return err
		}
		// a granted Banding is the same record's Banding (lower_banding.go)
		if rest, err = lc.splitBandingGrant(rest, prot, zone == zBattlefield && lc.pumpObjectRef(obj)); err != nil {
			return err
		}
	}
	rest, landwalk, err := lc.splitLandwalkPump(rest) // lower_landwalkgrant.go
	if err != nil {
		return err
	}
	if landwalk != nil && (zone != zBattlefield || !lc.pumpObjectRef(obj)) {
		return refuse("granted landwalk to a non-permanent subject")
	}
	if prot.ProtFrom != nil && (zone != zBattlefield || !lc.pumpObjectRef(obj)) {
		return refuse("granted protection to a non-permanent subject")
	}
	if rest != "" || prot.ProtFrom == nil && !prot.Banding && len(fbs)+myriads+len(attackKws) == 0 && landwalk == nil {
		ks, err := pumpKeywords(rest)
		if err != nil {
			return err
		}
		if lc.pumpPlayerRef(obj) {
			if err := lc.pumpPlayerEOT(ks); err != nil {
				return err
			}
		} else {
			lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: strings.Join(ks, "&")})
		}
	}
	if prot.ProtFrom != nil || prot.Banding {
		// CR 702.16 granted until end of turn: an end-of-turn override record
		// carrying the protection's quality filter (kop.Override.ProtFrom),
		// the colour-mask clauses of the "Protection from <colour>" spellings
		// and lc.filter's clauses of the Protection:<spec> spellings beside it
		lc.emitOverride(obj, prot, "EOT")
	}
	if landwalk != nil {
		// CR 702.14 granted until end of turn: an end-of-turn override
		// record carrying the land filter (kop.Override.Landwalk)
		lc.emitOverride(obj, &lOverride{Landwalk: landwalk}, "EOT")
	}
	for _, n := range fbs {
		// CR 702.189a granted until end of turn: the latent Attacks
		// trigger rides an end-of-turn override record; identical
		// applications stack (lower_firegrant.go)
		idx, err := lc.firebendGrantTrigger(n)
		if err != nil {
			return err
		}
		lc.emitOverride(obj, &lOverride{GrantAbility: []int{idx}}, "EOT")
	}
	for _, line := range attackKws {
		// CR 702.86a / 702.91a granted until end of turn (Reckless
		// Pyrosurfer): the latent Attacks Self trigger rides an end-of-turn
		// override record (lower_attackkwgrant.go)
		idx, err := lc.attackKwGrantTrigger(line)
		if err != nil {
			return err
		}
		lc.emitOverride(obj, &lOverride{GrantAbility: []int{idx}}, "EOT")
	}
	for range myriads {
		// CR 702.116a granted until end of turn (Duke Ulder Ravengard):
		// gorge's granted walk synthesizes the trigger from the
		// registered AddKeywords line (lower_myriad.go)
		idx, err := lc.myriadGrantTrigger()
		if err != nil {
			return err
		}
		lc.emitOverride(obj, &lOverride{GrantAbility: []int{idx}}, "EOT")
	}
	return nil
}

// cantAttackOrBlockKW is the Pump keyword sentence for "can't attack or block
// this turn" (CR 508.1, 509.1: Alchemist's Vial). Like mustBeBlockedKW it is a
// mark, not a keyword bit.
const cantAttackOrBlockKW = "HIDDEN CARDNAME can't attack or block."

// cantAttackKW is the Pump keyword sentence "can't attack" (Twisted Caverns'
// Duration$ UntilYourNextTurn grant): the override record's CantAttack.
const cantAttackKW = "HIDDEN CARDNAME can't attack."

// mustBeBlockedKW is the one Pump / PumpAll keyword sentence that is a mark,
// not a keyword bit (CR 509.1c, ExtMustBeBlocked).
const mustBeBlockedKW = "HIDDEN CARDNAME must be blocked if able."

// lureKW is the Pump keyword sentence "All creatures able to block CARDNAME do
// so." (CR 509.1c; gorge hidden keyword AllMustBlock): a mark too, set by
// ExtMustBeBlocked with C = MustBeBlockedLure.
const lureKW = "HIDDEN All creatures able to block CARDNAME do so."

// protEachColorKW is the PumpAll KW$ term "Protection from each color"
// (Akroma's Will): a mark per colour, not a keyword bit (ExtColorGrant).
const protEachColorKW = "Protection from each color"

// splitKWSentence takes one mark sentence (mustBeBlockedKW,
// cantAttackOrBlockKW) out of a KW$ list: the rest of the list (possibly
// empty) and whether the sentence was there. Any other spelling stays in the
// list, where pumpKeywords refuses it.
func splitKWSentence(kw, sentence string) (rest string, found bool) {
	var keep []string
	for _, k := range strings.Split(kw, "&") {
		if strings.TrimSpace(k) == sentence {
			found = true
			continue
		}
		keep = append(keep, strings.TrimSpace(k))
	}
	return strings.Join(keep, " & "), found
}

// pumpKeywords lowers a KW$ list ("Flying & Double Strike").
func pumpKeywords(kw string) ([]string, error) {
	var out []string
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		switch {
		case simpleKeywords[k]:
			out = append(out, k)
		case k == "HIDDEN CARDNAME can't block.":
			out = append(out, "CantBlock")
		case k == "Prevent all combat damage that would be dealt to CARDNAME.":
			out = append(out, "PreventCombatDamageTo")
		case k == "Prevent all combat damage that would be dealt by CARDNAME.":
			// a granted "dealt by" sentence (Hunter's Ambush's nongreen
			// creatures, Feint's target): gorge's preventKWs by-combat head
			// (rules/replacement_prevent_kw.go), the kernel's prevCombatFrom
			out = append(out, "PreventCombatDamageFrom")
		case k == "Wither":
			out = append(out, "Wither") // CR 702.80 granted; not in simpleKeywords (that map also drives the printed K: switch)
		case k == "Banding":
			// a granted Banding (CR 702.22, Battering Ram): no KwMask bit is
			// free; the head stays in the list and takeBanding moves it onto
			// the record's Banding (lower_banding.go)
			out = append(out, "Banding")
		case k == "Flashback":
			out = append(out, "FlashbackOwnCost")
		case strings.HasPrefix(k, "Protection from "):
			c := strings.TrimPrefix(k, "Protection from ")
			if i := strings.Index("whiteblueblackredgreen", c); i < 0 || !(c == "white" || c == "blue" || c == "black" || c == "red" || c == "green") {
				return nil, refuse("keyword %q", k)
			}
			out = append(out, "ProtectionFrom"+strings.ToUpper(c[:1])+c[1:])
		default:
			return nil, refuse("keyword %q", k)
		}
	}
	return out, nil
}

// colorGrantPump lowers a Pump DefinedKW$ ChosenColor after ChooseColorRes
// (Skrelv, Defector Mite; Mondo Gecko, Defined$ Self; gorge definedKWGrant,
// cr-pump-definedkw-chosencolor): the object -- the one object target, or
// the source itself (Defined$ Self) -- gains, until end of turn, hexproof
// from the chosen colour (Hexproof:ChosenColor) and/or toxic N (Toxic:N):
// OpExt ExtColorGrant, C bit 0 hexproof, A the toxic amount. No P/T
// change, no other keyword, no other duration.
func (lc *lowerCtx) colorGrantPump(ps *params) error {
	if !lc.colGrant || ps.str("DefinedKW") != "ChosenColor" {
		return refuse("Pump DefinedKW$ %q outside a chosen-colour grant", ps.str("DefinedKW"))
	}
	for _, k := range [...]string{"NumAtt", "NumDef", "Duration", "KWChoice", "PumpZone"} {
		if _, ok := ps.get(k); ok {
			return refuse("chosen-colour Pump %s$", k)
		}
	}
	if d, ok := ps.get("Defined"); ok && d != "Targeted" && d != "Self" {
		return refuse("chosen-colour Pump Defined$ %q", d)
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget && obj.Kind != rSelf {
		return refuse("chosen-colour Pump without its own target")
	}
	in := lInstr{Op: "ColorGrant", Obj: obj, A: cst(0), Note: "until end of turn: the chosen colour's grants"}
	for _, kw := range strings.Split(ps.str("KW"), "&") {
		kw = strings.TrimSpace(kw)
		switch {
		case kw == "Hexproof:ChosenColor":
			in.C |= 1
		case strings.HasPrefix(kw, "Toxic:"):
			n, err := strconv.Atoi(kw[len("Toxic:"):])
			if err != nil || n < 1 || n > 7 || in.A.N != 0 {
				return refuse("chosen-colour Pump %q", kw)
			}
			in.A = cst(n)
		default:
			return refuse("chosen-colour Pump keyword %q", kw)
		}
	}
	lc.colTgt = obj
	lc.emit(in)
	return nil
}

// colorGrantEffect lowers the Effect after a chosen-colour Pump: one
// CantBlockBy static, ValidAttacker$ Creature.IsRemembered (the pumped
// target, RememberObjects$ Targeted) and ValidBlocker$ Creature.ChosenColor,
// for the turn (gorge effEffect + freezeChosenColor): ExtColorGrant C bit 1.
func (lc *lowerCtx) colorGrantEffect(ps *params) error {
	if ps.str("RememberObjects") != "Targeted" || lc.colTgt.Kind != rTarget {
		return refuse("chosen-colour Effect RememberObjects$ %q", ps.str("RememberObjects"))
	}
	for _, k := range [...]string{"Duration", "Triggers", "ReplacementEffects", "ExileOnMoved", "ForgetOnMoved"} {
		if _, ok := ps.get(k); ok {
			return refuse("chosen-colour Effect %s$", k)
		}
	}
	name := strings.TrimSpace(ps.str("StaticAbilities"))
	line, ok := lc.face.SVars[name]
	if !ok || strings.Contains(name, ",") {
		return refuse("chosen-colour Effect statics %q", name)
	}
	want := map[string]string{"Mode": "CantBlockBy", "ValidAttacker": "Creature.IsRemembered", "ValidBlocker": "Creature.ChosenColor"}
	seen := 0
	for _, part := range strings.Split(line, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "$")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "Description" {
			continue
		}
		if want[k] != v {
			return refuse("chosen-colour Effect static param %s", k)
		}
		seen++
	}
	if seen != len(want) {
		return refuse("chosen-colour Effect static shape")
	}
	lc.emit(lInstr{Op: "ColorGrant", Obj: lc.colTgt, A: cst(0), C: 2, Note: "this turn: can't be blocked by creatures of the chosen colour"})
	return nil
}
