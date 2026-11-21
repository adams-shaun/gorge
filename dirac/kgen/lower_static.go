package kgen

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// staticUnreadRefuse refuses a static's unread params; when the static's
// spell-ability filter (ValidSA$) is among them the refusal names the guard
// (T12: no kernel word reads it yet, T13c's FwSA* family), for the census to
// count it.
func staticUnreadRefuse(left []string) error {
	if slices.Contains(left, "ValidSA") {
		return refuse("static spell-ability filter ValidSA %v", left)
	}
	return refuse("static unread params %v", left)
}

func (lc *lowerCtx) lowerStatic(s cards.Static) error {
	if s.Mode == "FlipCoinMod" {
		return lc.flipCoinModStatic(s) // lower_turnfacts.go
	}
	if s.Mode == "TapPowerValue" {
		return lc.tapPowerValueStatic(s) // lower_hiddenpick.go
	}
	if s.Mode == "OptionalCost" {
		return lc.optCostStatic(s) // lower_optcost.go
	}
	if s.Mode == "Continuous" && (s.Params["CanBlockAmount"] != "" || s.Params["CanBlockAny"] != "") {
		if ok, err := lc.extraBlockStatic(s); ok || err != nil {
			return err // lower_extrablock.go
		}
	}
	if s.Mode == "MinMaxBlocker" && s.Params["Max"] == "" {
		return lc.minMaxBlockerStatic(s) // lower_choosetype.go
	}
	if s.Params["Condition"] == "MaxSpeed" {
		return lc.maxSpeedStatic(s) // lower_speed.go
	}
	if isChosenModeStatic(s) {
		return lc.chosenModeStatic(s) // lower_siegemode.go
	}
	if s.Mode == "CantAttackUnless" {
		if err := attackTaxReadable(s.Params); err != nil {
			return err // lower_attacktax.go: before the EffectZone$ strip below
		}
	}
	if s.Mode == "CantBlockUnless" {
		if err := blockTaxReadable(s.Params); err != nil {
			return err // lower_blocktax.go: before the EffectZone$ strip below
		}
	}
	s, inGrave, err := lc.graveContinuous(s) // lower_gycontinuous.go
	if err != nil {
		return err
	}
	if s.Params["EffectZone"] == "Battlefield" {
		// a printed static's default zone spelled out (gorge
		// EffectZoneOK: the battlefield, as with no EffectZone$)
		ps := make(map[string]string, len(s.Params))
		for k, v := range s.Params {
			if k != "EffectZone" {
				ps[k] = v
			}
		}
		s.Params = ps
	}
	// a printed Mode$ Continuous static reads ExiledWithSource against its own
	// source (kernel ctxStatic); an emblem's or an Effect's static has another
	// source object, so only this entry sets it
	lc.exwStatic = s.Mode == "Continuous"
	lc.srcRemStatic = lc.out.Token && lc.exwStatic && s.Params["CharacteristicDefining"] == "True"
	sts, err := lc.staticBody(s)
	lc.exwStatic, lc.srcRemStatic = false, false
	if err != nil {
		return err
	}
	if inGrave {
		if sts, err = graveContinuousFlag(sts); err != nil {
			return err
		}
	}
	for _, st := range sts {
		if st.Kind == "" {
			continue // a view-only MayLookAt lowers to no static
		}
		lc.out.Statics = append(lc.out.Statics, st)
	}
	return nil
}

// hasFlash reports whether the face prints Flash.
func (lc *lowerCtx) hasFlash() bool {
	for _, kw := range lc.face.Keywords {
		if kw == "Flash" {
			return true
		}
	}
	return false
}

// selfFlashPresent is the IsPresent$ spec shape a self flash grant may read:
// a type or subtype word and controller / owner / simple card-type
// qualifiers. The gate is evaluated for the caster with the carrier still in
// hand (no battlefield source), so a source-relative term (Self, Other,
// Enchanted, ...) is refused.
var selfFlashPresent = regexp.MustCompile(`^[A-Z][A-Za-z]*(\.(YouCtrl|YouOwn|OppCtrl|OppOwn|nonCreature|nonLand))*$`)

// selfFlashShape checks a CastWithFlash static that names EffectZone$ is the
// self grant kernel selfFlash evaluates (gorge castWithFlashTargets over
// withSelfStatics): EffectZone$ All, ValidCard$ Card.Self, and only the
// IsPresent$ / PresentZone$ / PresentCompare$ gate, read against the caster.
func (lc *lowerCtx) selfFlashShape(s cards.Static, ez string) error {
	if strings.TrimSpace(ez) != "All" {
		return refuse("CastWithFlash EffectZone %q", ez)
	}
	if strings.TrimSpace(s.Params["ValidCard"]) != "Card.Self" {
		return refuse("CastWithFlash EffectZone$ All beside ValidCard %q", s.Params["ValidCard"])
	}
	if lc.hasFlash() {
		return refuse("CastWithFlash self grant on a card with flash")
	}
	for k := range s.Params {
		switch k {
		case "Mode", "ValidCard", "ValidSA", "Caster", "EffectZone", "IsPresent", "PresentZone", "PresentCompare", "Description":
		default:
			return refuse("CastWithFlash self grant with %s$ (the gate is read with no source on the battlefield)", k)
		}
	}
	if ip, ok := s.Params["IsPresent"]; ok && !selfFlashPresent.MatchString(strings.TrimSpace(ip)) {
		return refuse("CastWithFlash self grant IsPresent %q", ip)
	}
	if _, ok := s.Params["IsPresent"]; !ok {
		if _, ok := s.Params["PresentZone"]; ok {
			return refuse("CastWithFlash self grant PresentZone$ without IsPresent$")
		}
	}
	return nil
}

// staticBody lowers a static's parameters.
func (lc *lowerCtx) staticBody(s cards.Static) ([]lStatic, error) {
	sp := newParams(s.Params)
	sp.consume("Mode")
	if lc.aiHintSVars(s.Params["AddSVar"]) || s.Mode == "Continuous" && lc.inertAddSVar(s.Params["AddSVar"]) {
		sp.consume("AddSVar") // a variable grant gorge's layer walk drops or no rules reader reads
	}
	st := lStatic{Kind: s.Mode}
	readF := func(k string) (*lFilter, error) {
		v, ok := sp.get(k)
		if !ok {
			return nil, nil
		}
		return lc.filter(v)
	}
	readCond := func() error {
		if c, ok := sp.get("Condition"); ok {
			switch c {
			case "Threshold":
				st.Cond = lCond{Kind: cThreshold}
			case "PlayerTurn":
				st.Cond = lCond{Kind: cYourTurn}
			default:
				// gorge continuousConditionHolds (Continuous) and
				// costConditionHolds (a self cost static: the payer is
				// the controller) read these against the controller. The
				// self-carried AlternativeCost (gorge
				// alternativeCostScopeOK's costConditionHolds read, the
				// Force cycle: the caster is the static's controller, the
				// printed you) and the combat restriction statics (gorge
				// combatBoard.StaticGateHolds' continuousGateHolds,
				// Monstrous Hound) read theirs against the static's
				// controller, which statCond's and offerAltFreeHand's
				// Cond2 contexts name too.
				selfCost := (s.Mode == "ReduceCost" || s.Mode == "RaiseCost") && s.Params["ValidCard"] == "Card.Self"
				turnGate := s.Mode == "Continuous" || selfCost || reduceActivatorYou(s.Mode, s.Params) ||
					s.Mode == "AlternativeCost" || s.Mode == "CantBlock" || s.Mode == "CantAttack"
				switch {
				case c == "NotPlayerTurn" && turnGate:
					st.Cond = lCond{Kind: cYourTurn, Neg: true}
				case c == "Metalcraft" && (s.Mode == "Continuous" || selfCost):
					f, err := lc.filter("Artifact.YouCtrl")
					if err != nil {
						return err
					}
					st.Cond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(3)}
				case c == "Delirium" && (s.Mode == "Continuous" || selfCost):
					st.Cond = deliriumCond() // gorge graveyardCardTypeCount >= 4
				case c == "Hellbent" && s.Mode == "Continuous":
					st.Cond = lCond{Kind: cValue, A: lValue{Kind: vHandSizeOf, Ref: lRef{Kind: rYou}}, Op: opEQ, B: cst(0)}
				case c == "Blessing" && (s.Mode == "Continuous" || s.Mode == "CantBlockBy"):
					// gorge condBlessing: the controller's latch (CR 702.131).
					// A CantBlockBy static's condition rides Static.Cond2,
					// which the kernel's blockForbiddenGen consults through
					// statCond with the static's controller, the same
					// context gorge's BlockRestricted' StaticGateHolds walk
					// reads the condition against.
					st.Cond = lCond{Kind: cBlessing}
				default:
					return refuse("static Condition %q", c)
				}
			}
		}
		if cs, ok := sp.get("CheckSVar"); ok {
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
			if cmp, ok := sp.get("SVarCompare"); ok {
				op, b, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				c.Op, c.B = op, b
			}
			st.Cond = andCond(st.Cond, c)
		}
		if s.Mode == "AlternativeCost" {
			if err := altCostSecondSVar(lc, sp, &st); err != nil {
				return err // lower_altcostsecondsvar.go
			}
		}
		if ip, ok := sp.get("IsPresent"); ok {
			if band := levelBandFilters(ip); len(band) > 1 && sp.m["PresentCompare"] == "" && sp.m["PresentZone"] == "" {
				// a level band's two counter compares (lower_levelband.go)
				for _, bf := range band {
					f, err := lc.filter(bf)
					if err != nil {
						return err
					}
					st.Cond = andCond(st.Cond, lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)})
				}
				return nil
			}
			f, err := lc.filter(ip)
			if err != nil {
				return err
			}
			c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
			if pc, ok := sp.get("PresentCompare"); ok {
				op, v, err := lc.cmpSpec(pc)
				if err != nil {
					return err
				}
				c.Op, c.A = op, v
			}
			if pz, ok := sp.get("PresentZone"); ok {
				if c, err = presentZoneCond(c, pz, ip, lc); err != nil {
					return err
				}
			}
			st.Cond = andCond(st.Cond, c)
		}
		return nil
	}
	if err := readCond(); err != nil {
		return nil, err
	}
	if b, ok := sp.get("ClassBand"); ok {
		if s.Mode != "Continuous" {
			return nil, refuse("ClassBand on a %s static", s.Mode) // only the Continuous condition is lockstep-verified
		}
		c, err := classBandCond(b) // lower_delayed.go
		if err != nil {
			return nil, err
		}
		st.Cond = andCond(st.Cond, c)
	}
	var extra []lStatic
	h := staticModes[s.Mode]
	if h == nil {
		return nil, refuse("static mode %s", s.Mode)
	}
	sc := &staticCtx{lc: lc, s: s, sp: sp, st: st, readF: readF}
	if out, done, err := h(sc); err != nil || done {
		return out, err
	}
	st, extra = sc.st, sc.extra
	if left := sp.unread(); len(left) > 0 {
		return nil, staticUnreadRefuse(left)
	}
	if inner := lc.grantInner; len(inner) > 0 {
		// a station self grant's inner statics follow the outer static
		lc.grantInner = nil
		if extra == nil {
			extra = []lStatic{st}
		}
		return append(extra, inner...), nil
	}
	if extra != nil {
		return extra, nil
	}
	return []lStatic{st}, nil
}

// mayPlayStatic lowers "you may play/cast cards matching Affected from
// AffectedZone" (Muldrotha, Tinybones, Quilled Greatwurm, Omniscience).
func (lc *lowerCtx) mayPlayStatic(sp *params, st *lStatic, aff string) error {
	if sp.str("AffectedZone") == "Library" {
		if err := libTopRefusals(sp); err != nil {
			return err
		}
		var err error
		if aff, err = libTopAffected(aff); err != nil {
			return err
		}
	}
	if err := laterPermanentAlternative(aff); err != nil {
		return err
	}
	f, err := lc.filter(aff)
	if err != nil {
		return err
	}
	if err := ownGraveLandsOnly(sp, f); err != nil {
		return err
	}
	st.Kind = "MayPlayFrom"
	st.F = f
	zRaw := sp.str("AffectedZone")
	z := zRaw
	if z == "All" {
		// Weftwalking's first-spell free cast: the exact shape only
		if !sp.is("MayPlayDontGrantZonePermissions", "True") ||
			sp.str("MayPlayPlayer") != "Player.Active" ||
			!sp.is("MayPlayWithoutManaCost", "True") {
			return refuse("MayPlay zone %q outside the first-spell free shape", zRaw)
		}
		z = "Hand"
		st.Flags = append(st.Flags, "FreeOnly")
	}
	// "from outside the game" (The Holodeck's AffectedZone$ Sideboard,Graveyard,
	// CR 400.11): the sideboard rides beside the one real zone as a flag, so a
	// two-zone permission stays ONE static with one once-per-turn count
	// (gorge keys the limit on the static, not the zone)
	if base, ok := strings.CutPrefix(z, "Sideboard,"); ok && !strings.Contains(base, ",") {
		z = base
		st.Flags = append(st.Flags, "AlsoSideboard")
	}
	st.Zone = zoneNames[z]
	if st.Zone != zGraveyard && st.Zone != zExile && st.Zone != zHand && st.Zone != zLibrary {
		return refuse("MayPlay zone %q", z)
	}
	if dg, ok := sp.get("MayPlayDontGrantZonePermissions"); ok && (dg != "True" || zRaw != "All") {
		if !exileZeroAltShape(sp, dg, zRaw) {
			return refuse("MayPlayDontGrantZonePermissions$ %q on zone %q", dg, zRaw)
		}
		// Dragon's Smile: once during each of your turns, {0} instead of the
		// mana cost of a spell cast from exile through another permission
		// (gorge mayPlayAltCosts / mayPlayCastAlts: a non-granting
		// alternative, AltCostIndex 1 beside the plain cast). A FreeOnly
		// static on Exile is that alternative (dirac/cast_mayzeroalt.go);
		// OncePerTurn and the PlayerTurn condition ride as for any static.
		sp.consume("MayPlayAltManaCost")
		st.Flags = append(st.Flags, "FreeOnly", "WithoutManaCost")
	}
	if mp, ok := sp.get("MayPlayPlayer"); ok {
		if mp != "Player.Active" || zRaw != "All" {
			return refuse("MayPlayPlayer$ %q", mp)
		}
		st.Flags = append(st.Flags, "SeatActive")
	}
	if ez, ok := sp.get("EffectZone"); ok {
		switch ez {
		case "Battlefield":
		case "Graveyard":
			st.Flags = append(st.Flags, "FunctionsInGraveyard")
		case "Exile":
			st.Flags = append(st.Flags, "FunctionsInExile")
		default:
			return refuse("MayPlay EffectZone %q", ez)
		}
	}
	if l, ok := sp.get("MayPlayLimit"); ok {
		if l != "1" {
			return refuse("MayPlayLimit %q", l)
		}
		// once per turn PER STATIC, a library permission's too (gorge
		// engine/rules/mayplay.go mayPlayKeyLimitReached)
		st.Flags = append(st.Flags, "OncePerTurn")
	}
	if sp.is("MayPlayIgnoreType", "True") {
		st.Flags = append(st.Flags, "AnyManaType")
	}
	if _, ok := sp.get("MayPlaySnowIgnoreColor"); ok {
		// Draugr Necromancer's "spend mana from snow sources as though it
		// were any colour": gorge ignores the rider (engine/effects/
		// staticline.go reads MayPlayIgnoreColor, not the snow variant), so
		// the grant is the plain may-play in both engines.
		sp.consume("MayPlaySnowIgnoreColor")
	}
	if sp.is("MayPlayWithoutManaCost", "True") {
		st.Flags = append(st.Flags, "WithoutManaCost")
	}
	if rc, ok := sp.get("RaiseCost"); ok {
		// RemoveAnyCounter<N/Any/Creature>: remove N counters from among
		// creatures you control (Quilled Greatwurm 6, Dawnhand Dissident 3);
		// the count rides in Static.A (the flag keeps its spelling)
		n, ok := removeAnyCounterN(rc)
		if !ok {
			return lc.mayRaiseStatic(rc, st) // PayLife / Discard / PayLife + Sac (lower_mayraise.go)
		}
		st.Flags = append(st.Flags, "RaiseCostRemove6CountersFromCreatures")
		st.Amount = cst(n)
	}
	return mayPlayAltManaCost(sp, st)
}

// mayPlayAltManaCost reads a MayPlay static's MayPlayAltManaCost$ as the
// permission's alternative cost (CR 118.9). The kernel carries one shape,
// kop.StMayPayLife: "pay life equal to its mana value rather than pay its
// mana cost" on a library-top or exile permission (Bolas's Citadel,
// Valgavoth, Terror Eater), the flag the Effect-delivered library grant
// (Gwenom, Remorseless) already sets. It rides alone: any other flag on the
// static (a limit, a waived cost, a raise, a function zone) or any other
// alternative (a fixed mana cost: Glimpse the Cosmos) is refused, since the
// kernel static has no field for the cost.
func mayPlayAltManaCost(sp *params, st *lStatic) error {
	if slices.Contains(st.Flags, "FreeOnly") {
		return nil // Dragon's Smile's {0} alternative, consumed above
	}
	v, ok := sp.get("MayPlayAltManaCost")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "PayLife<ConvertedManaCost>" {
		return refuse("MayPlayAltManaCost %q", v)
	}
	if (st.Zone != zLibrary && st.Zone != zExile) || len(st.Flags) > 0 {
		return refuse("MayPlayAltManaCost %q beside zone %d flags %v", v, st.Zone, st.Flags)
	}
	st.Flags = append(st.Flags, "MayPayLife")
	return nil
}

// exileZeroAltShape reports whether a MayPlayDontGrantZonePermissions$ static
// is Dragon's Smile's exact "once during each of your turns, you may pay {0}
// rather than pay the mana cost for a spell you cast from exile": the only
// non-granting alternative-cost shape the kernel models.
func exileZeroAltShape(sp *params, dg, zone string) bool {
	_, mp := sp.get("MayPlayPlayer")
	_, rc := sp.get("RaiseCost")
	_, ez := sp.get("EffectZone")
	return dg == "True" && zone == "Exile" && sp.is("MayPlayAltManaCost", "0") &&
		sp.is("MayPlayLimit", "1") && sp.is("Condition", "PlayerTurn") &&
		!mp && !rc && !ez && !sp.is("MayPlayWithoutManaCost", "True")
}

// removeAnyCounterN parses `RemoveAnyCounter<N/Any/Creature>` for N in 1..255
// (no leading zero); any other counter kind or filter is not read.
func removeAnyCounterN(rc string) (int, bool) {
	body, ok := strings.CutPrefix(rc, "RemoveAnyCounter<")
	if !ok {
		return 0, false
	}
	body, ok = strings.CutSuffix(body, "/Any/Creature>")
	if !ok || body == "" || body[0] == '0' {
		return 0, false
	}
	n, err := strconv.Atoi(body)
	if err != nil || n < 1 || n > 255 {
		return 0, false
	}
	return n, true
}

// laterPermanentAlternative refuses a MayPlay$ Affected$ list whose second or
// later alternative has the Permanent base (Kagha, Shadow Archdruid's
// "Land.YouOwn+...,Permanent.YouOwn+..."). Off the battlefield Forge reads
// Permanent as a permanent card, which the kernel's filter does, but gorge's
// targetSpecForZone (engine/rules/stack.go) rewrites only a leading
// Permanent to PermanentCard, so gorge's later alternative matches no card
// in the affected zone and the two engines offer different casts.
func laterPermanentAlternative(aff string) error {
	alts := strings.Split(aff, ",")
	for _, a := range alts[1:] {
		a = strings.TrimSpace(a)
		if b, ok := strings.CutPrefix(a, "Permanent"); ok && (b == "" || b[0] == '.' || b[0] == '+') {
			return refuse("MayPlay Affected$ %q: a later Permanent alternative off the battlefield (gorge targetSpecForZone rewrites only the first)", aff)
		}
	}
	return nil
}

// ownGraveLandsOnly refuses a typed (MayPlayText$) graveyard MayPlay$ grant
// that admits a land its controller may not own (Coram, the Undertaker's
// "play a land ... from among cards in graveyards"). gorge registers no
// may-play effect for a MayPlayText$ static (layers.go mayPlayGrant), so its
// land offers come from the fallback walk alone, which reads only the
// player's own graveyard (legal_mayplay.go mayPlayLandIds walk(p, true));
// the kernel offers an opponent's land too.
func ownGraveLandsOnly(sp *params, f *lFilter) error {
	if strings.TrimSpace(sp.m["MayPlayText"]) == "" || !strings.Contains(sp.str("AffectedZone"), "Graveyard") {
		return nil
	}
	for _, c := range f.Clauses {
		if (c.Types == 0 || c.Types&tLand != 0) && c.NotTypes&tLand == 0 && c.Owner != whoYou && c.Ctrl != whoYou {
			return refuse("MayPlayText$ graveyard grant of a land you may not own: gorge's land walk reads only your graveyard")
		}
	}
	return nil
}
