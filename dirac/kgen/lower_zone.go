package kgen

import (
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

func zoneOf(ps *params, key string, dflt int) (int, error) {
	v, ok := ps.get(key)
	if !ok {
		return dflt, nil
	}
	z, ok := zoneNames[v]
	if !ok {
		return 0, refuse("%s %q", key, v)
	}
	return z, nil
}

// changeZone lowers api:ChangeZone: targeted moves, defined moves, and the
// hidden-zone searches (library, or a graveyard choice without targeting).
func (lc *lowerCtx) changeZoneMove(ps *params) error {
	if err := libPlaceManyDefined(ps); err != nil { // lower_libplace.go
		return err
	}
	if _, ok := ps.m["DestAltSVar"]; ok {
		return lc.czAltDest(ps) // lower_czaltdest.go
	}
	if isSelfAndChosen(ps) {
		return lc.selfAndChosenExile(ps) // lower_selfchosen.go
	}
	if v, ok := ps.m["Unearth"]; ok {
		// K:Unearth's expansion (gorge cards/kw_unearth.go: AB$ ChangeZone
		// Origin$ Graveyard Destination$ Battlefield Defined$ Self
		// SorcerySpeed$ ActivationZone$ Graveyard Unearth$ True): the
		// return, then OpUnearth's riders (gorge
		// rules/unearth.go unearthEnter / unearthReplacementApplies)
		if v != "True" || ps.m["Origin"] != "Graveyard" || ps.m["Destination"] != "Battlefield" || ps.m["Defined"] != "Self" {
			return refuse("Unearth$ shape")
		}
		ps.consume("Unearth")
		delete(ps.m, "Unearth")
		if err := lc.changeZoneMove(ps); err != nil {
			return err
		}
		lc.emit(lInstr{Op: "Unearth", Note: "haste until end of turn; exiled at the next end step or instead of leaving"})
		return nil
	}
	if done, err := lc.stackExileOptional(ps); done || err != nil {
		return err // lower_stackexile.go
	}
	if _, tt := ps.m["TargetType"]; (tt || ps.str("ValidTgts") == "Card.inZoneStack") && ps.str("Origin") == "Stack" {
		return lc.stackToZone(ps) // lower_modecost.go
	}
	if lc.breachStage == 1 {
		return lc.breachReturn(ps) // lower_repeatgravechoice.go
	}
	if strings.TrimSpace(ps.m["Origin"]) == "Battlefield,Stack" {
		return lc.permOrSpellMove(ps) // lower_permorspell.go
	}
	if so := sideboardOrigin(ps); so != sideNone {
		return lc.sideboardSearch(ps, so) // lower_sideboardsearch.go
	}
	dual := strings.TrimSpace(ps.str("Origin")) == "Graveyard,Exile"
	// a delayed program's return of the object its registration remembered
	// (The Guardian of Forever): a known-origin move that takes the object
	// from either zone, not a pick (dualDelayedReturn)
	dualDef := dual && dualDelayedReturn(ps)
	if dualDef {
		dual = false
	}
	gxTgt := false
	if _, has := ps.m["ValidTgts"]; dual && has {
		if err := lc.graveExileTarget(ps); err != nil { // lower_graveexiletgt.go
			return err
		}
		gxTgt, dual = true, false // a known-origin targeted move, not a pick
	}
	if dual && !everyRememberedOrImprinted(ps.str("ChangeType")) {
		return refuse("ChangeZone Origin$ Graveyard,Exile ChangeType$ %q without IsRemembered", ps.str("ChangeType"))
	}
	hg := strings.TrimSpace(ps.str("Origin")) == "Hand,Graveyard" // lower_handgravepick.go
	origin := zNone
	var err error
	bg := isOriginBattlefieldGrave(ps.str("Origin")) // lower_originmulti.go
	if bg {
		if err = lc.originBattlefieldGrave(ps); err != nil {
			return err
		}
		origin = zAny
	}
	lg := isOriginLibraryGrave(ps) // lower_libgrave.go
	if dualDef || gxTgt {
		ps.get("Origin") // read
		origin = zAny
	} else if lg {
		ps.get("Origin") // read
		origin = zLibrary
	} else if !dual && !hg && !bg {
		if origin, err = zoneOf(ps, "Origin", zNone); err != nil {
			return err
		}
	}
	dest, err := zoneOf(ps, "Destination", zNone)
	if err != nil {
		return err
	}
	if dest == zNone {
		return refuse("ChangeZone without Destination")
	}
	flags := []string{}
	if ps.is("Tapped", "True") {
		flags = append(flags, "Tapped")
	}
	if ps.is("RememberChanged", "True") {
		flags = append(flags, "Remember")
	}
	if v, ok := ps.get("ForgetChanged"); ok {
		// the moved object leaves the resolution's remembered set (gorge
		// eventForgetChanged); changeZone refuses it on any op that does not honor it
		if v != "True" {
			return refuse("ChangeZone ForgetChanged$ %q", v)
		}
		flags = append(flags, "ForgetChanged")
	}
	lki, lkiCtrl := false, false
	if v, ok := ps.get("RememberLKI"); ok {
		// gorge effChangeZone: the moved object joins the resolution's
		// remembered set (not the source's), its pre-move controller
		// captured (zone_change.go LKIObject)
		if v != "True" {
			return refuse("RememberLKI$ %q", v)
		}
		lki = true
		if !ps.is("RememberChanged", "True") {
			flags = append(flags, "Remember")
		}
		if lkiCtrl, err = lc.rememberLKIGuard(ps); err != nil {
			return err
		}
	}
	underOpp := false
	if gc, ok := ps.get("GainControl"); ok { // lower_hiddendefined.go: True, or ChosenPlayer on a Defined$ return
		switch {
		case gainControlTrue(gc) == nil:
			flags = append(flags, "UnderYourControl")
		case lc.gainControlChosen(ps, gc, dest, dual || hg):
			underOpp = true // the flag is added where the move becomes a Return (below)
		default:
			return gainControlTrue(gc)
		}
	}
	transformed := false
	if v, ok := ps.get("Transformed"); ok {
		if v != "True" {
			return refuse("ChangeZone Transformed$ %q", v)
		}
		transformed = true
	}
	if transformed {
		if dest != zBattlefield {
			return refuse("ChangeZone Transformed$ to %s", zoneLabel[dest])
		}
		switch {
		case dual:
			return refuse("Transformed$ on a graveyard/exile choice")
		case ps.m["ChangeType"] != "":
			return refuse("Transformed$ on a ChangeType$ move")
		case ps.m["Hidden"] != "":
			return refuse("Transformed$ with Hidden$")
		case ps.m["Reveal"] != "" || ps.m["Shuffle"] != "":
			return refuse("Transformed$ on a revealed/shuffled move")
		case ps.m["Duration"] != "":
			return refuse("Transformed$ with Duration$")
		}
	}
	var counters lValue
	var counterKind string
	if ct, ok := ps.get("WithCountersType"); ok {
		counterKind = ct
		if counters, err = lc.amount(ps, "WithCountersAmount", 1); err != nil {
			return err
		}
	}
	var override *lOverride
	if se, ok := ps.get("StaticEffect"); ok {
		o, err := lc.staticEffectOverride(se)
		if err != nil {
			return err
		}
		override = o
	}
	pos := 0 // library position: 0 top, -1 bottom
	if lp, ok := ps.get("LibraryPosition"); ok {
		if pos, err = strconv.Atoi(lp); err != nil {
			return refuse("LibraryPosition %q", lp)
		}
	}
	if raw, ok := ps.m["ChooseFromDefined"]; ok && !lc.digRememberedLibrary && strings.TrimSpace(raw) == "TriggeredCards" {
		return lc.dreadTriggeredPick(ps) // lower_dreadtriggerpick.go
	}
	if _, ok := ps.m["ChooseFromDefined"]; ok && lc.digRememberedLibrary {
		return lc.digChosenLibraryPick(ps, ps.m["ChooseFromDefined"]) // lower_digimprint.go
	}
	if ps.str("Defined") == "TopOfLibrary" {
		return lc.topLibraryMove(ps, origin, dest, pos, flags, counterKind, override) // lower_toplibrary.go
	}
	if strings.TrimSpace(ps.str("Defined")) == "ExiledWith" {
		return lc.exiledWithMove(ps, origin, dest, flags, counterKind, override) // lower_exw.go
	}
	// Hidden-zone selection: a library search or an untargeted choice.
	if hg {
		return lc.handGravePick(ps, dest, flags, counterKind, override)
	}
	if dual {
		ct, ok := ps.get("ChangeType")
		if !ok {
			return refuse("Origin$ Graveyard,Exile without ChangeType$")
		}
		if !ps.is("Hidden", "True") {
			return refuse("Origin$ Graveyard,Exile without Hidden$")
		}
		if dest != zHand && dest != zBattlefield && dest != zLibrary {
			return refuse("Origin$ Graveyard,Exile to %s", zoneLabel[dest])
		}
		if dest == zLibrary && pos != 0 {
			return refuse("Origin$ Graveyard,Exile to Library LibraryPosition$ %d", pos)
		}
		if ct == "" {
			return refuse("ChangeType$ empty")
		}
		f, err := lc.filter(ct)
		if err != nil {
			return err
		}
		n, err := lc.amount(ps, "ChangeNum", 1)
		if err != nil {
			return err
		}
		return lc.graveExilePick(ps, f, n, flags, counterKind, override, dest)
	}
	if handled, err := lc.handPutBack(ps, origin, dest, pos, flags, counterKind, override); handled || err != nil { // lower_handmove.go
		return err
	}
	if origin == zHand && dest == zExile && ps.m["ChangeType"] == "" && ps.m["Defined"] == "" &&
		ps.m["ValidTgts"] == "" && ps.m["DefinedPlayer"] == "" {
		// gorge changezone_params: an absent ChangeType$ is "Card", so a
		// bare hand-to-exile move is a pick from the controller's hand (All
		// Good Things . . .'s "exile a card from your hand"), never the
		// source
		m := make(map[string]string, len(ps.m)+1)
		for k, v := range ps.m {
			m[k] = v
		}
		m["ChangeType"] = "Card"
		ps = &params{m: m, used: ps.used}
	}
	if lc.imprintMox && origin == zHand && dest == zExile && ps.is("Imprint", "True") {
		// the imprint mox's pick: the kernel's reverse exiled-with record
		// is the imprint (lower_imprintmox.go)
		ps.consume("Imprint")
	}
	if ct, ok := ps.get("ChangeType"); ok {
		ownSearch, err := lc.targetPlayerOwnSearch(ps, origin, dest) // lower_partnerwith.go
		if err != nil {
			return err
		}
		if _, has := ps.m["ValidTgts"]; has && !ownSearch && !(origin == zHand && (dest == zExile || dest == zBattlefield || dest == zLibrary) ||
			(origin == zBattlefield || origin == zGraveyard) && (dest == zExile || dest == zHand) && ps.is("Hidden", "True") ||
			origin == zExile && dest == zGraveyard && ps.is("Hidden", "True")) {
			return refuse("ChangeType$ with ValidTgts$")
		}
		exwPick := origin == zExile && dest == zBattlefield && ps.is("Hidden", "True")
		lc.exwOK = exwPick          // a hidden pick out of exile is a resolution-time mover: ExiledWithSource reads the resolving source
		f, err := lc.pickFilter(ct) // lower_each.go
		lc.exwOK = false
		if err != nil {
			return err
		}
		n, err := lc.amount(ps, "ChangeNum", 1)
		if err != nil {
			return err
		}
		if f.Each {
			if err := eachPickGate(ps, origin, dest, n); err != nil { // lower_each.go
				return err
			}
		}
		if ps.is("Mandatory", "True") || origin == zLibrary && !effects.SearchStatesQuality(ct) {
			// a search that states no quality (Hour of Victory's "a card")
			// cannot fail to find: gorge zone_search.go forces Min to Max
			// (CR 701.23b)
			flags = append(flags, "Mandatory")
		}
		// ShuffleNonMandatory$ marks Forge's "if you do, shuffle" wording, not a
		// "you may shuffle": gorge's library-search shuffle is mandatory,
		// so the flag is read and lowers to nothing.
		ps.consume("ShuffleNonMandatory")
		ps.consume("Hidden")
		if origin == zLibrary {
			noShuffle := false
			shuffleTrue := false
			if v, ok := ps.get("Shuffle"); ok {
				// Shuffle$ False: no shuffle after the moves (gorge
				// zone_search's shuffle tail skips it; Flourishing
				// Bloom-Kin's three-link search)
				switch v {
				case "False":
					noShuffle = true
				case "True":
					shuffleTrue = true
				default:
					return refuse("search Shuffle$ %q", v)
				}
			}
			if v, ok := ps.get("NoShuffle"); ok {
				// NoShuffle$ True is Shuffle$ False: gorge shuffleLibrary
				// returns on cz.NoShuffle || cz.ShuffleFalse
				// (zone_library.go:21-26; compiled from the Forge params at
				// changezone_params.go:711-714). Claim Jumper
				if v != "True" {
					return refuse("search NoShuffle$ %q", v)
				}
				if shuffleTrue {
					// gorge would not shuffle either way; no corpus carrier
					// states both.
					return refuse("search Shuffle$ True with NoShuffle$ True")
				}
				noShuffle = true
			}
			if noShuffle {
				flags = append(flags, "NoShuffle")
			}
			if v, ok := ps.get("Reveal"); ok && v != "True" {
				return refuse("search Reveal$ %q", v) // a reveal is no state change
			}
			if v, ok := ps.get("NoReveal"); ok && v != "True" {
				// NoReveal$ True only suppresses the search's default reveal
				// Note (gorge zone_search.go:1473-1474), no state change
				// (Case of the Stashed Skeleton).
				return refuse("search NoReveal$ %q", v)
			}
			if v, ok := ps.get("NoLooking"); ok && v != "True" {
				// NoLooking$ True: the chooser picks without seeing the
				// rest of the library; gorge still offers the matching cards
				// (zone_search), no state change
				return refuse("search NoLooking$ %q", v)
			}
			if dest == zLibrary {
				if pos != 0 {
					return refuse("library-to-library search position %d", pos)
				}
				if !noShuffle {
					flags = append(flags, "ThenTop")
				}
			}
			if override != nil {
				return refuse("search with override")
			}
			arm, clear, err := lc.searchCounterArm(dest, counterKind, counters, n, f.Each)
			if err != nil {
				return err
			}
			// DefinedPlayer$: whose library is searched (lower_eachsearch.go)
			obj, err := lc.searchOwner(ps, arm != nil)
			if err != nil {
				return err
			}
			// Optional$ True/You gates the search on the fetching player's
			// yes/no before any pick (gorge zone_hand.go optionalConfirmMarker
			// and zone_search.go's confirmAction).
			if opt, ok := ps.get("Optional"); ok {
				switch strings.TrimSpace(opt) {
				case "True", "You":
					flags = append(flags, "Confirm")
				case "False", "":
				default:
					return refuse("ChangeZone Optional$ %q", opt)
				}
			}
			// DifferentNames$ True: no two found cards share a name (gorge
			// zone_search.go Group = name; kernel SrDifferentNames).
			if v, ok := ps.get("DifferentNames"); ok {
				if v != "True" {
					return refuse("ChangeZone DifferentNames$ %q", v)
				}
				flags = append(flags, "DifferentNames")
			}
			// OriginAlternative$: "search your
			// library and/or graveyard (hand)". gorge merges the alternative
			// into the origin set and poses one zone election before the pick
			// (zone_search.go originAlternativeZones); the kernel reads it
			// from SrAltOrigin. Only the Standard carriers: a Graveyard or
			// Hand alternative, or the Graveyard,Hand pair, to a Hand or
			// Battlefield destination, a search that states a quality, none
			// of the shuffle riders.
			if av, ok := ps.get("OriginAlternative"); ok {
				alt := strings.TrimSpace(av)
				// The Graveyard,Hand pair is Agency Outfitter's two-name
				// battlefield search and Say Its Name's one-card search by
				// name; every other list stays refused.
				pair := alt == "Graveyard,Hand"
				if alt != "Graveyard" && alt != "Hand" && !pair {
					return refuse("OriginAlternative$ %q", av)
				}
				switch {
				case dest != zHand && dest != zBattlefield:
					return refuse("OriginAlternative$ search to %s", zoneLabel[dest])
				case slices.Contains(flags, "Mandatory"):
					return refuse("OriginAlternative$ search with no stated quality or Mandatory$")
				case noShuffle || shuffleTrue:
					return refuse("OriginAlternative$ with Shuffle$/NoShuffle$")
				case obj.Kind != rYou:
					return refuse("OriginAlternative$ with DefinedPlayer$")
				case arm != nil:
					return refuse("OriginAlternative$ with WithCountersType$")
				case pair && !(dest == zBattlefield && (isConstN(n, 1) || isConstN(n, 2))):
					// the pair is only the two battlefield searches (Agency
					// Outfitter's two names, Say Its Name's one); the other
					// carriers are a later lowering.
					return refuse("OriginAlternative$ with ChangeNum$ %v", n)
				case !pair && !isConstN(n, 1):
					return refuse("OriginAlternative$ with ChangeNum$ %v", n)
				}
				flags = append(flags, "alt="+alt)
			}
			if lg {
				if flags, err = libGraveFlags(ps, dest, n, obj, arm != nil, flags); err != nil { // lower_libgrave.go
					return err
				}
			}
			if flags, err = shareLandType(ps, f, flags); err != nil { // lower_sharelandtype.go
				return err
			}
			if flags, err = searchDestAlt(ps, dest, obj, flags); err != nil { // lower_destalt.go
				return err
			}
			if dest == zBattlefield && !isConstN(n, 1) {
				// CR 303.4f: the kernel parks one found Aura for its enchant ask.
				for _, c := range f.Clauses {
					if c.Types == 0 && c.Subtype == "" && c.Name == "" || c.Types&tEnchantment != 0 {
						return refuse("search onto the battlefield for %v cards with a filter that may admit an Aura", n)
					}
				}
			}
			if arm != nil {
				// The found card enters with the rider's counters: arm the
				// entry, search, then clear (a failed or declined search
				// leaves nothing behind). dirac/ctr_search.go.
				lc.emit(*arm)
			}
			if err := lc.rememberSearcher(ps, obj, flags); err != nil { // lower_remembersearcher.go
				return err
			}
			lc.eachSearchLoop(obj) // lower_eachsearch.go
			lc.emit(lInstr{Op: "Search", Obj: obj, A: n, F: f, C: dest, CS: strings.Join(flags, "|")})
			if clear != nil {
				lc.emit(*clear)
			}
			return nil
		}
		// A WithCountersType$ rider lowers for the battlefield arm: the kind
		// travels as counters=<KIND> in CS and build_ab.go packs it into the
		// ChooseMove's B bits 4..11, which handGraveArm turns into exactly
		// one entry counter (gorge withCounterAmount defaults to 1). Any
		// other amount refuses (the kernel cannot arm it exactly), and
		// an unknown kind refuses by name. gorge ignores the rider on a
		// destination that cannot carry counters (counterDestination,
		// engine/effects/zone_hand.go), so the hand arm below drops it
		// exactly as gorge does.
		if counterKind != "" {
			if _, ok := counterKinds[counterKind]; !ok {
				return refuse("WithCountersType$ %q", counterKind)
			}
			if dest == zBattlefield && !isConstN(counters, 1) {
				return refuse("WithCountersAmount$ on a graveyard pick")
			}
		}
		if origin == zGraveyard && (dest == zExile || dest == zHand || dest == zBattlefield) && ps.is("AtRandom", "True") {
			return lc.randomGraveMove(ps, f, n, flags, counterKind, override, dest) // lower_randomgravemove.go
		}
		graveBF := dest == zBattlefield && override == nil && graveToBattlefieldFlags(flags)
		if origin == zGraveyard && (dest == zHand || graveBF) {
			for i := range f.Clauses {
				if f.Clauses[i].Owner != whoAny {
					continue
				}
				if f.Clauses[i].Ctrl == whoYou {
					// Off the battlefield a card's controller is its owner
					// (gorge events/apply_zone.go:818-821), so YouCtrl on a
					// graveyard card reads as "owned by you"
					// (effects/filter.go:79-81); the pick pool is the
					// game-wide graveyard, narrowed to your cards here
					// (Another Chance, Deadly Brew).
					f.Clauses[i].Owner = whoYou
					f.Clauses[i].Ctrl = whoAny
					continue
				}
				// An ownerless clause is gorge's game-wide graveyard pool
				// (zone_hidden.go gameWide: every player's graveyard in seat
				// order). The kernel's plain graveyard walk is the chooser's
				// own graveyard; only a GainControl$ pick onto the battlefield
				// (SrUnderYourControl) walks every graveyard
				// (gameGraveCands), and enters under the chooser as gorge
				// enters it under the resolving controller. Without
				// GainControl$ gorge enters an opponent's card under its
				// owner, which the kernel's plain pick does not do.
				if dest == zBattlefield && slices.Contains(flags, "UnderYourControl") {
					continue
				}
				return refuse("graveyard choice without an owner")
			}
			// Optional$ True/You: gorge hiddenPickConfirms asks the chooser
			// yes/no (hidden_pick_confirm) before the pick, even with an
			// empty pool; the kernel poses it as the library search does.
			if opt, ok := ps.get("Optional"); ok {
				switch strings.TrimSpace(opt) {
				case "True", "You":
					if ps.is("Mandatory", "True") {
						return refuse("graveyard choice Optional$ with Mandatory$ True")
					}
					flags = append(flags, "Confirm")
				case "False", "":
				default:
					return refuse("ChangeZone Optional$ %q", opt)
				}
			}
			// WithTotalCMC$ is the same cumulative budget a Dig reads; it
			// rides the pick filter's head (kop.Filter.PickSumMax) and the
			// kernel sums the hidden pick's chosen cards (State.pickFits).
			budget, hasBudget, err := pickTotalCMC(ps)
			if err != nil {
				return err
			}
			f = pickConstraintFilter(f, budget, hasBudget, false)
			cs := "from=Graveyard|" + strings.Join(flags, "|")
			if counterKind != "" && dest == zBattlefield {
				cs += "|counters=" + counterKind
			}
			lc.emit(lInstr{Op: "ChooseMove", Obj: lRef{Kind: rYou}, A: n, F: f, C: dest, CS: cs})
			return nil
		}
		if exwPick {
			return lc.exilePick(ps, f, n, flags, counterKind, override, dest) // lower_exw.go
		}
		if origin == zHand && (dest == zBattlefield || dest == zExile || dest == zLibrary && pos == 0) {
			return lc.handMove(ps, f, n, flags, counterKind, override, dest)
		}
		if origin == zBattlefield && (dest == zHand || dest == zExile) ||
			origin == zGraveyard && (dest == zHand || dest == zExile) && ps.is("Hidden", "True") ||
			origin == zExile && dest == zGraveyard && ps.is("Hidden", "True") {
			return lc.publicPick(ps, f, n, flags, counterKind, override, origin, dest)
		}
		if origin == zCommand && dest == zHand && ps.is("Hidden", "True") && strings.Contains(ct, "IsCommander") {
			// Command Beacon's "put your commander into your hand": lowered
			// to nothing on the assumption that no seat has a commander
			// (kop.W7IsCommander), so the command zone holds no other card
			// and gorge's pick over the empty pool moves nothing and poses no
			// decision; the cmd5 pool guard reads the counter
			noteCmdShortcut(cmdShortcutCommandBeacon)
			ps.consume("SelectPrompt")
			return nil
		}
		return refuse("ChangeType$ move %s->%s", zoneLabel[origin], zoneLabel[dest])
	}
	if _, ok := ps.m["Hidden"]; ok && !hiddenDefinedInert(ps) { // lower_hiddendefined.go
		return refuse("Hidden$ without ChangeType$")
	}
	definedMoveInert(ps, origin)                                                                                 // lower_handmove.go
	if handled, err := lc.thisAndTarget(ps, origin, dest, flags, counterKind, override); handled || err != nil { // lower_handmove.go
		return err
	}
	if !ps.is("Reveal", "True") && ps.is("Shuffle", "True") && dest == zLibrary && len(flags) == 0 && counterKind == "" && override == nil {
		return lc.shuffleInto(ps, origin)
	}
	if ps.is("Reveal", "True") || ps.is("Shuffle", "True") {
		return refuse("revealed/shuffled move")
	}
	if dur, ok := ps.get("Duration"); ok {
		if dur != "UntilHostLeavesPlay" || origin != zBattlefield || dest != zExile {
			return refuse("ChangeZone Duration %q", dur)
		}
		obj, err := lc.objOrTarget(ps, "", zBattlefield)
		if err != nil {
			return err
		}
		ctrs, err := lc.exileReturnCounters(ps) // lower_exilereturncounters.go
		if err != nil {
			return err
		}
		eu := lInstr{Op: "ExileUntilLeaves", Obj: obj}
		if ctrs > 0 {
			eu.A = cst(ctrs) // the return rider's +1/+1 counters (A unset = none)
		}
		lc.emit(eu)
		return nil
	}
	// A known-origin move: gorge's object path asks the chooser yes/no per
	// object (changeZoneOptionalConfirm); lowered below as an OpMayAsk
	// over the move for the one shape the guard mirrors exactly.
	optional := false
	if v, ok := ps.get("Optional"); ok {
		if strings.TrimSpace(v) != "True" {
			return refuse("ChangeZone Optional$ %q", v)
		}
		if _, ok := ps.get("Chooser"); ok {
			return refuse("ChangeZone Optional$ with Chooser$ on a known-origin move")
		}
		optional = true
	}
	tz := origin
	if tz == zNone || tz == zAny {
		tz = zBattlefield
	}
	obj, err := lc.objOrTarget(ps, "Self", tz)
	if err != nil {
		return err
	}
	if obj.Kind == rTriggered && origin != zBattlefield && origin != zNone {
		// The triggering card moved; the move acts on it in its new zone.
		obj.Kind = rTriggeredNew
	}
	if obj.Kind == rTarget && origin == zNone {
		return refuse("targeted ChangeZone without Origin")
	}
	self, err := lc.alsoSelf(ps, obj)
	if err != nil {
		return err
	}
	// Mandatory$ True on a targeted or Defined known-origin move is a no-op in
	// gorge (it reads cz.Mandatory only in the hand, hidden and search walks).
	if v, ok := ps.get("Mandatory"); ok {
		if v != "True" {
			return refuse("ChangeZone Mandatory$ %q", v)
		}
		if lc.optionalTarget() {
			return refuse("Mandatory$ with an optional target (gorge re-asks a declined up-to target at resolution; gorge finding)")
		}
	}

	if rt, ok := ps.get("RememberTargets"); ok {
		// gorge rememberChosenTargets: after the body, the chosen targets
		// join the remembered set (ForgetOtherTargets$ first forgets the
		// rest). Lowered only for a targeted battlefield exile, where the
		// kernel's ExileEx MvRemember remembers exactly the moved targets
		// (a legal battlefield target always moves), and a targeted
		// graveyard-to-hand return, where ReturnEx MvRemember remembers the
		// returned card (an illegal target does not move: CR 608.2b;
		// Cemetery Recruitment); with no target it is a no-op in gorge.
		if rt != "True" {
			return refuse("RememberTargets %q", rt)
		}
		if obj.Kind == rTarget {
			// The kernel's remembered set lives for one resolution
			// (resolve.go), gorge's persists on the source: the targets
			// must be read later in this chain (a flicker), not by
			// another ability (Journey to Nowhere's leaves trigger).
			if !lc.restReadsRemembered() && !lc.remPersistShape() { // lower_rempersist.go
				return refuse("RememberTargets read outside its own resolution")
			}
			if !(dest == zExile && tz == zBattlefield) && !(origin == zGraveyard && dest == zHand) {
				return refuse("RememberTargets on a %s->%s move", zoneLabel[origin], zoneLabel[dest])
			}
			if ps.is("ForgetOtherTargets", "True") {
				lc.emit(lInstr{Op: "ClearRemembered"})
			}
			if !strings.Contains(strings.Join(flags, "|"), "Remember") {
				flags = append(flags, "Remember")
			}
		}
	}
	if transformed {
		if origin != zGraveyard && origin != zExile {
			return refuse("Transformed$ from %s", zoneLabel[origin])
		}
		if lc.out.FaceMode != kop.FaceTransform {
			return refuse("Transformed$ on a layout the kernel does not turn (FaceMode %d)", lc.out.FaceMode)
		}
		switch obj.Kind {
		case rSelf:
		case rRemembered:
			if !lc.rememberedFromOwnSelfExile() {
				return refuse("Transformed$ Defined$ Remembered without this chain's own self-exile")
			}
		case rTriggeredNew:
			if ps.str("Defined") != "TriggeredCard" || ps.is("GainControl", "True") || !lc.selfDiesTrigger() { // lower_damageraise.go (TriggeredNewCardLKICopy and GainControl$ stay refused)
				return refuse("Transformed$ of %s", refNames[obj.Kind])
			}
		case rDelayedRemembered:
			if lc.ab == nil || !lc.ab.DelayedSelf { // lower_delayedobjects.go: only the source's own card turns over
				return refuse("Transformed$ of %s", refNames[obj.Kind])
			}
		default:
			return refuse("Transformed$ of %s", refNames[obj.Kind])
		}
		flags = append(flags, "Transformed")
	}
	in := lInstr{Obj: obj, CS: strings.Join(flags, "|")}
	switch dest {
	case zExile:
		in.Op = "Exile"
	case zHand:
		in.Op = "Bounce"
		// a remembered set from exile (Worlds Within Worlds' return) is
		// the Return walk: Bounce moves the set's first object only
		if origin == zGraveyard || origin == zExile && obj.Kind == rRemembered {
			in.Op = "Return"
			in.C = zHand
		}
	case zLibrary:
		in.Op = "ToLibrary"
		switch pos {
		case 0:
			in.C = 0
		case -1:
			in.C = 1
		case 2:
			// third from the top (gorge libraryOrderPlacementAt), read exactly
			// for the source leaving the battlefield (Braided Quipu); another
			// object or origin is lockstep-unverified and stays refused
			if !lc.libThirdOK(obj, origin) { // lower_libthird.go
				return refuse("library position 2 of %s from %s", refNames[obj.Kind], zoneLabel[origin])
			}
			in.C = kop.LibThird
		case 1:
			// second from the top is read only as AlternativeDecider$'s
			// primary (gorge zone_change.go asks the owner, then
			// libraryOrderPlacementAt(1)); a bare position 1 stays refused
			if _, ok := ps.get("AlternativeDecider"); !ok {
				return refuse("library position %d", pos)
			}
		default:
			return refuse("library position %d", pos)
		}
		if slices.Contains(flags, "Remember") && pos == -1 && origin == zGraveyard && obj.Kind == rTriggeredNew {
			in.C = kop.LibBottomRememberGrave // zone_tolibraryremember.go
		}
		if c, ok, err := ctrlPositionChoice(ps, origin, pos, obj); err != nil { // lower_destalt.go
			return err
		} else if ok {
			in.C = c
		}
		if ad, ok := ps.get("AlternativeDecider"); ok {
			if ad != "TargetedOwner" || ps.str("DestinationAlternative") != "Library" || ps.str("LibraryPositionAlternative") != "-1" || pos != 0 && pos != 1 {
				return refuse("alternative destination shape")
			}
			in.C = 3 // the owner chooses top or bottom
			if pos == 1 {
				in.C = 4 // the owner chooses second from the top or bottom
			}
		}
	case zBattlefield:
		in.Op = "Return"
		in.C = zBattlefield
		if origin == zHand || origin == zBattlefield {
			return refuse("move %s->Battlefield", zoneLabel[origin])
		}
		// Attacking$ True: the returned permanent enters attacking
		// (gorge attackingParam; kernel MvAttacking). Any other value is
		// a selector gorge resolves differently.
		if v, ok := ps.get("Attacking"); ok {
			if v != "True" {
				return refuse("ChangeZone Attacking$ %q", v)
			}
			if lc.optionalTarget() {
				return refuse("Attacking$ in a chain with an optional target (gorge re-asks a declined up-to target at resolution; gorge finding)")
			}
			in.CS = strings.TrimPrefix(in.CS+"|Attacking", "|")
		}
		if self {
			in.CS = strings.TrimPrefix(in.CS+"|AlsoSelf", "|")
		}
		if underOpp {
			if err := underOpponentShape(obj, self, counterKind, override, transformed, in.CS); err != nil { // lower_hiddendefined.go
				return err
			}
			in.CS = strings.TrimPrefix(in.CS+"|UnderOpponent", "|")
		}
		if at, ok := ps.get("AttachedTo"); ok {
			bearer, bearerF, err := lc.czAttachBearer(at, obj, origin) // lower_changezoneattach.go
			if err != nil {
				return err
			}
			if self || underOpp || counterKind != "" || override != nil || transformed || lki {
				return refuse("AttachedTo$ with AlsoSelf/UnderOpponent/counters/StaticEffect/Transformed/RememberLKI")
			}
			in.Obj2, in.F = bearer, bearerF
			in.CS = strings.TrimPrefix(in.CS+"|AttachTo", "|")
		}
	case zGraveyard:
		return refuse("ChangeZone to graveyard")
	default:
		return refuse("destination %s", zoneLabel[dest])
	}
	if underOpp && in.Op != "Return" {
		return refuse("GainControl$ ChosenPlayer on a %s", in.Op)
	}
	if lki && obj.Kind == rTarget {
		// the kernel reads the captured controller from the target's LKI
		// snapshot (g.tli, taken as a resolving effect moves a target off
		// the battlefield): kop.MvLKI on the Bounce/ExileEx that moves it
		switch {
		case in.Op == "Bounce" || in.Op == "Exile":
			in.CS = strings.TrimPrefix(in.CS+"|LKI", "|")
		case lkiCtrl:
			return refuse("RememberLKI$ controller read on a %s", in.Op)
		}
	}
	if fo, ok := ps.get("ForgetOtherRemembered"); ok {
		// gorge effChangeZone: Defined$ resolves first, then forgetOther
		// clears the remembered set, then the moves run (RememberChanged$
		// re-adding the moved objects): kop.MvForgetOther on ExileEx/ReturnEx
		if fo != "True" {
			return refuse("ForgetOtherRemembered$ %q", fo)
		}
		if in.Op != "Exile" && in.Op != "Return" {
			return refuse("ForgetOtherRemembered$ on a %s", in.Op)
		}
		in.CS = strings.TrimPrefix(in.CS+"|ForgetOther", "|")
	}
	if err := lc.changeZoneImprint(ps, &in); err != nil { // lower_imprintzone.go
		return err
	}
	if self && (in.Op != "Return" || in.C != zBattlefield || counterKind != "" || override != nil) {
		return refuse("ThisDefinedAndTgts$ Self on a %s move", in.Op)
	}
	if origin != zNone && origin != zAny {
		in.CS = strings.TrimPrefix(in.CS+"|from="+zoneLabel[origin], "|")
	}
	if dualDef {
		in.CS = strings.TrimPrefix(in.CS+"|from=Graveyard,Exile", "|") // the object may have moved from one zone to the other
	}
	if origin == zLibrary && (in.Op == "Bounce" || in.Op == "Return") {
		if v, ok := ps.get("NoLooking"); ok && v != "True" {
			// NoLooking$ True on a Defined$ fetch (the second half of a
			// split search): the chooser picks without seeing the rest of
			// the library, no state change, as on the search path
			return refuse("library fetch NoLooking$ %q", v)
		}
		if lc.fetchShuffleOwed(ps) {
			in.CS += "|ShuffleOwed"
		}
	}
	if counterKind != "" {
		if err := withCountersList(counterKind, in.Op, in.C); err != nil { // lower_counterlist.go
			return err
		}
		if in.Op != "Return" || in.C != zBattlefield && dest != zExile {
			if dest != zExile {
				return refuse("WithCounters on a %s move", in.Op)
			}
		}
		in.A = counters
		in.Note = "with counters " + counterKind
		in.CS += "|counters=" + counterKind
	}
	if optional {
		// The ask is read exactly for the shapes lower_optionalmove.go names:
		// the guard is the live object gorge's loop keeps. Another object set
		// or origin has its own skip rules (changeZoneMoveSkip,
		// sourceReturnedSincePush, a player target) and stays refused.
		guard, ok := lc.optionalMoveShape(obj, origin, in, override, self) // lower_optionalmove.go
		if !ok {
			return refuse("ChangeZone Optional$ on a %s %s->%s move", refNames[obj.Kind], zoneLabel[origin], zoneLabel[dest])
		}
		lc.emit(lInstr{Op: "MayAsk", C: guard, Skip: 1, Note: "you may move it"})
	}
	arm, bracket, err := lc.libPlaceTargets(ps, obj, in, pos) // lower_libplace.go
	if err != nil {
		return err
	}
	if bracket {
		lc.emitLibPlace(arm, in)
	} else {
		lc.emit(in)
	}
	if override != nil {
		if in.Op != "Return" || in.C != zBattlefield {
			return refuse("StaticEffect on a %s", in.Op)
		}
		lc.emitOverride(lRef{Kind: rRemembered}, override, "Permanent|enterWith")
		lc.ab.Prog[len(lc.ab.Prog)-1].Note = "applies to the returned object"
		lc.ab.Prog[len(lc.ab.Prog)-1].Obj = obj
	}
	return nil
}

// optionalTarget reports whether the ability being lowered has a target slot
// that may be empty (TargetMin$ 0, "up to N"). gorge re-asks the target of a
// ChangeZone at resolution when the announcement left it empty and, with the
// ask unanswered, resolves the Defined$ default (the source): a declined
// "up to one" target still moves the source (The Neutrinos, Koya; kernel
// lockstep matchup 1 seed 1). The lifted shapes beside such a slot are
// refused until gorge stops re-asking.
func (lc *lowerCtx) optionalTarget() bool {
	for i := range lc.ab.Targets {
		if t := &lc.ab.Targets[i]; t.Min.isConst() && t.Min.N == 0 {
			return true
		}
	}
	return false
}

// alsoSelf reads ThisDefinedAndTgts$: gorge appends the named extra objects
// after the chosen targets (zone_change.go thisDefinedAndTgtsTargets), so
// "Self" on a targeted known-origin move moves the target, then the source.
// Only Self on a single targeted object is lowered; ParentTarget (gorge
// resolves only the creature target; P15), TopOfLibrary and any other
// value are refused by name.
func (lc *lowerCtx) alsoSelf(ps *params, obj lRef) (bool, error) {
	v, ok := ps.get("ThisDefinedAndTgts")
	if !ok {
		return false, nil
	}
	if v != "Self" {
		return false, refuse("ThisDefinedAndTgts$ %q", v)
	}
	if obj.Kind != rTarget {
		return false, refuse("ThisDefinedAndTgts$ Self without a target")
	}
	return true, nil
}

// staticEffectOverride lowers a ChangeZone StaticEffect$ SVar: "the returned
// object is an X in addition to its other types (and has keyword)".
func (lc *lowerCtx) staticEffectOverride(name string) (*lOverride, error) {
	body := lc.face.SVars[name]
	ss, ok := cards.ParseStaticLines(body)
	if !ok || len(ss) != 1 || ss[0].Mode != "Continuous" {
		return nil, refuse("StaticEffect %q", body)
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode")
	if sp.str("Affected") != "Card.IsRemembered" {
		return nil, refuse("StaticEffect affected %q", sp.str("Affected"))
	}
	o := &lOverride{}
	if t, ok := sp.get("AddType"); ok {
		for _, w := range strings.Split(t, "&") {
			w = strings.TrimSpace(w)
			if b, ok := typeAddBit(w); ok {
				// a card type in addition (gorge layer 4 AddType$ without
				// RemoveCardTypes$: Bronzehide Lion's kin)
				o.AddTypes |= b
				continue
			}
			switch w {
			case "Plains", "Island", "Swamp", "Mountain", "Forest":
				// CR 305.6: its mana ability is not granted by an override
				return nil, refuse("StaticEffect adds basic land type %q", w)
			}
			if !lc.known(w) {
				return nil, refuse("StaticEffect type %q", w)
			}
			o.AddSubtypes = append(o.AddSubtypes, w)
		}
	}
	pS, okP := sp.get("SetPower")
	tS, okT := sp.get("SetToughness")
	if okP || okT {
		// base P/T in layer 7b (gorge StaticSet), both halves given
		pn, e1 := strconv.Atoi(pS)
		tn, e2 := strconv.Atoi(tS)
		if !okP || !okT || e1 != nil || e2 != nil {
			return nil, refuse("StaticEffect set P/T %q/%q", pS, tS)
		}
		o.HasSetPT, o.SetP, o.SetT = true, pn, tn
	}
	if k, ok := sp.get("AddKeyword"); ok {
		ks, err := pumpKeywords(k)
		if err != nil {
			return nil, err
		}
		o.AddKw = ks
	}
	if c, ok := sp.get("AddColor"); ok {
		// "is a black Zombie in addition to its other colors" (Rise from
		// the Grave): gorge parseStaticEffectGrant AddColor$ adds in layer 5
		// (CR 613.1e), the record's AddColors; an empty set refuses
		cols, ok := colorWordSet(c)
		if !ok || cols == "" {
			return nil, refuse("StaticEffect AddColor$ %q", c)
		}
		o.AddColors = cols
	}
	if sp.is("RemoveCardTypes", "True") {
		// "it's an <type> (it's no longer a creature)": the added types
		// replace the card types (the override static's RemoveCardTypes$
		// reading)
		if o.AddTypes == 0 {
			return nil, refuse("StaticEffect RemoveCardTypes$ without a card type")
		}
		o.HasSetTypes, o.SetTypes, o.AddTypes = true, o.AddTypes, 0
		o.HasSetSubtypes, o.SetSubtypes, o.AddSubtypes = true, o.AddSubtypes, nil
		if err := o.keepSubtypes(strings.Split(sp.str("AddType"), "&")); err != nil {
			return nil, err
		}
	}
	if v, ok := sp.get("RemoveAllAbilities"); ok {
		// "it loses all abilities" (gorge parseStaticEffectGrant
		// RemoveAllAbilities$; one LAbilities effect with the keyword
		// grants of the same body, so the record's AddKw survives the loss)
		if v != "True" {
			return nil, refuse("StaticEffect RemoveAllAbilities$ %q", v)
		}
		o.LoseAbilities = true
	}
	if left := sp.unread(); len(left) > 0 {
		return nil, refuse("StaticEffect unread %v", left)
	}
	return o, nil
}

// emitOverride emits the override-record ops for o on obj.
func (lc *lowerCtx) emitOverride(obj lRef, o *lOverride, dur string) {
	in := lInstr{Op: "Override", Obj: obj, CS: dur}
	in.Note = overrideString(o)
	lc.out.overrides = append(lc.out.overrides, o)
	in.C = len(lc.out.overrides) - 1
	lc.emit(in)
	if o.MustBeBlocked {
		lc.emit(lInstr{Op: "MustBeBlocked", Obj: obj}) // animateHiddenKeywords
	}
}

func overrideString(o *lOverride) string {
	var p []string
	if o.HasSetPT {
		if o.LivePT {
			p = append(p, "livept")
		}
		if o.PVal.Kind != vNone || o.TVal.Kind != vNone {
			p = append(p, "pt="+o.PVal.String()+"/"+o.TVal.String())
		} else {
			p = append(p, "pt="+strconv.Itoa(o.SetP)+"/"+strconv.Itoa(o.SetT))
		}
		if o.KeepP {
			p = append(p, "keepP")
		}
		if o.KeepT {
			p = append(p, "keepT")
		}
	}
	if o.PTDelta {
		p = append(p, "ptdelta="+o.PVal.String()+"/"+o.TVal.String())
	}
	if o.HasSetTypes {
		p = append(p, "types="+strconv.Itoa(int(o.SetTypes)))
	}
	if o.AddTypes != 0 {
		p = append(p, "addtypes="+strconv.Itoa(int(o.AddTypes)))
	}
	if o.HasSetSubtypes {
		p = append(p, "subtypes="+strings.Join(o.SetSubtypes, ","))
	}
	if len(o.AddSubtypes) > 0 {
		p = append(p, "addsub="+strings.Join(o.AddSubtypes, ","))
	}
	if o.RemoveCreatureSub {
		p = append(p, "rmcreaturesub")
	}
	if o.AllCreatureTypes {
		p = append(p, "allcreaturetypes")
	}
	if o.HasSetColors {
		p = append(p, "colors="+o.SetColors)
	}
	if o.NoCost {
		p = append(p, "nocost")
	}
	if o.AddColors != "" {
		p = append(p, "addcolors="+o.AddColors)
	}
	if o.RemoveLegendary {
		p = append(p, "nonlegendary")
	}
	if o.GrantStatics != 0 {
		p = append(p, "grantstatics="+strconv.Itoa(int(o.GrantStatics)))
	}
	if o.LoseAbilities {
		p = append(p, "loseabilities")
	}
	if len(o.RemoveKw) > 0 {
		p = append(p, "removekw="+strings.Join(o.RemoveKw, ","))
	}
	if len(o.AddKw) > 0 {
		p = append(p, "kw="+strings.Join(o.AddKw, "&"))
	}
	for _, g := range o.GrantAbility {
		p = append(p, "grant="+strconv.Itoa(g))
	}
	return "override{" + strings.Join(p, " ") + "}"
}

func (lc *lowerCtx) changeZoneAll(ps *params) error {
	bracket, err := libPlaceAll(ps) // lower_libplace.go
	if err != nil || !bracket {
		if err != nil {
			return err
		}
		return lc.changeZoneAllSweep(ps)
	}
	lc.emit(lInstr{Op: "LibPlace", C: kop.PlaceArmTop, Note: "count the cards this sweep puts into libraries"})
	if err := lc.changeZoneAllSweep(ps); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "LibPlace", C: kop.PlaceAsk, Note: "each owner of two or more orders them (CR 401.4)"})
	return nil
}

func (lc *lowerCtx) changeZoneAllSweep(ps *params) error {
	if lc.digImprinted && ps.is("RandomOrder", "True") {
		return lc.digImprintBottom(ps) // lower_digimprint.go
	}
	if lc.playLibrary && ps.is("RandomOrder", "True") {
		return lc.playLibraryBottom(ps) // lower_playlibrary.go
	}
	if _, ok := ps.get("Transformed"); ok {
		return refuse("ChangeZoneAll Transformed$")
	}
	if strings.TrimSpace(ps.str("Origin")) == "Graveyard,Exile" && !everyRemembered(ps.str("ChangeType")) {
		return refuse("ChangeZoneAll Origin$ Graveyard,Exile ChangeType$ %q without IsRemembered", ps.str("ChangeType"))
	}
	if strings.TrimSpace(ps.str("Origin")) == "Battlefield,Graveyard" {
		return lc.originBattleGrave(ps) // lower_changezoneall.go
	}
	if strings.TrimSpace(ps.str("Origin")) == "Library,Sideboard" {
		return lc.originLibrarySideboard(ps) // lower_sideboardsearch.go
	}
	lc.exwOK = true // a ChangeZoneAll sweep is a resolution-time mover: ExiledWithSource reads the resolving source
	f, err := lc.changeZoneAllFilter(ps)
	lc.exwOK = false
	if err != nil {
		return err
	}
	dual := strings.TrimSpace(ps.str("Origin")) == "Graveyard,Exile"
	if o := strings.TrimSpace(ps.str("Origin")); o == "Graveyard,Hand" || o == "Hand,Graveyard" {
		return lc.originGraveHand(ps) // lower_changezoneall.go
	}
	origin := zNone
	if !dual {
		if origin, err = zoneOf(ps, "Origin", zNone); err != nil {
			return err
		}
	}
	dest, err := zoneOf(ps, "Destination", zNone)
	if err != nil {
		return err
	}
	if dest == zNone || !dual && origin == zNone {
		return refuse("ChangeZoneAll zones")
	}
	if dual && dest != zHand && dest != zBattlefield {
		return refuse("ChangeZoneAll Origin$ Graveyard,Exile to %s", zoneLabel[dest])
	}
	from := zoneLabel[origin]
	if dual {
		from = "Graveyard,Exile"
	}
	in := lInstr{Op: "ChangeZoneAll", F: f, C: dest, CS: "from=" + from}
	if vt, ok := ps.get("ValidTgts"); ok {
		pf, err := lc.filter(vt)
		if err != nil {
			return err
		}
		if len(pf.Clauses) != 0 {
			return refuse("ChangeZoneAll object target")
		}
		lt := lTarget{F: pf}
		if err := lc.playerTargetRange(ps, &lt); err != nil { // lower_handmove.go
			return err
		}
		t := lc.addTarget(lt)
		in.Obj = lRef{Kind: rTarget, N: t}
		in.Note = "only the target player's zone"
	} else if err := lc.czallDefinedScope(ps, &in); err != nil { // lower_czalldefined.go
		return err
	}
	if ps.is("GainControl", "True") {
		in.CS += "|UnderYourControl"
	}
	if rc, ok := ps.get("RememberChanged"); ok {
		// each moved object joins the remembered set, in move order (gorge
		// effChangeZoneAll emitMove; kop.MvRemember)
		if rc != "True" {
			return refuse("ChangeZoneAll RememberChanged$ %q", rc)
		}
		in.CS += "|Remember"
	}
	if err := lc.czallParams3(ps, &in, origin, dest); err != nil { // lower_changezoneall.go
		return err
	}
	if _, ok := ps.m["Tapped"]; ok && dest == zBattlefield && ps.is("Tapped", "True") {
		in.CS += "|Tapped" // kop.MvTapped: each enters tapped
	}
	if err := lc.changeZoneAllRiders(ps, &in, origin, dest, dual); err != nil {
		return err
	}
	if err := lc.changeZoneAllAtEOT(ps, &in, origin, dest, dual); err != nil {
		return err
	}
	if _, ok := ps.m["Shuffle"]; ok {
		if in.A.Kind != vNone || strings.Contains(in.CS, "|static=") || strings.Contains(in.CS, "|atEOT=") {
			return refuse("ChangeZoneAll entry riders with Shuffle$")
		}
		return lc.changeZoneAllShuffle(ps, in, origin, dest)
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) dig(ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "DigNum", 1)
	if err != nil {
		return err
	}
	var take lValue
	switch cn := ps.str("ChangeNum"); cn {
	case "All":
		take = cst(-1)
	case "Any":
		take = cst(-2)
	case "":
		take = cst(1)
	default:
		if take, err = lc.value(cn); err != nil {
			return err
		}
	}
	dest, err := zoneOf(ps, "DestinationZone", zHand)
	if err != nil {
		return err
	}
	dest2, err := zoneOf(ps, "DestinationZone2", zLibrary)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Dig", Obj: who, A: n, B: take, C: dest}
	var flags []string
	flags = append(flags, "rest="+zoneLabel[dest2])
	if cv, ok := ps.get("ChangeValid"); ok {
		if in.F, err = lc.filter(cv); err != nil {
			return err
		}
	}
	// WithTotalCMC$/WithDifferentPowers$ ride the pick filter's head; the
	// kernel narrows the candidates and the running take (State.pickFits).
	budget, hasBudget, err := pickTotalCMC(ps)
	if err != nil {
		return err
	}
	distinct, err := digDifferentPowers(ps)
	if err != nil {
		return err
	}
	if err := digBoundShape(who, take, hasBudget, distinct); err != nil { // lower_digparams.go
		return err
	}
	in.F = pickConstraintFilter(in.F, budget, hasBudget, distinct)
	forgetOther := false
	if v, ok := ps.get("ForgetOtherRemembered"); ok {
		// gorge effDig copies the pre-clear candidates, then clears the
		// resolution's remembered set before the walk (dig.go). The kernel
		// walks the live set with no pre-clear snapshot, so a ChangeValid$
		// that reads it refuses by name.
		if v != "True" {
			return refuse("Dig ForgetOtherRemembered$ %q", v)
		}
		if strings.Contains(ps.m["ChangeValid"], "Remembered") || filterReadsRemembered(in.F) {
			return refuse("Dig ForgetOtherRemembered$ with a ChangeValid$ that reads the remembered set")
		}
		forgetOther = true
	}
	if lp, ok := ps.get("LibraryPosition"); ok {
		if dest != zLibrary && dest2 != zLibrary {
			return refuse("LibraryPosition without a library destination")
		}
		flags = append(flags, "pos="+lp)
	}
	var restTop bool
	if _, d2 := ps.m["DestinationZone2"]; !d2 && strings.TrimSpace(ps.m["SkipReorder"]) == "True" {
		// gorge effDig's rest(): SkipReorder$ leaves the remainder in place
		// before LibraryPosition2$ is read, so the position is moot (the
		// Undercity's Throne of the Dead Three)
		ps.consume("LibraryPosition2")
	} else if restTop, err = digRestTop(ps, n, take, dest2); err != nil {
		return err
	}
	if v, ok := ps.get("SkipReorder"); ok {
		// gorge effDig: SkipReorder$ True leaves the untaken cards where
		// they are, on top in their order (Knight-Errant of Eos, which
		// then shuffles); beside a DestinationZone2$ it is unread there
		if _, d2 := ps.m["DestinationZone2"]; v != "True" || d2 || ps.is("RestRandomOrder", "True") {
			return refuse("SkipReorder %q beside a remainder placement", v)
		}
		restTop = true
	}
	if restTop {
		flags = append(flags, "RestTop")
	}
	noReveal, err := digNoReveal(ps)
	if err != nil {
		return err
	}
	if err := digNoLooking(ps, who); err != nil { // lower_digparams.go
		return err
	}
	for _, k := range []string{"Optional", "RestRandomOrder", "ForceRevealToController", "Reveal", "RememberChanged"} {
		if ps.is(k, "True") && !(k == "Reveal" && noReveal) {
			flags = append(flags, k)
		}
	}
	if ps.is("PromptToSkipOptionalAbility", "True") || ps.str("OptionalAbilityPrompt") != "" {
		// gorge effDig's promptToSkipOptional: either spelling names the same
		// optional take as Optional$ True (the take is a 0..N election, and
		// with ChangeNum$ All a yes/no), which the kernel's Optional flag
		// already carries. The corpus writes it only beside Optional$ True.
		if !ps.is("Optional", "True") {
			return refuse("Dig optional prompt without Optional$ True")
		}
		ps.consume("PromptToSkipOptionalAbility", "OptionalAbilityPrompt")
	}
	if dest == zBattlefield && ps.is("Tapped", "True") {
		flags = append(flags, "Tapped")
	}
	// gorge applyMoveFaceDown (zone_change.go:1286-1335)
	// stamps a face-down may-look exile only for ExileFaceDown$ AND
	// WithMayLook$ with an exile destination; every other combination is
	// not read exactly (a blind face-down exile, a look on a non-exile move)
	fd, look := ps.is("ExileFaceDown", "True"), ps.is("WithMayLook", "True")
	blind := fd && !look && dest == zExile && (lc.repeatBlindDig(ps) || lc.blindDigShape(ps)) // lower_blinddig.go
	if fd != look && !blind || fd && dest != zExile {
		return refuse("Dig ExileFaceDown$ shape")
	}
	if fd {
		flags = append(flags, "ExileFaceDown")
	}
	if blind && lc.blind != nil && lc.blind.exilerOther {
		flags = append(flags, "ExilerOther")
	}
	imprintSet, err := lc.digImprint(ps, dest, take) // lower_digparams.go
	if err != nil {
		return err
	}
	in.CS = strings.Join(flags, "|")
	if forgetOther {
		// both Dig paths clear before the walk; DigRemember then re-adds the
		// taken cards (Primal Surge)
		lc.emit(lInstr{Op: "ClearRemembered"})
	}
	if ct, ok := ps.get("WithCounters"); ok && lc.name == undercityKey {
		// the taken card enters with counters (CR 122.6;
		// the Undercity's Throne). gorge effDig reads neither param
		// (cr-dig-withcounters); the kernel plays the card text.
		kind, err := counterKind(strings.TrimSpace(ct))
		if err != nil {
			return err
		}
		amt, err := lc.amount(ps, "WithCountersAmount", 1)
		if err != nil {
			return err
		}
		if dest != zBattlefield || !take.isConst() || take.N != 1 || !amt.isConst() || amt.N < 1 {
			return refuse("Dig WithCounters$ beside a %s take of %s", zoneLabel[dest], take)
		}
		lc.emit(lInstr{Op: "DigCounters", A: amt, C: int(kind)})
		lc.emit(in)
		lc.emit(lInstr{Op: "DigCounters", A: cst(0)})
		return nil
	}
	lc.emit(in)
	if imprintSet {
		lc.emit(lInstr{Op: "DigImprint"}) // the dig's takes join the imprinted set
	}
	return nil
}

// filterReadsRemembered reports whether a compiled filter reads the
// resolution's remembered set (Card.IsRemembered). A Dig's candidates come
// from a pre-clear snapshot in gorge, which the kernel cannot reproduce for
// such a filter.
func filterReadsRemembered(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags&fRemembered != 0 {
			return true
		}
	}
	return false
}

func (lc *lowerCtx) digUntil(ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	f, err := lc.filter(ps.str("Valid"))
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	found, err := zoneOf(ps, "FoundDestination", zNone)
	if err != nil {
		return err
	}
	rev, err := zoneOf(ps, "RevealedDestination", zNone)
	if err != nil {
		return err
	}
	in := lInstr{Op: "DigUntil", Obj: who, A: n, F: f, C: found, CS: "revealed=" + zoneLabel[rev]}
	if found == zBattlefield {
		// The kernel's battlefield arm enters the found card plainly: only
		// a Valid$ that cannot admit an Aura (no bearer choice, CR 303.4f;
		// handMove's rule: a typed clause without Enchantment).
		for _, c := range f.Clauses {
			if c.Types == 0 || c.Types&tEnchantment != 0 {
				return refuse("DigUntil onto the battlefield of a Valid$ that may admit an Aura")
			}
		}
		if f.Players != 0 {
			return refuse("DigUntil Valid$ with players")
		}
		if ps.is("Tapped", "True") {
			in.CS += "|Tapped"
		}
	}
	if v, ok := ps.get("Attacking"); ok {
		// gorge effects/diguntil.go:176-184: the ChangeZone rider on the found move
		if v != "True" || found != zBattlefield {
			return refuse("DigUntil Attacking$ %q to %s", v, zoneLabel[found])
		}
		in.CS += "|Attacking"
	}
	ps.get("NoPutDesc") // descriptive (gorge dig_params.go:226), no reader
	if p, ok := ps.get("RevealedLibraryPosition"); ok {
		in.CS += "|pos=" + p
	}
	if ps.is("RevealRandomOrder", "True") {
		in.CS += "|RandomOrder"
	}
	if found == zNone {
		// An absent FoundDestination$: the found card goes with the
		// revealed pile (gorge effects/diguntil.go).
		in.C = rev
		in.CS += "|FoundWithRevealed"
	}
	if ps.is("ImprintFound", "True") {
		if ps.is("RememberRevealed", "True") {
			return lc.digUntilImprint(ps, in, found, rev) // lower_digimprint.go
		}
		return lc.digImprintFound(ps, in, found, rev) // lower_digimprint.go
	}
	if ps.is("RememberRevealed", "True") {
		return lc.digRememberedRevealed(ps, in, found, rev) // lower_digimprint.go
	}
	if v, ok := ps.get("RememberFound"); ok {
		// gorge effDigUntil: the found cards replace the resolution's
		// remembered set; RememberRevealed$ beside it stays unread
		if v != "True" {
			return refuse("RememberFound %q", v)
		}
		in.CS += "|RememberFound"
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) peekAndReveal(ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "PeekAmount", 1)
	if err != nil {
		return err
	}
	has := func(k string) bool { _, ok := ps.m[k]; return ok }
	if !has("NoReveal") && !has("RevealOptional") && !has("RevealValid") && !has("RememberPeeked") {
		// gorge effReveal's public PeekAndReveal: the top PeekAmount$ cards
		// are revealed with no ask (a public Note, no look_ack) and, with
		// RememberRevealed$, join the resolution's remembered set (appended,
		// not reset). NoPeek$ is unread in gorge, so it is consumed here.
		ps.consume("NoPeek")
		cs := "Public"
		if ps.is("RememberRevealed", "True") {
			cs += "|Remember"
		} else if has("RememberRevealed") {
			return refuse("PeekAndReveal RememberRevealed %q", ps.str("RememberRevealed"))
		}
		lc.emit(lInstr{Op: "Peek", Obj: who, A: n, CS: cs})
		return nil
	}
	if who.Kind == rEachPlayer && (has("RevealOptional") || has("RevealValid") || has("RememberPeeked")) {
		// only the public reveal walks every player; the may-reveal and
		// remembered-look arms read one player's library
		return refuse("PeekAndReveal over each player outside the public reveal")
	}
	if ps.is("NoReveal", "True") && !has("RememberPeeked") && !has("RevealOptional") && !has("RevealValid") {
		// the bare look that remembers nothing (Mishra's Bauble): gorge's
		// look_ack when the library holds a card, no other effect
		lc.emit(lInstr{Op: "LookAck", Obj: who, A: n, CS: "Library"})
		return nil
	}
	if ps.is("RevealOptional", "True") && has("RevealValid") && ps.is("RememberRevealed", "True") &&
		!has("NoReveal") && !has("RememberPeeked") && !has("Look") && !has("RevealType") &&
		!has("RevealDefined") && !has("Random") {
		// gorge effReveal's may-reveal arm: the
		// top PeekAmount$ cards narrowed by RevealValid$; with a match the
		// library's owner is asked yes/no and a yes reveals and remembers
		// it. Only the single-card window is read exactly.
		if n.Kind != vConst || n.N != 1 {
			return refuse("PeekAndReveal RevealOptional$ shape")
		}
		f, err := lc.filter(ps.str("RevealValid"))
		if err != nil {
			return err
		}
		ps.consume("NoPeek")
		lc.emit(lInstr{Op: "Peek", Obj: who, A: n, F: f, CS: "MayReveal|Remember"})
		return nil
	}
	if has("RevealOptional") || has("RevealValid") {
		return refuse("PeekAndReveal RevealOptional$ shape")
	}
	if !ps.is("NoReveal", "True") || !ps.is("RememberPeeked", "True") {
		return refuse("PeekAndReveal shape")
	}
	lc.emit(lInstr{Op: "Peek", Obj: who, A: n, CS: "Remember"})
	return nil
}

func (lc *lowerCtx) twoPiles(ps *params) error {
	if ps.str("Zone") == "Battlefield" {
		return lc.twoPilesSac(ps)
	}
	return lc.twoPilesRemembered(ps) // lower_twopiles.go
}

func (lc *lowerCtx) chooseCard(ps *params) error {
	if each, ok := ps.get("ChooseEach"); ok {
		// Liliana's -9: each opponent chooses a permanent they control of
		// each permanent type; the SacrificeAll that follows takes the rest.
		if ps.str("Choices") != "Permanent" || ps.str("ControlledByPlayer") != "Chooser" || ps.str("Defined") != "Opponent" {
			return refuse("ChooseEach shape")
		}
		ps.consume("Mandatory", "Reveal")
		lc.emit(lInstr{Op: "ChooseEachType", Obj: lRef{Kind: rEachOpp}, CS: each})
		return nil
	}
	// A ValidTgts$ chooser must be read before Choices$: the Choices$ filter
	// (TargetedPlayerCtrl) reads the target slot this adds (lower_choosecardtarget.go).
	chooser, targeted, err := lc.chooseCardTargetChooser(ps)
	if err != nil {
		return err
	}
	_, ownTgt := ps.get("ValidTgts")
	zone, err := zoneOf(ps, "ChoiceZone", zBattlefield)
	if err != nil {
		return err
	}
	lc.attachedTgtOK = !ownTgt && lc.ab != nil && len(lc.ab.Targets) > 0 // lower_filterreferents.go
	// a choice among the exile cards is made as the ability resolves, so
	// ExiledWithSource reads the resolving source (gorge cardChoices'
	// filter context, exiledBySourceAt with the trigger's fire incarnation;
	// kernel exwFlag): Borg Cube's and Assimilation Assessor's "a creature
	// card exiled with it" (CR 607.2a)
	lc.exwOK = zone == zExile
	optional := !ps.is("Mandatory", "True")
	choices := ps.str("Choices")
	if v := ps.str("ControlledByPlayer"); v != "" && !optional {
		// the kernel evaluates a mandatory ChooseCard from the resolving
		// controller too, but no lowered mandatory shape applies the
		// restriction: applying it only under optional would silently draw
		// the candidates from every seat, so refuse instead.
		return refuse("mandatory ChooseCard ControlledByPlayer$ %q", v)
	}
	if cbp := ps.str("ControlledByPlayer"); cbp != "" && optional {
		// gorge controlledByChoicePlayer Chooser: the pick's pool is the
		// chooser's own objects. The kernel evaluates a ChooseCard's filter
		// from the chooser's perspective (cardCands' p), so a YouCtrl clause
		// is the chooser's control here (Flaming Gambit).
		if cbp != "Chooser" {
			return refuse("optional ChooseCard ControlledByPlayer$ %q", cbp)
		}
		choices = addFilterProp(choices, "YouCtrl")
		ps.consume("ControlledByPlayer")
	}
	f, err := lc.filter(choices)
	lc.attachedTgtOK, lc.exwOK = false, false
	if err != nil {
		return err
	}
	if !optional {
		// A mandatory ChooseCard's chooser is the resolving controller and
		// its pick count is one (gorge chooseCardChoosers, choiceBounds max
		// default 1). ChoiceTitle$ is display-only (the decision's Prompt),
		// so any value reads; Amount$ 1 and Defined$ You are exactly the
		// defaults, and anything else stays refused by name.
		ps.consume("ChoiceTitle")
		if v := ps.str("Amount"); v != "" && v != "1" {
			return refuse("ChooseCard Amount$ %q", v)
		}
		if v := ps.str("Defined"); v != "" && v != "You" {
			return refuse("ChooseCard Defined$ %q", v)
		}
	}
	if optional && zone == zLibrary {
		return lc.chooseLibraryUpTo(ps, f)
	}
	fromDefined := false
	if optional {
		// the optional battlefield path: ChoiceTitle$ is display only (as
		// the mandatory path's), Defined$ You is the default chooser, and
		// MinAmount$ 0 is what no Mandatory$ already means (gorge
		// choiceBounds: up to one); any other value refuses
		ps.consume("ChoiceTitle")
		if v := ps.str("Defined"); v != "" && v != "You" {
			if v != "TargetedOrController" {
				return refuse("optional ChooseCard Defined$ %q", v)
			}
			// gorge definedAliases spells TargetedOrController as
			// TargetedController: a player target is that player, an object
			// target its controller (lower_effect.go's rTargetController).
			// The referent is the chain's target, not this pick's.
			t := lc.lastTarget()
			if t < 0 {
				return refuse("optional ChooseCard Defined$ TargetedOrController without an earlier target")
			}
			chooser = lRef{Kind: rTargetController, N: t}
			fromDefined = true
		}
		if v := ps.str("MinAmount"); v != "" && v != "0" {
			return refuse("optional ChooseCard MinAmount$ %q", v)
		}
		ps.consume("Defined", "MinAmount")
	}
	if optional && (zone != zBattlefield || ps.str("Amount") != "" && ps.str("Amount") != "1") {
		return refuse("optional ChooseCard")
	}
	if !targeted && !fromDefined {
		chooser = lRef{Kind: rYou}
	}
	in := lInstr{Op: "ChooseCard", Obj: chooser, F: f, C: zone}
	if optional && ps.is("ForgetOtherRemembered", "True") {
		return refuse("optional ChooseCard with ForgetOtherRemembered")
	}
	if optional {
		// gorge choiceBounds: no Mandatory$ True is "up to one" (Min 0),
		// a Done beside the cards (Gix's Command)
		in.CS = "Optional"
	}
	if ps.is("ForgetOtherRemembered", "True") {
		in.CS = "ForgetOthers"
	}
	lc.emit(in)
	return nil
}

// chooseLibraryUpTo lowers an optional ChooseCard among the library cards the
// ability's PeekAndReveal just looked at (Thunderous Debut): gorge asks one
// KChoose, Min 0 and Max min(Amount$, candidates), over the remembered cards
// in the library (choiceBounds, effChooseCard). The kernel holds the
// remembered set as the Peek's window of the top cards (A = that amount,
// B = Amount$), so the shape is read only straight after a Remember-only Peek
// of the chooser's own library and an IsRemembered choice filter; anything
// else refuses.
func (lc *lowerCtx) chooseLibraryUpTo(ps *params, f *lFilter) error {
	max := 1
	if v := ps.str("Amount"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 2 {
			return refuse("optional ChooseCard Amount$ %q from the library", v)
		}
		max = n
	}
	if d := ps.str("Defined"); d != "" && d != "You" {
		return refuse("optional ChooseCard Defined$ %q from the library", d)
	}
	for _, k := range []string{"ForgetOtherRemembered", "AtRandom", "MinAmount", "WithTotalPower", "DefinedCards", "ChooseEach", "ControlledByPlayer"} {
		if _, ok := ps.m[k]; ok {
			return refuse("optional ChooseCard from the library with %s$", k)
		}
	}
	n := len(lc.ab.Prog)
	if n == 0 {
		return refuse("optional ChooseCard from the library without a preceding Peek")
	}
	pk := lc.ab.Prog[n-1]
	if pk.Op != "Peek" || pk.CS != "Remember" || pk.Obj.Kind != rYou || !pk.A.isConst() {
		return refuse("optional ChooseCard from the library not straight after a Remember-only Peek of your library")
	}
	g := &lFilter{Players: f.Players, Clauses: append([]lClause(nil), f.Clauses...)}
	for i := range g.Clauses {
		c := &g.Clauses[i]
		if c.Flags&fRemembered == 0 || c.Ctrl != 0 || c.Owner != 0 {
			return refuse("optional ChooseCard from the library without an IsRemembered filter")
		}
		c.Flags &^= fRemembered
	}
	ps.consume("ChoiceTitle")
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: g, C: zLibrary, A: pk.A, B: cst(max), CS: "LibraryUpTo", Note: "up to B of the Peek's top A cards"})
	return nil
}

// playSAExpr translates ValidSA$ to a filter expression, ANDed with the
// population filter by the caller. The accepted spellings:
// Spell, Spell.cmcLE<n>, Spell.cmcLE<letter> (the bound is the ability's SVar
// of that letter, lowered by lc.filter like X), and Instant[.cmc...],Sorcery[.cmc...].
// A plain Spell is the land skip OpCastFree already applies, so it adds
// nothing; a Spell cmc bound adds just the bound (lands are still skipped).
// Gorge tests the printed face; kgen refuses a multi-face card before here.
func (lc *lowerCtx) playSAExpr(v string) (string, error) {
	if v == "" {
		return "", refuse("Play without ValidSA")
	}
	if v == "Spell" {
		return "", nil
	}
	var alts []string
	for _, alt := range strings.Split(v, ",") {
		parts := strings.Split(strings.TrimSpace(alt), ".")
		var clause string
		spellHead := false
		switch parts[0] {
		case "Spell":
			spellHead = true
			clause = "Card.nonLand"
		case "Instant", "Sorcery":
			clause = parts[0]
		default:
			return "", refuse("ValidSA %q", v)
		}
		for _, p := range parts[1:] {
			switch {
			case isCmcLEVar(p):
				clause = andClause(clause, p)
			case spellHead && lc.bluffPlay && playNonCreatureYouOwn(p):
				// Spell.nonCreature+YouOwn (Brazen Bluff's free cast from hand)
				clause = andClause(clause, p)
			case strings.HasPrefix(p, "cmcLE"), strings.HasPrefix(p, "cmcLT"), strings.HasPrefix(p, "cmcEQ"):
				if _, err := strconv.Atoi(p[5:]); err != nil {
					return "", refuse("ValidSA %q", v)
				}
				if spellHead && !strings.HasPrefix(p, "cmcLE") {
					return "", refuse("ValidSA %q", v)
				}
				clause = andClause(clause, p)
			default:
				return "", refuse("ValidSA %q", v)
			}
		}
		if clause != "" {
			alts = append(alts, clause)
		}
	}
	return strings.Join(alts, ","), nil
}

// isCmcLEVar reports a mana-value bound named by a one-letter SVar: cmcLEX,
// cmcLEY (Captain Benjamin Sisko's Officer count). gorge reads any one
// capital letter from the resolving ability's SVar table
// (effects/play.go validSAOK).
func isCmcLEVar(p string) bool {
	return len(p) == 6 && strings.HasPrefix(p, "cmcLE") && p[5] >= 'A' && p[5] <= 'Z'
}

// playNonCreatureYouOwn is the ValidSA$ property pair nonCreature+YouOwn.
func playNonCreatureYouOwn(p string) bool { return p == "nonCreature+YouOwn" }

// playOwnHand is a Play whose candidates are the resolving controller's hand:
// Valid$ Card, ValidZone$ Hand, no target, no Defined$, no Controller$ other
// than You (gorge effPlay's hand walk reads only that seat's slice).
func playOwnHand(ps *params) bool {
	_, vt := ps.m["ValidTgts"]
	c, hasC := ps.m["Controller"]
	return ps.m["Valid"] == "Card" && ps.m["ValidZone"] == "Hand" && !vt && ps.m["Defined"] == "" && (!hasC || c == "You")
}

// andClause joins two filter clauses with + (AND within one alternative).
func andClause(a, b string) string {
	if a == "" || a == "Card" {
		return b // a bare Card head adds nothing to the other side
	}
	if b == "" {
		return a
	}
	// nonLand is the spell base; the left side always carries the type
	// head, so drop a duplicate base from the right.
	if strings.HasPrefix(b, "Card.nonLand+") {
		b = b[len("Card.nonLand+"):]
	}
	return a + "+" + b
}

// andFilterExpr ANDs two simple filter expressions (comma alternatives,
// plus-joined props) by cross-product. Anything with a grouping or negation
// is outside the shapes this reader models and refuses.
func andFilterExpr(a, b string) (string, error) {
	if a == "" {
		return b, nil
	}
	if b == "" {
		return a, nil
	}
	if strings.ContainsAny(a, "()!|") || strings.ContainsAny(b, "()!|") {
		return "", refuse("filter %q AND %q", a, b)
	}
	var out []string
	for _, x := range strings.Split(a, ",") {
		for _, y := range strings.Split(b, ",") {
			out = append(out, andClause(strings.TrimSpace(x), strings.TrimSpace(y)))
		}
	}
	return strings.Join(out, ","), nil
}

// playRememberedClause reads the P1 Valid$ shapes:
// Card.IsRemembered[+nonLand][+YouOwn][+cmcLE<letter>]. IsRemembered is the population and
// nonLand is the kernel's land skip, so only the cmc bound reaches the filter.
func (lc *lowerCtx) playRememberedClause(valid string) (string, error) {
	if strings.Contains(valid, ",") {
		return "", refuse("Play Valid$ %q", valid)
	}
	var cmc string
	for _, part := range strings.Split(valid, "+") {
		switch {
		case part == "Card.IsRemembered", part == "IsRemembered", part == "nonLand", part == "Card.nonLand":
		case part == "YouOwn":
			// owned by the controller: kept as a filter clause (a library can
			// hold a card another player owns), not assumed from the Dig
			cmc = andClause(cmc, part)
		case isCmcLEVar(part):
			cmc = andClause(cmc, part)
		case strings.HasPrefix(part, "cmcLE"), strings.HasPrefix(part, "cmcLT"), strings.HasPrefix(part, "cmcEQ"):
			if _, err := strconv.Atoi(part[5:]); err != nil {
				return "", refuse("Play Valid$ %q", valid)
			}
			cmc = andClause(cmc, part)
		default:
			return "", refuse("Play Valid$ %q", valid)
		}
	}
	if cmc == "" {
		return "", nil
	}
	return andClause("Card.nonLand", cmc), nil
}

// play lowers api:Play's four resolution-time free-cast spellings.
func (lc *lowerCtx) play(ps *params) error {
	// Named parameters this reader does not model: refuse rather than drop.
	for _, k := range []string{"PlayCost", "WithTotalCMC", "RememberPlayed", "ManaConversion",
		"SorcerySpeed", "ShowCardToActivator", "ImprintPlayed", "ReplaceGraveyardValid", "TargetMin", "TargetMax"} {
		if _, ok := ps.get(k); ok {
			return refuse("Play %s", k)
		}
	}
	// CopyCard$ True (CR 707.12): the cast is of a copy of the card.
	// Read exactly for the one shape the kernel proves in lockstep: the
	// remembered cards in exile (Valid$ Card.IsRemembered, ValidZone$ Exile),
	// ValidSA$ Spell, free and optional. Every other population (Defined$,
	// targets, hand/graveyard, ExiledWith) is refused: a Defined$ Remembered
	// card may be in any zone, which the kernel's exile population does not
	// see.
	copyCard := false
	if v, ok := ps.get("CopyCard"); ok {
		if v != "True" {
			return refuse("Play CopyCard %q", v)
		}
		if ps.m["ValidSA"] != "Spell" || ps.m["ValidZone"] != "Exile" || !(strings.Contains(ps.m["Valid"], "IsRemembered") || lc.grantExile && ps.m["Valid"] == "Card.Self") ||
			ps.m["Defined"] != "" || ps.m["ValidTgts"] != "" || ps.m["ForgetPlayed"] != "" || ps.m["ReplaceGraveyard"] != "" {
			return refuse("Play CopyCard outside Valid$ IsRemembered, ValidZone$ Exile, ValidSA$ Spell")
		}
		copyCard = true
	}
	hideDef := ps.m["Defined"] == "ExiledWith" // the Hideaway Play keeps its ConditionCheckSVar$ gate (nodeCondition)
	for _, k := range sortedParamKeys(ps.m) {
		// a lone ConditionPresent$ is the node's generic SkipIf guard
		// (nodeCondition; Malcolm's "if there are four or more chorus
		// counters on NICKNAME")
		if hideDef && (k == "ConditionCheckSVar" || k == "ConditionSVarCompare") {
			continue
		}
		if playRememberedEmptyKey(ps, k) {
			continue // nodeCondition's SkipIf guard (Brazen Bluff's nothing-was-exiled gate)
		}
		if strings.HasPrefix(k, "Condition") && !(k == "ConditionPresent" && playLoneCondition(ps)) {
			ps.consume(k)
			return refuse("Play %s", k)
		}
	}
	ps.consume("DefinedDesc", "IfDesc")
	// the exile-granted ability's copy is cast paying its mana cost (gorge
	// beginPlay: the printed cost, mana window and all; lower_exilegrant.go)
	paid := lc.grantExile && copyCard && ps.m["Valid"] == "Card.Self" && ps.m["WithoutManaCost"] == ""
	if !paid && !ps.is("WithoutManaCost", "True") {
		return refuse("Play without WithoutManaCost")
	}
	if !ps.is("Optional", "True") {
		return refuse("Play without Optional")
	}
	if c, ok := ps.get("Controller"); ok && c != "You" {
		return refuse("Play Controller %q", c)
	}
	amount := ps.str("Amount")
	// Amount$ X with AllowRepeats$ True (CR 707.12: "copy that card X times, you
	// may cast the copies", Infinite Diversity): the one remembered card is
	// offered X times, each pick one copy cast. Read only for the copy-cast
	// shape the kernel proves in lockstep (CopyCard$ above).
	repeat, upTo := false, false
	if v, ok := ps.get("AllowRepeats"); ok {
		if v != "True" || !copyCard || amount == "" || amount == "1" || amount == "All" {
			return refuse("Play AllowRepeats %q", v)
		}
		repeat = true
	} else if amount != "" && amount != "1" && amount != "All" {
		// a literal Amount$ N (gorge effPlay: one ask, Max N, the picks
		// begun in answer order) on P1's remembered-exile population only
		// (Doom Reigns Supreme's "up to two spells")
		if n, err := strconv.Atoi(amount); err != nil || n < 2 || ps.m["ValidZone"] != "Exile" || !strings.Contains(ps.m["Valid"], "IsRemembered") || ps.m["Defined"] != "" || copyCard {
			return refuse("Play Amount %q", amount)
		}
		upTo = true
	}
	forget := false
	if v, ok := ps.get("ForgetPlayed"); ok {
		if v != "True" {
			return refuse("Play ForgetPlayed %q", v)
		}
		forget = true
	}
	replace := false
	if v, ok := ps.get("ReplaceGraveyard"); ok {
		if v != "Exile" {
			return refuse("Play ReplaceGraveyard %q", v)
		}
		replace = true
	}
	if hideDef {
		// the Hideaway card's Play (lower_hideaway.go)
		for _, k := range []string{"Valid", "ValidZone", "ValidTgts", "ValidSA"} {
			if _, ok := ps.m[k]; ok {
				return refuse("Play Defined$ ExiledWith with %s$", k)
			}
		}
		if forget || replace {
			return refuse("Play Defined$ ExiledWith with ForgetPlayed$/ReplaceGraveyard$")
		}
		ps.consume("Defined")
		return lc.hidePlay(ps, amount)
	}
	// nonCreature+YouOwn lowers behind Brazen Bluff's nothing-remembered
	// gate and over an untargeted, undefined hand population (Yue, the Moon
	// Spirit): gorge effPlay walks only the resolving controller's hand, whose
	// cards that player owns, so YouOwn filters nothing there
	lc.bluffPlay = playRememberedEmptyKey(ps, "ConditionDefined") || playOwnHand(ps)
	var saExpr string
	var err error
	if !playCommandZone(ps) {
		// (the command-zone Play reads its absent ValidSA$: lower_playcommand.go)
		saExpr, err = lc.playSAExpr(ps.str("ValidSA"))
	}
	lc.bluffPlay = false
	if err != nil {
		return err
	}
	// gorge tests a ValidSA$ filter against the printed face, so a filtered
	// Play stays refused on a multi-face host; a plain ValidSA$ Spell tests
	// nothing about the host (Etali, Primal Conqueror's transform back)
	if saExpr != "" && faceLayout(lc.card, lc.out.Token) != kop.FaceNone {
		return refuse("Play on a multi-face card")
	}
	_, hasVT := ps.m["ValidTgts"]
	cs := "Optional"
	if amount == "All" {
		cs += "|AnyNumber"
	}
	// An absent Amount$ is Max 1 (gorge effects/play.go: absent or 1 gives
	// Max 1), so only a targeted population (always one card) skips the bit.
	if amount == "1" || amount == "" && !hasVT {
		cs += "|One"
	}
	if copyCard {
		cs += "|Copy"
	}
	if paid {
		cs += "|Paid"
	}
	var repeatN lValue
	if repeat {
		if repeatN, err = lc.value(amount); err != nil {
			return err
		}
		cs += "|Repeat"
	}
	if upTo {
		if repeatN, err = lc.value(amount); err != nil {
			return err
		}
		cs += "|UpTo"
	}
	if forget {
		cs += "|Forget"
	}
	if replace {
		cs += "|ReplaceGY"
	}
	def := ps.str("Defined")
	valid := ps.str("Valid")
	_, hasValid := ps.m["Valid"]
	vzone := ps.str("ValidZone")
	tgtZone := ps.str("TgtZone")
	emit := func(obj lRef, zone int, expr string) error {
		var f *lFilter
		if expr != "" {
			if f, err = lc.filter(expr); err != nil {
				return err
			}
		}
		lc.emit(lInstr{Op: "CastFree", Obj: obj, A: repeatN, F: f, C: zone, CS: cs})
		return nil
	}

	// P2: targeted card in a graveyard.
	if hasVT {
		if tgtZone != "Graveyard" && vzone != "Graveyard" {
			return refuse("Play target zone %q", tgtZone)
		}
		if def != "" && def != "Targeted" {
			return refuse("Play Defined$ %q with ValidTgts$", def)
		}
		if forget {
			return refuse("Play ForgetPlayed$ on a targeted Play")
		}
		obj, err := lc.objOrTarget(ps, "", zGraveyard)
		if err != nil {
			return err
		}
		return emit(obj, zGraveyard, saExpr)
	}
	if def == "Remembered" {
		if vzone == "Graveyard" && valid == "Card.IsRemembered" && !forget && !replace {
			// P1g: Defined$ Remembered in a graveyard (Malcolm's discarded
			// card, RememberDiscarded$ in the same chain)
			return emit(lRef{Kind: rRemembered}, zGraveyard, saExpr)
		}
		// P1: Defined$ Remembered.
		if hasValid || vzone != "" {
			return refuse("Play Defined$ Remembered with Valid$/ValidZone$")
		}
		return emit(lRef{Kind: rRemembered}, zExile, saExpr)
	}
	if def == "Imprinted" && lc.digImprinted && !hasValid && vzone == "" && !forget && !replace {
		// the DigUntil's imprinted found card, the resolution's remembered
		// set in the kernel (lower_digimprint.go)
		return emit(lRef{Kind: rRemembered}, zExile, saExpr)
	}
	if def != "" {
		return refuse("Play Defined$ %q", def)
	}
	if !hasValid {
		return refuse("Play shape")
	}
	if paid {
		// the source card itself, in exile (lower_exilegrant.go): Valid$
		// Card.Self + ValidZone$ Exile, Amount$ All, ValidSA$ Spell
		if saExpr != "" || amount != "All" || vzone != "Exile" || valid != "Card.Self" || forget || replace || repeat {
			return refuse("Play Card.Self outside Amount$ All, ValidZone$ Exile, ValidSA$ Spell")
		}
		return emit(lRef{Kind: rSelf}, zExile, "")
	}
	switch vzone {
	case "Exile":
		// P1: Valid$ Card.IsRemembered... + ValidZone$ Exile.
		if !strings.Contains(valid, "IsRemembered") {
			return refuse("Play ValidZone$ Exile without IsRemembered")
		}
		clause, err := lc.playRememberedClause(valid)
		if err != nil {
			return err
		}
		expr, err := andFilterExpr(clause, saExpr)
		if err != nil {
			return err
		}
		return emit(lRef{Kind: rRemembered}, zExile, expr)
	case "Library":
		// P1l: the remembered cards still in the controller's library (Cosmic
		// Cube's peeked six), gorge's Valid$ + ValidZone$ walk
		if valid != "Card.IsRemembered" || replace {
			return refuse("Play ValidZone$ Library with Valid$ %q", valid)
		}
		lc.playLibrary = true
		return emit(lRef{Kind: rRemembered}, zLibrary, saExpr)
	case "Command":
		// P5: the resolving controller's own commanders in the command zone
		// (Geode Golem; lower_playcommand.go)
		if !playCommandZone(ps) || forget || replace {
			return refuse("Play ValidZone$ Command with Valid$ %q", valid)
		}
		return emit(lRef{Kind: rNone}, zCommand, "")
	case "Hand", "Graveyard":
		// P3/P4: untargeted filter population over the controller's hand or
		// the graveyards, in seat order.
		if !strings.Contains(valid, "YouOwn") && !strings.Contains(valid, "YouCtrl") && !(valid == "Card" && (playRememberedEmptyKey(ps, "ConditionDefined") || playOwnHand(ps)) && ps.str("ValidSA") == "Spell.nonCreature+YouOwn") {
			return refuse("Play Valid$ %q without YouOwn/YouCtrl", valid)
		}
		if forget {
			return refuse("Play ForgetPlayed$ outside P1")
		}
		if vzone == "Graveyard" && amount != "1" {
			return refuse("Play from Graveyard without Amount$ 1")
		}
		expr, err := andFilterExpr(valid, saExpr)
		if err != nil {
			return err
		}
		zone := zHand
		if vzone == "Graveyard" {
			zone = zGraveyard
		}
		return emit(lRef{Kind: rNone}, zone, expr)
	default:
		return refuse("Play ValidZone$ %q", vzone)
	}
}

// effectStaticBodies resolves StaticAbilities$ to its parsed static lines.
// The value is a whitespace/comma-separated list of SVar names (Kardur's
// "AttackEach,AttackOther"); gorge's effEffect splits it the same way and
// parses each body through parseStaticLine.
func (lc *lowerCtx) effectStaticBodies(list string) ([]cards.Static, error) {
	var out []cards.Static
	for _, name := range strings.FieldsFunc(list, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		body, ok := lc.face.SVars[name]
		if !ok {
			return nil, refuse("Effect static %q", name)
		}
		ss, ok := cards.ParseStaticLines(body)
		if !ok || len(ss) == 0 {
			return nil, refuse("Effect static %q", name)
		}
		out = append(out, ss...)
	}
	if len(out) == 0 {
		return nil, refuse("Effect static %q", list)
	}
	return out, nil
}

func (lc *lowerCtx) effectAPI(ps *params) error {
	if tr, ok := ps.get("Triggers"); ok {
		if _, st := ps.m["StaticAbilities"]; st {
			if done, err := lc.effectUntapDenial(ps, tr); done || err != nil { // lower_untapdenial.go
				return err
			}
		}
		if _, st := ps.m["StaticAbilities"]; st && ps.m["RememberObjects"] != "" {
			return lc.effectStaticTriggerPair(ps, tr) // lower_effectcaptured.go
		}
		return lc.emblem(ps, tr)
	}
	// EffectOwner$ (CR 611.2): effEffect resolves the created effect's owner
	// lazily with EffectOwnerPlayers, whose empty value and You both name the
	// resolving controller (engine/effects/effect.go:197-199,
	// engine/effects/context.go:1804-1805) -- exactly the owner the kernel
	// already gives an Effect. Only You is the default; any other spelling
	// (Opponent, TriggeredTarget, ...) stays unread, so the node check
	// refuses it by name.
	if lc.blind != nil {
		if err := lc.blindEffectOwner(ps); err != nil {
			return err
		}
	}
	if v, ok := ps.m["EffectOwner"]; ok && strings.TrimSpace(v) == "You" {
		ps.consume("EffectOwner")
	}
	st, ok := ps.get("StaticAbilities")
	if !ok {
		if _, ok := ps.get("ReplacementEffects"); ok {
			return lc.effectPrevent(ps)
		}
		return refuse("Effect without statics")
	}
	ss, serr := lc.effectStaticBodies(st)
	if serr != nil {
		return serr
	}
	if ok, err := lc.noRegenRemembered(ps, ss); ok || err != nil { // lower_noregen.go
		return err
	}
	if ps.is("Duration", "Permanent") {
		if ok, err := lc.partitionMayPlay(ps, ss); ok || err != nil {
			return err
		}
		if len(ss) != 1 {
			return refuse("Effect static %q", st)
		}
		return lc.emblemStatic(ps, ss[0])
	}
	if ok, err := lc.unspentRedEOT(ps, ss); ok || err != nil {
		return err
	}
	if obj, ok := lc.chosenMayPlayNextTurn(ps, ss); ok { // lower_effectdurations.go
		return lc.effectTargetStatic(ps, ss[0], obj)
	}
	_, hasTgt := ps.m["ValidTgts"]
	if _, hasRO := ps.get("RememberObjects"); !hasTgt && !hasRO {
		return lc.effectStaticEOT(ps, ss)
	}
	if ok, err := lc.cantAttackRemembered(ps, ss); ok || err != nil { // lower_effectcantattackplayer.go
		return err
	}
	if len(ss) != 1 && !attackBlockPair(ss) { // lower_effectattacksifable.go
		return refuse("Effect static %q", st)
	}
	s := ss[0]
	var obj lRef
	var err error
	if vt, ok := ps.m["ValidTgts"]; ok {
		_ = vt
		zone := zBattlefield
		if z, ok := ps.m["TgtZone"]; ok {
			zone = zoneNames[z]
		}
		if obj, err = lc.objOrTarget(ps, "", zone); err != nil {
			return err
		}
		// On a link with its own target, Targeted and ThisTargetedCard name
		// the same object: gorge effectRememberedCodes maps both to the
		// link's own answer (staticline.go), as Forge's getSATargetingCard
		// stops at the link itself (lower_damageprevent.go reads both too).
		if ro := ps.str("RememberObjects"); ro != "Targeted" && ro != "ThisTargetedCard" {
			return refuse("Effect RememberObjects %q", ro)
		}
	} else {
		ro := ps.str("RememberObjects")
		if ro == "RememberedCard" {
			// gorge effectRememberedCodes: RememberedCard is Remembered
			ro = "Remembered"
		}
		if obj, err = lc.defined(ro); err != nil {
			return err
		}
	}
	if len(ss) == 2 {
		return lc.attacksOrBlocksIfAble(ps, ss, obj) // lower_effectattacksifable.go
	}
	return lc.effectTargetStatic(ps, s, obj)
}

// effectTargetStatic lowers the one static s of a targeted (or remembered)
// Effect over its resolved object obj.
func (lc *lowerCtx) effectTargetStatic(ps *params, s cards.Static, obj lRef) error {
	sp := newParams(s.Params)
	sp.consume("Mode", "Description")
	switch s.Mode {
	case "CantBlockBy":
		// Creature.IsRemembered names the same attackers as Card.IsRemembered
		// (an attacker is a creature); Speed, Young Avenger spells it so.
		if va := sp.str("ValidAttacker"); va != "Card.IsRemembered" && va != "Creature.IsRemembered" {
			return refuse("Effect CantBlockBy shape")
		}
		exile, hasExile := ps.get("ExileOnMoved")
		exHaste := false
		exC := 0
		if vb, ok := sp.get("ValidBlocker"); ok {
			// Agility Bobblehead: "can't be blocked this turn except by
			// creatures with haste" (kernel ExtExHaste, the blocker's CURRENT
			// keywords like gorge's withoutHaste); Cavern Stomper / Cheeky
			// House-Mouse: Creature.power<op><N> (lower_cantblockpower.go, the
			// blocker's CURRENT power). The only ValidBlocker$ the kernel
			// holds besides the chosen-colour form (colorGrantEffect); the
			// ExileOnMoved$ Battlefield shape, or (Speed, Young Avenger) a
			// targeted creature with no ExileOnMoved$ and no Duration$: the
			// grant matches the remembered object id, which a zone change
			// replaces, so it lapses on departure either way, and it ends at
			// the turn's end as the exile form does.
			if !hasExile && !lc.cantBlockTargetedUntilEOT(ps, obj) {
				return refuse("Effect CantBlockBy ValidBlocker %q", vb)
			}
			c, err := lc.cantBlockValidBlocker(vb)
			if err != nil {
				return err
			}
			exHaste, exC = true, c
		}
		switch {
		case hasExile && strings.TrimSpace(exile) == "Battlefield":
			// ends for each permanent as it leaves (CR 611.2c), as a record does
		case !hasExile && exHaste:
			// the targeted haste or power-compare form checked above
		case !hasExile && lc.cantBlockAttackingTarget(ps, obj):
			// (Gossip's Talent) ExileOnMoved$ is
			// unread here because gorge's CantBlockBy matches the remembered
			// object id, and a zone change mints a new id -- the grant lapses
			// on departure with or without the exile sweep. The target is
			// attacking and the kernel has no extra combat, so "this turn"
			// reaches only this combat's declare-blockers step.
		default:
			return refuse("Effect CantBlockBy shape")
		}
		if ro := ps.str("RememberObjects"); ro != "" {
			switch ro {
			case "TriggeredAttacker":
				// gorge's effectRememberedCodes has no "TriggeredAttacker"
				// entry, so the nested Effect resolves with an empty
				// Remembered set and its CantBlockBy grants nothing (kc10b
				// Gravelgill Scoundrel / Tidal Terror); granting to the
				// trigger source would diverge.
				return refuse("Effect CantBlockBy RememberObjects %q", ro)
			case "TriggeredAttackerLKICopy":
				// gorge DOES remember this one: effectRememberedCodes maps
				// TriggeredAttackerLKICopy to RememberedLKI
				// (engine/effects/staticline.go:911 at the pin), so the
				// restriction grants unblockable. The kernel has no
				// remembered-LKI grant; do not lower it as a live object.
				return refuse("Effect CantBlockBy RememberObjects LKICopy")
			}
		}
		if exHaste {
			lc.emit(lInstr{Op: "ExHaste", Obj: obj, C: exC, Note: "until end of turn: blockable only by creatures with haste, or not by creatures whose power compares (C)"})
			break
		}
		lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: "Unblockable"})
	case "MustAttack":
		if err := lc.attacksIfAble(ps, sp, obj); err != nil { // lower_effectattacksifable.go
			return err
		}
	case "MustBlock":
		if err := lc.blocksIfAble(ps, sp, obj); err != nil { // lower_effectattacksifable.go
			return err
		}
	case "CantBeActivated":
		// (Braided Net) "its activated abilities can't be activated
		// for as long as it remains tapped" on the tapped target. Only the
		// exact shape gorge's effEffect registers with a UntilTargetedUntaps
		// lifetime (kernel ExtLockActs): the single remembered target, every
		// activated ability (mana abilities too), gone at its first untap or
		// departure.
		if obj.Kind != rTarget || ps.str("RememberObjects") != "Targeted" || ps.str("Duration") != "UntilTargetedUntaps" ||
			ps.str("ForgetOnMoved") != "Battlefield" || sp.str("ValidCard") != "Card.IsRemembered" || sp.str("ValidSA") != "Activated" {
			return refuse("Effect CantBeActivated shape")
		}
		ps.consume("Duration", "ForgetOnMoved")
		sp.consume("ValidCard", "ValidSA")
		lc.emit(lInstr{Op: "LockActs", Obj: obj, Note: "its activated abilities can't be activated while it stays tapped"})
	case "Panharmonicon":
		// (Lwaxana Troi, Charming Guest) "for as long as this remains tapped,
		// if a triggered ability of that creature triggers, that ability
		// triggers an additional time". Only the exact shape gorge's effEffect
		// registers (kernel ExtGrantPanharm): the single remembered target,
		// no scoping term beyond ValidCard$ Card.IsRemembered, a host-tap
		// lifetime and the target leaving the battlefield ending it.
		if obj.Kind != rTarget || ps.str("RememberObjects") != "Targeted" || ps.str("Duration") != "UntilUntaps" ||
			ps.str("ExileOnMoved") != "Battlefield" || sp.str("ValidCard") != "Card.IsRemembered" {
			return refuse("Effect Panharmonicon shape")
		}
		ps.consume("Duration", "ExileOnMoved")
		sp.consume("ValidCard")
		lc.emit(lInstr{Op: "GrantPanharm", Obj: obj, Note: "the target's triggers trigger an additional time while the source stays tapped"})
	case "Continuous":
		if sp.is("RemoveAllAbilities", "True") { // lower_effectdurations.go
			return lc.loseAbilitiesWhileHost(ps, sp, obj)
		}
		if !sp.is("MayPlay", "True") {
			return refuse("Effect continuous shape")
		}
		// EffectZone$ restricts a static to its SOURCE's zone
		// (engine/rules/statics_cast.go:833-841: affectedZoneOK(az, src.Zone)).
		// An Effect is registered as a command-zone object, so on an Effect's
		// static EffectZone$ Command never denies. Any other zone stays
		// unread, so the static check refuses it by name.
		if v, ok := sp.m["EffectZone"]; ok && strings.TrimSpace(v) == "Command" {
			sp.consume("EffectZone")
		}
		aff := sp.str("Affected")
		nonLand := false
		if (obj.Kind == rRemembered || obj.Kind == rTarget || lc.digImprintChosen && obj.Kind == rChosen) && (aff == "Card.IsRemembered+nonLand" || aff == "Card.nonLand+IsRemembered") {
			// "you may cast" the remembered nonland cards, or (Silas Renn) the
			// target RememberObjects$ Targeted remembers: the grant skips a
			// land (its type cannot change off the battlefield)
			aff, nonLand = "Card.IsRemembered", true
		}
		instSorc := false
		azs := sp.str("AffectedZone")
		if obj.Kind == rRemembered && (aff == "Instant.IsRemembered,Sorcery.IsRemembered" || aff == "Sorcery.IsRemembered,Instant.IsRemembered") && azs == "Exile,Stack" {
			// Chandra, Hope's Beacon: the grant covers the remembered instants
			// and sorceries; the Stack half of the zone list adds no cast
			// permission (a spell already on the stack is not cast again)
			aff, azs, instSorc = "Card.IsRemembered", "Exile", true
		}
		zone := zoneNames[azs]
		zones := []int{zone}
		if azs == "Graveyard,Exile" {
			// (Ark of Hunger) gorge's grant is live over the zone list and
			// ends when the remembered card leaves either; the kernel's
			// per-card record is version-tied, so each zone's instruction
			// grants the same set a list-valued grant would.
			if obj.Kind != rRemembered {
				return refuse("Effect MayPlay AffectedZone %q over %s", azs, obj)
			}
			ex, ok := ps.get("ExileOnMoved")
			if !ok || strings.TrimSpace(ex) != azs {
				return refuse("Effect MayPlay AffectedZone %q with ExileOnMoved %q", azs, ex)
			}
			zone = zGraveyard
			zones = []int{zGraveyard, zExile}
		} else if strings.Contains(azs, ",") {
			return refuse("Effect MayPlay AffectedZone %q", azs)
		}
		// Mosswood Dreadknight's "cast it from your graveyard as an
		// Adventure": the grant permits only the Adventure face's cast
		// (gorge MayPlayAdventureSpec, cr-mayplay-adventure-from-graveyard)
		advOnly := false
		if aff == "Card.IsRemembered+Adventure" && zone == zGraveyard && len(zones) == 1 && obj.Kind == rSelf {
			aff, advOnly = "Card.IsRemembered", true
		}
		if (aff != "Card.IsRemembered" && aff != "Card.ChosenCard") || (zone != zExile && zone != zGraveyard) {
			return refuse("Effect MayPlay shape")
		}
		dur := "EOT"
		if d, ok := ps.get("Duration"); ok {
			switch d {
			case "UntilTheEndOfYourNextTurn":
				dur = "EndOfYourNextTurn"
			case "EndOfTurn", "UntilEndOfTurn", "UntilYourNextEndStep":
				// gorge effectUntilEOTSet: the continuous side folds these
				// into this turn (UntilEOT, dropped at cleanup)
			case "AsLongAsControl":
				// gorge continuousLive: live while the Effect's source is on the
				// battlefield under the effect's controller, and gone for good
				// when it changes controller
				// (). An
				// instant or sorcery source is never there, so refuse it.
				if lc.face.IsInstant() || lc.face.IsSorcery() {
					return refuse("Effect Duration %q from a spell", d)
				}
				dur = "WhileSrc"
			default:
				return refuse("Effect Duration %q", d)
			}
		}
		ps.consume("ForgetOnMoved")
		in := lInstr{Op: "MayPlay", Obj: obj, CS: dur}
		fl := 0
		// EffectOwner$ RememberedController (CR 611.2): gorge's effect
		// belongs to the remembered card's controller, and its may-play
		// grant is that player's. A remembered card is in exile here, where
		// gorge's controller read is the card's owner (a move resets
		// Controller to Owner; a RememberLKI$ capture is the only other
		// source and this shape has none), so MayPlayPlayer$ CardOwner --
		// the kernel's MayPlayOwner record -- names the same seat.
		// Expedited Inheritance's Dig from the damaged creature's
		// controller's library.
		if owner, ok := ps.m["EffectOwner"]; ok && strings.TrimSpace(owner) == "RememberedController" &&
			obj.Kind == rRemembered && (zone == zExile || len(zones) == 2) {
			fl |= kop.MayPlayOwner
			ps.consume("EffectOwner")
		}
		if _, ok := ps.get("ExileOnMoved"); ok && obj.Kind == rRemembered {
			// gorge effectMoveSweep: a remembered card leaving the zone ENDS
			// the whole effect; the kernel's per-card permissions match it
			// only for one remembered card (B&1: OpMayPlay stalls on more)
			fl |= kop.MayPlayExileOnMoved
		}
		if nonLand {
			fl |= kop.MayPlayNonLand
		}
		if advOnly {
			fl |= kop.MayPlayAdventure
		}
		if instSorc {
			fl |= kop.MayPlayInstSorc
		}
		// gorge mayPlayParamsScan: the Effect-delivered grant's riders ride
		// the permission record (MayPlayFree, MayPlayIgnoreType)
		for _, r := range [...]struct {
			key string
			bit int
		}{{"MayPlayWithoutManaCost", kop.MayPlayFree}, {"MayPlayIgnoreType", kop.MayPlayAnyType},
			{"MayPlayIgnoreColor", kop.MayPlayIgnoreColor}} {
			if v, ok := sp.get(r.key); ok {
				if strings.TrimSpace(v) != "True" {
					return refuse("Effect %s %q", r.key, v)
				}
				fl |= r.bit
			}
		}
		if v, ok := sp.get("MayPlayLimit"); ok {
			// gorge MayPlayStaticParams: a non-negative integer, 0 unlimited
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n < 0 || n > 7 {
				return refuse("Effect MayPlayLimit %q", v)
			}
			fl |= n << kop.MayPlayLimitShift
		}
		if v, ok := sp.get("MayPlayAltManaCost"); ok {
			// Inside Information: the exiled cards are cast by paying life
			// equal to their mana value instead of their mana cost
			// (CR 118.9). The one alternative the kernel's remembered
			// permission record carries; it rides alone, over the
			// end-of-turn grant of the remembered set.
			if strings.TrimSpace(v) != "PayLife<ConvertedManaCost>" || obj.Kind != rRemembered || dur != "EOT" ||
				fl&(kop.MayPlayFree|kop.MayPlayAnyType|kop.MayPlayIgnoreColor|kop.MayPlayLimitMask|kop.MayPlayOwner|kop.MayPlayAdventure|kop.MayPlayOpponent) != 0 {
				return refuse("Effect MayPlayAltManaCost %q", v)
			}
			fl |= kop.MayPlayPayLife
		}
		if fl != 0 {
			in.B = cst(fl)
		}
		for _, z := range zones {
			in.C = z
			lc.emit(in)
		}
	default:
		return refuse("Effect static mode %s", s.Mode)
	}
	if left := sp.unread(); len(left) > 0 {
		return refuse("Effect static unread %v", left)
	}
	return nil
}

// emblemStatic lowers a planeswalker emblem with one static (Vivien Reid).
func (lc *lowerCtx) emblemStatic(ps *params, s cards.Static) error {
	ps.consume("Name", "Image", "Stackable")
	sts, err := lc.staticBody(s)
	if err != nil {
		return err
	}
	for _, st := range sts {
		if err := emblemFiltersRefuse("Emblem static", st.F, st.F2); err != nil {
			return err
		}
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Statics: sts})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return nil
}

// emblem lowers a planeswalker emblem with one trigger (Kaito).
func (lc *lowerCtx) emblem(ps *params, tr string) error {
	lc.granted++
	defer func() { lc.granted-- }()
	_, hasDur := ps.get("Duration")
	eot := !hasDur
	next := hasDur && ps.is("Duration", "UntilYourNextTurn")
	if hasDur && !next && !ps.is("Duration", "Permanent") {
		return refuse("Effect triggers without a permanent duration")
	}
	ps.consume("Name", "Image", "Stackable")
	if sa, ok := ps.get("StaticAbilities"); ok && lc.pairCap == nil {
		// Osteomancer Adept: gorge's effEffect Notes its may-play grant as
		// unimplemented and offers NOTHING through it (the RaiseCost$
		// Forage rider; mayplay_effect_riders_test.go
		// TestEffectGrantUnreadRiderStillRefused), while the Triggers$
		// half registers as normal. Only that exact rider shape is
		// consumed here, granting nothing; anything else refuses by name
		if err := lc.emblemUnimplementedMayPlay(sa); err != nil {
			return err
		}
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[tr])
	if !ok {
		return refuse("emblem trigger %q", tr)
	}
	if t.Mode == "BecomeMonarch" {
		return monarchEffectTrigger() // lower_monarch.go
	}
	if eot && t.Mode == "TapsForMana" {
		return effectTapExtraRefuse() // lower_tapextra.go
	}
	if (eot || next) && t.Mode != "SpellCast" && t.Mode != "ChangesZone" && t.Mode != "Sacrificed" && t.Mode != "DamageDone" && !(t.Mode == "DamageDealtOnce" && lc.pairCap != nil) &&
		t.Mode != "AttackersDeclared" && t.Mode != "AttackersDeclaredOneTarget" && t.Mode != "Attacks" {
		// an until-end-of-turn Effect's trigger (gorge: the effect leaves
		// the command zone at cleanup) is the kernel's EOT emblem, and an
		// until-your-next-turn one its next-turn emblem; both fire only for
		// the events emblems listen to
		return refuse("Effect triggers without a permanent duration")
	}
	if t.Effect == nil {
		t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
	}
	// RememberObjects$ Targeted beside ValidTgts$ (The Inner Light: "choose
	// target creature. Whenever it deals combat damage to a player this
	// turn"): the chain's target slot is the emblem's capture
	mem, err := lc.effectRemember(ps, &t, eot, next) // lower_effectremember.go
	if err != nil {
		return err
	}
	var capture lRef
	var hasCap, player bool
	if mem.kind == memNone {
		if capture, hasCap, player, t, err = lc.emblemCapture(ps, t, eot); err != nil {
			return err
		}
	}
	if !hasCap && eot && mem.kind == memNone {
		var src bool
		if t, src, err = effectSourceTrigger(ps, t); err != nil {
			return err
		}
		if src {
			capture, hasCap = lRef{Kind: rSelf}, true // the emblem remembers the Effect's source object
		}
	}
	saved := len(lc.out.Abilities)
	lc.capRem = hasCap && t.Mode == "DamageDealtOnce"
	lc.effCount = mem.kind == memCount
	err = lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "Command") })
	lc.capRem, lc.effCount = false, false
	if err != nil {
		return err
	}
	if len(lc.out.Abilities) != saved+1 {
		return refuse("emblem trigger lowering")
	}
	ab := lc.out.Abilities[saved]
	if mem.kind != memNone {
		if err := effectRememberBody(ab, mem); err != nil { // lower_effectremember.go
			return err
		}
	}
	if hasCap && !player {
		if err := emblemCaptureFilter(ab); err != nil {
			return err
		}
	}
	if eot || next {
		if err := emblemFiltersRefuse("Effect trigger", ab.Trigger.F, ab.Trigger.F2); err != nil {
			return err
		}
		if err := eotCombatEmblemRefuse(ab); err != nil {
			return err
		}
	}
	ab.Latent = true
	ab.Name = "EmblemTrigger"
	em := lEmblem{Trigger: saved, UntilEOT: eot, UntilNextTurn: next, Capture: capture, CaptureSet: hasCap, CaptureIsPlayer: player}
	ce := lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}}
	mem.apply(&em, &ce) // lower_effectremember.go
	lc.out.Emblems = append(lc.out.Emblems, em)
	ce.C = len(lc.out.Emblems) - 1
	lc.emit(ce)
	return nil
}

// emblemUnimplementedMayPlay consumes an Effect's StaticAbilities$ list
// beside its Triggers$ (Osteomancer Adept). gorge's
// effEffect Notes a Continuous may-play grant it does not implement as
// "continuous effect Continuous unimplemented" and offers nothing through
// it, so the kernel grants nothing too — but only for that exact spelling:
// the one static body is Mode$ Continuous | MayPlay$ True | AffectedZone$
// Graveyard | RaiseCost$ Forage. Any other shape would grant in gorge and
// refuses here by name rather than be dropped.
func (lc *lowerCtx) emblemUnimplementedMayPlay(list string) error {
	ss, err := lc.effectStaticBodies(list)
	if err != nil {
		return err
	}
	if len(ss) != 1 || ss[0].Mode != "Continuous" {
		return refuse("Effect StaticAbilities$ %q beside Triggers$", list)
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode", "Description")
	if sp.str("Affected") != "Creature.nonLand+YouOwn" || !sp.is("MayPlay", "True") ||
		sp.str("AffectedZone") != "Graveyard" || sp.str("RaiseCost") != "Forage" {
		return refuse("Effect StaticAbilities$ %q beside Triggers$ (gorge implements nothing here)", list)
	}
	sp.consume("Affected", "MayPlay", "AffectedZone", "RaiseCost")
	if left := sp.unread(); len(left) > 0 {
		return refuse("Effect StaticAbilities$ %q beside Triggers$ unread %v", list, left)
	}
	return nil
}

func (lc *lowerCtx) animate(ps *params) error {
	if _, ok := ps.get("Abilities"); ok {
		return lc.animateGrantExile(ps) // lower_exilegrant.go
	}
	if d, ok := ps.get("Defined"); ok && strings.HasPrefix(d, "ValidGraveyard ") {
		return lc.animateGraveMayPlay(ps, strings.TrimPrefix(d, "ValidGraveyard "))
	}
	if d, ok := ps.m["Defined"]; ok && strings.HasPrefix(strings.TrimSpace(d), "Valid ") {
		_, err := lc.definedValid(&cards.SA{API: "Animate"}, ps)
		return err
	}
	var guard lCond
	selfDefined := false
	if d, ok := ps.get("Defined"); ok && strings.HasPrefix(d, "Targeted.") {
		// Defined$ Targeted.<spec>: the chain's target if it matches spec
		// at resolution (gorge definedSpec with a property filter; Azure
		// Beastbinder's "if it's a creature")
		lc.unionProbe("animate:" + d)
		t := lc.lastTarget()
		if t < 0 {
			return refuse("Defined$ %q with no target slot", d)
		}
		q := strings.TrimPrefix(d, "Targeted.")
		if strings.HasPrefix(q, "!") {
			// gorge definedCardQualifierMatches reads the qualifier as
			// "Card." + q, and "!X" negates X (Tezzeret, Cruel Captain's
			// emblem: "if it's not a creature")
			q = "Card." + q
		}
		f, err := lc.filter(q)
		if err != nil {
			return err
		}
		ps.consume("Defined")
		ref := lRef{Kind: rTarget, N: t}
		guard = lCond{Kind: cRefMatches, Ref: ref, F: f, Op: opGE, A: cst(1)}
		o, dur, chosen, err := lc.animateOverride(ps, lRef{})
		if err != nil {
			return err
		}
		if chosen {
			// five records, one op; the guard
			// bounds them by the target matching the qualifier
			ts := &lc.ab.Targets[t]
			if ts.Zone != zBattlefield || !ts.Max.isConst() || ts.Max.N != 1 {
				return refuse("chosen-colour Animate target slot: not one battlefield permanent")
			}
			return lc.guardBlock(guard, func() error {
				lc.emitChosenColorAnimate(ref, o, dur)
				return nil
			})
		}
		return lc.guardBlock(guard, func() error {
			lc.emitOverride(ref, o, dur)
			return nil
		})
	}
	if d, ok := ps.get("Defined"); ok && strings.HasPrefix(d, "Self.") {
		// Defined$ Self.<qualifier>: the source if it matches the qualifier
		// at resolution (gorge triggeredCardQualified's definedSpecSelf base;
		// Iron Man Armor's "if this Equipment isn't a creature"). The whole
		// animation, its optional election and its end-step rider sit behind
		// the guard, as gorge's empty set reaches none of them.
		q := strings.TrimSpace(strings.TrimPrefix(d, "Self."))
		if q == "" || strings.Contains(q, " & ") {
			return refuse("Defined$ %q", d)
		}
		f, err := lc.filter("Card." + q)
		if err != nil {
			return err
		}
		ps.consume("Defined")
		selfDefined = true
		guard = lCond{Kind: cRefMatches, Ref: lRef{Kind: rSelf}, F: f, Op: opGE, A: cst(1)}
	}
	obj := lRef{Kind: rSelf}
	if !selfDefined {
		var err error
		obj, err = lc.objOrTarget(ps, "Self", zBattlefield)
		if err != nil {
			return err
		}
	}
	_, optional := ps.m["Optional"]
	ps.consume("OptionQuestion") // prompt text only (gorge combatfx.go); the yes/no is the same
	if optional && obj.Kind != rSelf {
		// gorge asks only while some Defined object is still there
		// (animateWouldChange): MayGuardMove tests that for the one source
		// ref the next op names
		return refuse("Animate Optional$ on a non-Self object")
	}
	o, dur, chosen, err := lc.animateOverride(ps, obj)
	if err != nil {
		return err
	}
	if chosen && obj.Kind == rTarget {
		// the chosen-colour op applies to ONE battlefield object (the
		// kernel's OpOverride skips a ref off the battlefield, and one op
		// reaches one ref): gorge's Animate reaches every target of an
		// "any number" slot (Sway of Illusion) and reads a spell on the
		// stack (Vodalian Mystic) -- refuse rather than silently apply
		// only the first / none
		t := &lc.ab.Targets[obj.N]
		if t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N != 1 {
			return refuse("chosen-colour Animate target slot: not one battlefield permanent")
		}
	}
	atEOT, err := lc.animateAtEOT(ps, obj)
	if err != nil {
		return err
	}
	return lc.guardBlock(guard, func() error {
		if optional {
			if chosen {
				return refuse("Animate Optional$ with a chosen colour")
			}
			if err := lc.mayGate(ps, kop.MayGuardMove, lRef{Kind: rYou}); err != nil {
				return err
			}
		}
		if chosen {
			if o.MustBeBlocked {
				return refuse("Animate HiddenKeywords$ must be blocked with a chosen colour")
			}
			lc.emitChosenColorAnimate(obj, o, dur)
		} else {
			lc.emitOverride(obj, o, dur)
		}
		if atEOT != nil {
			atEOT()
		}
		return nil
	})
}

// animateOverride reads an Animate / AnimateAll grant as an override and
// its duration (EOT or Permanent). chosen: the grant's Colors$ is the
// ChosenColor token (a mid-resolution chosen-colour animation: the caller
// emits five records differing only in SetColors).
// obj is the one object the Animate reaches when the caller knows it is the
// source (rSelf); a self-bound granted static reads it.
func (lc *lowerCtx) animateOverride(ps *params, obj lRef) (*lOverride, string, bool, error) {
	o := &lOverride{}
	dur := "EOT"
	if d, ok := ps.get("Duration"); ok {
		switch d {
		case "Permanent", "UntilYourNextTurn":
		case "UntilHostLeavesPlay":
			o.WhileHost, d = true, "WhileHost" // lower_tokenattacking.go
		case "AsLongAsInPlay":
			// "for as long as CARDNAME remains on the battlefield" (Awakener
			// Druid, Skilled Animator): the ANIMATING source is the host, the
			// same lifetime UntilHostLeavesPlay names (gorge animateHostScoped).
			o.WhileHost, d = true, "WhileHost"
		case "UntilUntaps":
			// "for as long as CARDNAME remains tapped" (Hedge Whisperer):
			// host-bound, and also ended by the host's untap (gorge
			// registerAnimateEffects' UntilUntapOf; static_animatewhiletapped.go)
			o.WhileHost, o.WhileTapped, d = true, true, "WhileHost"
		default:
			return nil, "", false, refuse("Animate Duration %q", d)
		}
		dur = d
	}
	if err := lc.animateHiddenKeywords(ps, o, obj, dur); err != nil {
		return nil, "", false, err
	}
	if err := lc.animateRemoveKeywords(ps, o, obj); err != nil {
		return nil, "", false, err
	}
	if t, ok := ps.get("Types"); ok {
		for _, w := range strings.Split(t, ",") {
			w = strings.TrimSpace(w)
			if b, ok := typeAddBit(w); ok {
				o.AddTypes |= b
				continue
			}
			if pseudoSubtype(w) {
				// a dynamic chosen-type override is not a literal subtype
				return nil, "", false, refuse("Animate dynamic type %q", w)
			}
			if !lc.known(w) {
				return nil, "", false, refuse("Animate type %q", w)
			}
			o.SetSubtypes = append(o.SetSubtypes, w)
		}
		rmCreat := ps.is("RemoveCreatureTypes", "True")
		if ps.is("RemoveCardTypes", "True") {
			// gorge animateGrant removeCardTypes: the card types go before
			// Types$ apply (Lizard, Connors's Curse)
			o.HasSetTypes = true
			o.SetTypes, o.AddTypes = o.AddTypes&^tSupertype, o.AddTypes&tSupertype
			o.HasSetSubtypes = true
			if err := o.keepSubtypes(strings.Split(t, ",")); err != nil {
				return nil, "", false, err
			}
		} else {
			// gorge applyTypeEffect strips only the creature subtypes, an
			// Equipment or land type staying (CR 205.1a), then adds Types$
			o.RemoveCreatureSub = rmCreat
			o.AddSubtypes, o.SetSubtypes = o.SetSubtypes, nil
		}
	}
	if ps.is("AddAllCreatureTypes", "True") {
		o.AllCreatureTypes = true
	}
	if c, ok := ps.get("Colors"); ok {
		// gorge animateGrant: Colors$ with OverwriteColors$ True REPLACES
		// the object's colours (layer 5, CR 613.1e; the Restless lands'
		// "becomes a red and white Ox"); without it the colours are added,
		// which the override record's AddColors expresses
		overwrite := ps.is("OverwriteColors", "True")
		if strings.EqualFold(strings.TrimSpace(c), "ChosenColor") {
			if !overwrite {
				return nil, "", false, refuse("Animate Colors$ without OverwriteColors$")
			}
			// gorge combatfx animateGrant resolves
			// the token alone to the source's ChosenColor AT RESOLUTION
			// (failing closed to no grant when unset); a mixed list keeps
			// refusing here. The preceding ChooseColorRes is what sets
			// res.chosen for the caller's five records.
			if !lc.colGrant {
				return nil, "", false, refuse("Animate Colors$ %q outside a chosen-colour grant", c)
			}
			return o, dur, true, nil
		}
		cols, ok := colorWordSet(c)
		if !ok {
			return nil, "", false, refuse("Animate Colors$ %q", c)
		}
		if overwrite {
			o.HasSetColors, o.SetColors = true, cols
		} else {
			o.AddColors = cols // ORed in at layer 5
		}
	}
	if sv, ok := ps.get("sVars"); ok {
		// the SVars the object gains: AI hints no rules reader consults
		if !lc.aiHintSVars(sv) {
			return nil, "", false, refuse("Animate sVars$ %q", sv)
		}
	}
	if ps.is("RemoveAllAbilities", "True") {
		// layer 6 for the effect's duration (gorge Animate
		// RemoveAllAbilities$: the record's LoseAbilities, CR 613.1f)
		o.LoseAbilities = true
	}
	pS, okP := ps.get("Power")
	tS, okT := ps.get("Toughness")
	if okP || okT {
		// each is set only when present (gorge effAnimate's
		// SetPowerPresent / SetToughnessPresent: Symmetry Sage's base
		// power 2 keeps the toughness)
		pv, tv := cst(0), cst(0)
		var err error
		if okP {
			if pv, err = lc.value(pS); err != nil {
				return nil, "", false, refuse("Animate Power$ %q: %v", pS, err)
			}
		}
		if okT {
			if tv, err = lc.value(tS); err != nil {
				return nil, "", false, refuse("Animate Toughness$ %q: %v", tS, err)
			}
		}
		if o.WhileHost && !(pv.isConst() && tv.isConst()) {
			// the applied record keeps the host's object and version in its P/T
			// fields, so a host-bound P/T must come from the table (SetP/SetT);
			// a value read at resolution has no room (gorge registers the P/T
			// half with the host as DurationSource)
			return nil, "", false, refuse("Animate host-bound duration with a non-constant P/T")
		}
		if o.LivePT {
			return nil, "", false, refuse("Animate Power$/Toughness$ beside a granted static's P/T")
		}
		o.HasSetPT, o.KeepP, o.KeepT = true, !okP, !okT
		if pv.isConst() && tv.isConst() {
			o.SetP, o.SetT = pv.N, tv.N
		} else {
			o.PVal, o.TVal = pv, tv
		}
	}
	if n, ok := ps.get("Name"); ok {
		// a literal layer-3 rename (gorge combatfx.go: a chooser token is
		// not a rename there; refuse it here rather than mint a name card)
		n = strings.TrimSpace(n)
		if strings.EqualFold(n, "ChosenName") || strings.EqualFold(n, "Chosen") || n == "" {
			return nil, "", false, refuse("Animate Name$ %q", n)
		}
		o.Name = n
		lc.out.Tokens = append(lc.out.Tokens, nameCardKey+n)
	}
	if k, ok := ps.get("Keywords"); ok {
		k, err := lc.animateGrantedKeywordAbilities(k, o) // lower_animatekwability.go
		if err != nil {
			return nil, "", false, err
		}
		if k != "" {
			ks, err := pumpKeywords(k)
			if err != nil {
				return nil, "", false, err
			}
			o.AddKw = ks
		}
	}
	if tr, ok := ps.get("Triggers"); ok {
		idx, err := lc.latentTrigger(tr)
		if err != nil {
			return nil, "", false, err
		}
		o.GrantAbility = append(o.GrantAbility, idx)
	}
	return o, dur, false, nil
}

// emitChosenColorAnimate emits the five chosen-colour override records
// (SetColors W, U, B, R, G at consecutive out.overrides indexes) and ONE
// Override op whose C is the first: at resolution the kernel applies
// C + (res.chosen - 1), the record whose colour the controller just chose
// (ExtChooseColorRes; gorge effChooseColor + combatfx animateGrant read the
// source's ChosenColor AT RESOLUTION). The order IS the contract: kgen
// asserts it in build_ab (chosenColorOverridesOK).
func (lc *lowerCtx) emitChosenColorAnimate(obj lRef, o *lOverride, dur string) {
	start := len(lc.out.overrides)
	for _, col := range "WUBRG" {
		r := *o
		r.HasSetColors, r.SetColors = true, string(col)
		lc.out.overrides = append(lc.out.overrides, &r)
	}
	in := lInstr{Op: "Override", Obj: obj, CS: dur + "|chosenColor"}
	first := *lc.out.overrides[start]
	in.Note = overrideString(&first)
	in.C = start
	lc.emit(in)
}

// latentTrigger lowers a granted trigger (an SVar trigger line) as a latent
// ability and returns its index.
func (lc *lowerCtx) latentTrigger(name string) (int, error) {
	t, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok {
		return 0, refuse("granted trigger %q", name)
	}
	if t.Effect == nil {
		t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) <= idx {
		return 0, refuse("granted trigger lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = "GrantedTrigger"
	return idx, nil
}

func (lc *lowerCtx) copyPermanent(ps *params) error {
	if d, ok := ps.m["Defined"]; ok && strings.HasPrefix(strings.TrimSpace(d), "Valid ") {
		if _, ok := ps.m["ValidTgts"]; ok {
			// a player target whose permanents the Valid set names
			// (Mirror Universe Intrusion's kicked "each creature target
			// opponent controls": Creature.TargetedPlayerCtrl)
			tp := map[string]string{"ValidTgts": ps.str("ValidTgts")}
			for _, k := range []string{"TargetMin", "TargetMax"} {
				if v, ok := ps.get(k); ok {
					tp[k] = v
				}
			}
			r, err := lc.objOrTarget(newParams(tp), "", zBattlefield)
			if err != nil {
				return err
			}
			if t := lc.ab.Targets[r.N].F; t == nil || t.Players == 0 || len(t.Clauses) != 0 || !strings.Contains(d, "TargetedPlayerCtrl") {
				return refuse("CopyPermanent Defined$ %q beside a target that is not the player it names", d)
			}
			ps.consume("ValidTgts", "TgtPrompt", "ValidTgtsDesc")
		}
		_, err := lc.definedValid(&cards.SA{API: "CopyPermanent"}, ps)
		return err
	}
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) == "ExiledWith" {
		// every card exiled with the source, copied (gorge's exiledWithSet;
		// The Cloning of Shredder's chapters II and III). A Choices$ pool is
		// left unread and refuses by name: gorge asks the controller to pick
		// one of the set, which this op does not pose.
		ps.consume("Defined")
		exc, hasExc, err := lc.copyExceptions(ps)
		if err != nil {
			return err
		}
		f, err := lc.exiledWithFilter() // lower_exw.go
		if err != nil {
			return err
		}
		in := lInstr{Op: "CopyExiledWith", F: f}
		if hasExc {
			lc.out.overrides = append(lc.out.overrides, exc)
			in.C = len(lc.out.overrides) // index+1
			in.Note = "except " + overrideString(exc)
		}
		lc.emit(in)
		return nil
	}
	var obj lRef
	var named *namedCopy
	var picked bool
	var err error
	if n, ok := ps.get("DefinedName"); ok {
		// the source is a card of the name universe, not an object (lower_copynamed.go)
		if named, err = lc.copyNamedCard(ps, n); err != nil {
			return err
		}
	} else if _, ok := ps.m["Choices"]; ok {
		// the controller's one pick, copied (lower_copypicked.go)
		picked = true
		obj, err = lc.copyChoicePick(ps)
		if err != nil {
			return err
		}
	} else if ps.is("Populate", "True") {
		// populate: the controller's creature token, copied (lower_populate.go)
		picked = true
		obj, err = lc.populatePick(ps)
		if err != nil {
			return err
		}
	} else if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
		return err
	}
	in := lInstr{Op: "CopyAsToken", Obj: obj, A: cst(1)}
	if named != nil {
		in.Op = "CopyNamed"
	}
	if picked {
		in.Op = "CopyPicked"
	}
	var flags []string
	if ps.str("Controller") == "NonRememberedController" {
		// each player other than the remembered permanent's controller
		// (its last-known controller once it left; gorge
		// copyPermanentController's NonRememberedController): two-player,
		// that controller's opponent (Fractured Identity)
		if obj.Kind != rRemembered {
			return refuse("CopyPermanent Controller$ NonRememberedController over %v", obj)
		}
		delete(ps.m, "Controller") // read here, not as a copy exception
		flags = append(flags, "NonRemCtrl")
		lc.twoSeat("NonRememberedController read as the one opponent")
	}
	if n, ok := ps.get("NumCopies"); ok {
		// gorge copyPermanentNumCopies: CalculateAmount (For the Common
		// Good's X, Offspring's Count$OffspringPaid)
		v, err := lc.value(n)
		if err != nil {
			return err
		}
		in.A = v
	}
	// the copy exceptions (CR 707.9b; lower_copyexceptions.go)
	exc, hasExc, err := lc.copyExceptions(ps)
	if err != nil {
		return err
	}
	if hasExc && picked {
		return refuse("CopyPermanent Choices$ beside copy exceptions")
	}
	if hasExc && (named == nil || named.found) {
		lc.out.overrides = append(lc.out.overrides, exc)
		in.C = len(lc.out.overrides) // index+1
		in.Note = "except " + overrideString(exc)
	}
	if a, ok := ps.get("AtEOTTrig"); ok {
		if a != "Sacrifice" {
			return refuse("AtEOTTrig %q", a)
		}
		flags = append(flags, "SacAtEndStep")
	}
	if a, ok := ps.get("AtEOT"); ok {
		// gorge scheduleAtEOT: a one-shot delayed registration per copy,
		// exiled at the beginning of the next end step (Stormsplitter), or
		// sacrificed (__kwEncoreSacrifice: Reflection of Kiki-Jiki), the
		// kernel's one-shot dly entry either way
		switch a {
		case "Exile":
			flags = append(flags, "ExileAtEndStep")
		case "Sacrifice":
			if _, ok := ps.m["AtEOTTrig"]; ok {
				return refuse("CopyPermanent AtEOT %q beside AtEOTTrig$", a)
			}
			flags = append(flags, "SacAtEndDly")
		case "SacrificeUpkeep":
			// gorge atEOTRiders: the same untracked sacrifice body at the next
			// upkeep of any player (kop.CpSacAtUpkeep; Firion)
			if _, ok := ps.m["AtEOTTrig"]; ok {
				return refuse("CopyPermanent AtEOT %q beside AtEOTTrig$", a)
			}
			flags = append(flags, "SacAtUpkeep")
		default:
			return refuse("CopyPermanent AtEOT %q", a)
		}
	}
	if k, ok := ps.get("PumpKeywords"); ok {
		// not copiable (gorge :287, :308-327): a keyword grant on each token
		// after it enters, so a copy of the copy lacks it; permanent, or
		// until end of turn under PumpDuration$ EOT
		pumpFlag := "Pump="
		if d, ok := ps.get("PumpDuration"); ok {
			if d != "EOT" {
				return refuse("CopyPermanent PumpDuration %q", d)
			}
			pumpFlag = "PumpEOT="
		}
		ks, err := pumpKeywords(k)
		if err != nil {
			return err
		}
		if len(ks) != 1 {
			return refuse("CopyPermanent PumpKeywords %q: one keyword only", k)
		}
		if _, ok := kwBits[ks[0]]; !ok {
			return refuse("CopyPermanent PumpKeywords %q", k)
		}
		flags = append(flags, pumpFlag+ks[0])
	}
	if ps.is("RememberTokens", "True") {
		flags = append(flags, "Remember")
	}
	if picked {
		atk, err := lc.copyChoiceAttacking(ps)
		if err != nil {
			return err
		}
		if atk {
			flags = append(flags, "Attacking")
		}
	}
	if v, ok := ps.get("TokenTapped"); ok {
		// the token enters tapped (gorge copypermanent.go's CopyTokenTapped)
		if v != "True" {
			return refuse("CopyPermanent TokenTapped %q", v)
		}
		flags = append(flags, "Tapped")
	}
	if ct, ok := ps.get("WithCountersType"); ok {
		// the token enters with the counters (gorge copypermanent.go folds
		// them into the copy's own entry MoveZone; CR 122.6, 614.1c). The
		// kernel op carries the source, the amount and the kind and nothing
		// else: no copy exceptions, flags or NumCopies$
		amt, err := lc.amount(ps, "WithCountersAmount", 1)
		if err != nil {
			return err
		}
		if _, err := counterKind(ct); err != nil {
			return refuse("CopyPermanent WithCountersType$ %q", ct)
		}
		if hasExc || len(flags) != 0 || !in.A.isConst() || in.A.N != 1 {
			return refuse("CopyPermanent WithCountersType$ beside copy exceptions, AtEOT$/PumpKeywords$/RememberTokens$ or NumCopies$")
		}
		lc.emit(lInstr{Op: "CopyAsTokenCtr", Obj: obj, A: cst(1), B: amt, CS: ct})
		return nil
	}
	if named != nil {
		if !named.found {
			return nil // gorge copies nothing for a name its universe lacks
		}
		flags = append(flags, "card="+strings.TrimPrefix(named.key, copyCardKey))
		if !slices.Contains(lc.out.Tokens, named.key) {
			lc.out.Tokens = append(lc.out.Tokens, named.key)
		}
	}
	in.CS = strings.Join(flags, "|")
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) gainControl(ps *params) error {
	// NewController$ first: ParentTarget names the earlier link's player
	// target, not this node's own (Donate)
	to := lRef{}
	if nc, ok := ps.get("NewController"); ok && nc != "You" {
		r, err := lc.defined(nc)
		if err != nil {
			return err
		}
		if r.Kind != rTarget && r.Kind != rEachOpp {
			return refuse("GainControl NewController %q", nc)
		}
		if r.Kind == rTarget {
			t := lc.ab.Targets[r.N]
			if t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
				return refuse("GainControl NewController %q: not a player target", nc)
			}
		}
		to = r
	}
	// No ValidTgts$ and no Defined$: gorge's DefinedRef falls back to the
	// source ("an ability declaring ValidTgts$ acts on its chosen targets,
	// any other on its source"; effects/context.go DefinedRef), which is the
	// shape Contested Game Ball's "gains control of CARDNAME" takes. Any
	// other subject parameter (AllValid$ and kin) still reaches the node's
	// unread-params refusal.
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	// LoseControl$ (gorge ParseControlDuration): the effect lasts until the
	// first listed end; the kernel's layer-2 records (CR 613.1b) model
	// these lists, anything else is refused (a silently permanent steal is
	// wrong for any other lifetime)
	lcv := ps.str("LoseControl")
	dur := ""
	switch lcv {
	case "EOT":
		dur = "EOT"
	case "":
		dur = "Permanent"
	case "LeavesPlay":
		dur = "WhileSource"
	case "LeavesPlay,LoseControl", "LoseControl,LeavesPlay":
		dur = "WhileControlSource"
	case "UntilTheEndOfYourNextTurn":
		dur = "YourNextTurn"
	default:
		return refuse("GainControl LoseControl %q", lcv)
	}
	flags := []string{}
	if ps.is("Untap", "True") {
		flags = append(flags, "Untap")
	}
	if k, ok := ps.get("AddKWs"); ok {
		ks, err := pumpKeywords(k)
		if err != nil {
			return err
		}
		if dur != "EOT" || len(ks) != 1 || ks[0] != "Haste" {
			// gorge grants AddKWs$ for as long as the control effect lasts;
			// the kernel has the until-end-of-turn haste only
			return refuse("GainControl AddKWs %q with LoseControl %q", k, lcv)
		}
		flags = append(flags, "kw="+strings.Join(ks, "&"))
	}
	if rc, ok := ps.get("RememberControlled"); ok {
		// gorge effGainControl: each permanent whose control was registered
		// joins the remembered set (kop.GcRemember)
		if rc != "True" {
			return refuse("RememberControlled$ %q", rc)
		}
		flags = append(flags, "Remember")
	}
	if dur == "EOT" && to.Kind == rNone && strings.Join(flags, "|") == "Untap|kw=Haste" {
		// the threaten shape keeps its op (existing tables)
		lc.emit(lInstr{Op: "GainControlEOT", Obj: obj, CS: strings.Join(flags, "|")})
		return nil
	}
	flags = append(flags, "dur="+dur)
	in := lInstr{Op: "GainControl", Obj: obj, CS: strings.Join(flags, "|")}
	if to.Kind != rNone {
		in.Note = "to=" + to.String()
		in.C = int(to.Kind)<<8 | to.N
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) delayedTrigger(ps *params) error {
	if ps.str("Mode") == "SpellCast" {
		return lc.delayedSpellCast(ps) // lower_delayspell.go
	}
	if ps.str("Mode") == "Phase" && ps.str("Phase") == "Upkeep" {
		return lc.delayedUpkeep(ps)
	}
	if ps.str("Mode") == "Phase" && ps.str("Phase") == "EndCombat" {
		return lc.delayedEndCombat(ps)
	}
	if ps.str("Mode") == "Phase" && ps.str("Phase") == "BeginCombat" {
		return lc.delayedBeginCombat(ps) // lower_delayed.go
	}
	if ps.str("Mode") == "ChangesZone" {
		return lc.delayedLeaveReturn(ps) // lower_delayed.go
	}
	if ps.str("Mode") != "Phase" || ps.str("Phase") != "End of Turn" {
		return refuse("delayed trigger %s/%s", ps.str("Mode"), ps.str("Phase"))
	}
	when := "NextEndStep"
	switch vp := ps.str("ValidPlayer"); vp {
	case "", "Player":
	case "You":
		// the controller's next end step (gorge's ValidPlayer$ You: the
		// phase gate leaves the registration pending through another
		// player's end step; dirac/trig_delayedtiming.go dpExtYourEndStep)
		when = "YourNextEndStep"
	default:
		return refuse("delayed trigger ValidPlayer %q", vp)
	}
	ro := ps.str("RememberObjects")
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec == nil {
		return refuse("delayed trigger Execute %q", ps.str("Execute"))
	}
	var obj lRef
	group := false
	sacGroup := false
	lkiMany := false
	if ps.str("RememberChain") == "False" {
		if ro != "" {
			return refuse("RememberChain$ False delayed trigger shape")
		}
		if err := lc.groupDelayed(exec); err != nil {
			return err
		}
		group = true
	}
	if !group && ro == "Remembered" && delayedGroupSac(exec) {
		// a remembered set sacrificed as one group at the delayed step
		// (Will Riker, First in the Field: DigUntil RememberFound$ True, then
		// SacrificeAll Defined$ DelayTriggerRememberedLKI). DpGroup carries
		// the two objects the body sacrifices (lower_delayed.go).
		sacGroup = true
	}
	switch {
	case group:
		obj = lRef{Kind: rRemembered}
	case ro == "RememberedLKI":
		// the delayed record holds one object (kernel dprog/delayed): a
		// target slot taking several would return only the first
		if lc.delayedLKIMulti() {
			// a set of several objects: the kernel's DpMany program, for the
			// one shape it returns exactly (lower_delayedlkimany.go)
			if why := lc.delayedLKIManyShape(exec); why != "" {
				return refuse("delayed trigger RememberedLKI over a multi-object target slot: %s", why)
			}
			lkiMany = true
		}
		obj = lRef{Kind: rRemembered}
	case ro == "TriggeredNewCardLKICopy":
		obj = lRef{Kind: rTriggeredNew}
	default:
		var err error
		if obj, err = lc.delayedRemember(ro, exec); err != nil {
			return err
		}
	}
	lat := lc.newLatent("DelayedTrigger")
	lat.DelayedObjOnly = delayedObjOnly(ro)
	lat.DelayedSelf = delayedSelf(lc.ab.Trigger, ro)
	if group || sacGroup {
		lc.inGroup = true
	}
	err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) })
	lc.inGroup = false
	if err != nil {
		return err
	}
	in := lInstr{Op: "DelayedTrigger", Obj: obj, C: lat.Index, CS: when}
	if group || sacGroup {
		in.B = cst(1)
	}
	if ro == "Remembered" && lc.czForgetSet {
		// delayedRemember checked the body is one Bounce of the delayed set
		// (a many-object set: up to four objects, kernel DpMany)
		if len(lat.Prog) != 1 || lat.Prog[0].Op != "Bounce" || lat.Prog[0].Obj.Kind != rDelayedRemembered {
			return refuse("delayed many-object body lowered to %d ops, not one Bounce of the delayed set", len(lat.Prog))
		}
		lat.Prog[0].CS = "DelayedMany"
		in.B = cst(3)
	}
	if lkiMany {
		if len(lat.Prog) != 1 || lat.Prog[0].Op != "Return" || lat.Prog[0].Obj.Kind != rDelayedRemembered {
			return refuse("delayed trigger RememberedLKI over a multi-object target slot: the body lowered to %d ops, not one Return of the delayed set", len(lat.Prog))
		}
		if cs := lat.Prog[0].CS; cs == "" {
			lat.Prog[0].CS = "DelayedMany"
		} else {
			lat.Prog[0].CS = cs + "|DelayedMany"
		}
		in.B = cst(3)
	}
	lc.emit(in)
	return nil
}

// groupDelayed checks a RememberChain$ False delayed trigger is Mobilize's
// (CR 702.181a): the chain so far is exactly one remembering Token of the
// constant 1 or 2 and the body one Sacrifice of the delayed-remembered set.
func (lc *lowerCtx) groupDelayed(exec *cards.SA) error {
	pr := lc.ab.Prog
	if len(pr) != 1 || pr[0].Op != "CreateToken" || pr[0].C&4 == 0 || !pr[0].A.isConst() ||
		exec.API != "Sacrifice" || exec.Sub != nil || len(exec.Params) != 1 ||
		exec.Params["Defined"] != "DelayTriggerRememberedLKI" {
		return refuse("RememberChain$ False delayed trigger shape")
	}
	if n := pr[0].A.N; n != 1 && n != 2 {
		return refuse("Mobilize group of %d: a delayed program carries two objects", n)
	}
	return nil
}

// delayedGroupSac reports whether a remembered-set delayed trigger's body is
// one SacrificeAll of the delayed-remembered set (Will Riker, First in the
// Field: DigUntil RememberFound$ True, then SacrificeAll Defined$
// DelayTriggerRememberedLKI). The registration uses the DpGroup carrier and
// the body the same ExtSacDelayed op as Mobilize's group (lower_effect.go).
func delayedGroupSac(exec *cards.SA) bool {
	return exec != nil && exec.API == "SacrificeAll" && exec.Sub == nil &&
		len(exec.Params) == 1 && exec.Params["Defined"] == "DelayTriggerRememberedLKI"
}

func (lc *lowerCtx) immediateTrigger(ps *params) error {
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec == nil {
		return refuse("immediate trigger Execute %q", ps.str("Execute"))
	}
	n, err := lc.amount(ps, "TriggerAmount", 1)
	if err != nil {
		return err
	}
	if err := immediateIsPresent(ps); err != nil { // lower_remembered.go
		return err
	}
	lat := lc.newLatent("ReflexiveTrigger")
	lat.TrigRem = trigRemAllowed(ps) // lower_trigremembered.go
	savedRemX := lc.trigRemX
	lc.trigRemX = false
	if lc.trigXPaid && strings.TrimSpace(ps.m["RememberSVarAmount"]) == "X" {
		ps.consume("RememberSVarAmount") // CollectEvidence<X>: the announced X (Incinerator of the Guilty)
		lc.trigRemX = true
	}
	defer func() { lc.trigRemX = savedRemX }()
	capt, err := lc.reflexRememberTargeted(ps, lat, n) // lower_remembered.go
	if err != nil {
		return err
	}
	savedSpawn := lc.spawnDmg
	lc.spawnDmg = spawnDamagedOpp(lc.ab) // lower_filterwords.go
	savedAtk := lc.spawnAtk
	lc.spawnAtk = spawnAttacked(lc.ab) // lower_filterwords.go
	err = lc.withAbility(lat, func() error { return lc.chain(exec, nil) })
	lc.spawnAtk = savedAtk
	lc.spawnDmg = savedSpawn
	if err != nil {
		return err
	}
	b, err := lc.trigRememberedCapture(ps, lat)
	if err != nil {
		return err
	}
	if lat.ReflexObj && b.Kind != vNone {
		return refuse("ImmediateTrigger RememberObjects$ Targeted beside a TriggerRemembered$ capture")
	}
	lc.emit(lInstr{Op: "ReflexiveTrigger", Obj: capt, A: n, B: b, C: lat.Index})
	return nil
}

func (lc *lowerCtx) genericChoice(ps *params) error {
	// Perforating Artist: each opponent chooses one of the branches; the
	// fallback runs when no branch can be chosen.
	if d := ps.str("Defined"); (d == "You" || d == "") && ps.str("TempRemember") == "" {
		return lc.genericChoiceYou(ps)
	}
	who, err := lc.defined(ps.str("Defined"))
	if err != nil {
		return err
	}
	if ps.str("TempRemember") != "Chooser" {
		return refuse("GenericChoice without TempRemember Chooser")
	}
	choices := strings.Split(ps.str("Choices"), ",")
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatEach", Obj: who})
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rEach}, A: cst(len(choices))})
	branches := append(choices, ps.str("FallbackAbility"))
	for i, name := range branches {
		sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
		if sa == nil {
			return refuse("GenericChoice branch %q", name)
		}
		bat := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: i})
		if err := lc.chain(sa, nil); err != nil {
			return err
		}
		lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
	}
	// Remembered inside the branches is the chooser: the loop variable.
	for i := at; i < len(lc.ab.Prog); i++ {
		if lc.ab.Prog[i].Obj.Kind == rRemembered {
			lc.ab.Prog[i].Obj = lRef{Kind: rEach}
		}
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// lowerMana lowers an AB$ Mana.
func (lc *lowerCtx) lowerMana(sa *cards.SA, ps *params, ab *lAbility) error {
	m := &lMana{}
	split, err := lc.condSplitRoot(sa, ps, ab) // lower_condsplit.go: the sub folds into the root's lMana
	if err != nil {
		return err
	}
	m.Split = split
	prod := ps.str("Produced")
	if v := ps.peek("TriggersWhenSpent"); v != "" && prod == "R" {
		if err := lc.deferredMana(ab, ps, v); err != nil {
			return err
		}
		m.Deferred = true
	}
	switch {
	case len(prod) == 1 && strings.Contains("WUBRGC", prod):
		m.Colors = prod
	case prod == "ColorIdentity" || prod == "Combo ColorIdentity":
		// "any color in your commander's color identity" (CR 903.4). A plain
		// {T} source (Arcane Signet, Command Tower, Commander's Sphere, Hidden
		// Hideout) reads the controller's commanders' identity at each use
		// (kop.Ability.ManaIdentity; kernel mana_reflected.go identityColours):
		// one colour adds it unasked, none adds nothing (CR 903.4f, gorge
		// pay.CommanderIdentityColours). One with a rider (Path of Ancestry's
		// scry, Opal Palace's counters) or a cost beyond {T} stays a source of no
		// colour, which the kernel leaves out of payment: a TriggersWhenSpent$
		// rider waits on that mana being spent, so it never triggers
		if ps.peek("TriggersWhenSpent") == "" && ps.peek("AddsCounters") == "" && lc.granted == 0 && reflTapCost(&ab.Cost) {
			m.Special = "ReflectedIdentity"
			m.Colors, m.Choice, m.Amount = "WUBRG", true, cst(1)
			return lc.finishMana(sa, ps, ab, m)
		}
		ps.str("TriggersWhenSpent")
		m.Colors = ""
		m.Amount = cst(1)
		return lc.finishMana(sa, ps, ab, m)
	case strings.HasPrefix(prod, "Combo "):
		m.Choice = true
		m.Colors = strings.ReplaceAll(strings.TrimPrefix(prod, "Combo "), " ", "")
		if m.Colors == "Any" {
			m.Colors = "WUBRG"
		}
	case prod == "Any":
		m.Colors = "WUBRG"
	case prod == "Chosen":
		m.Colors = "Chosen"
	case strings.HasPrefix(prod, "Special EachColorAmong_Valid "):
		f, err := lc.filter(strings.TrimPrefix(prod, "Special EachColorAmong_Valid "))
		if err != nil {
			return err
		}
		m.Special = "EachColorAmong"
		m.SpecialF = f
		m.Colors = "WUBRG"
	default:
		// "R R": one colour named N times is N mana of it (gorge adds
		// each symbol); a mix of colours ("W U", "W W U U B B R R G G")
		// is simultaneous production, the same count of each listed
		// colour (lMana.Each; the kernel's shape guard is refuseMultiMana)
		syms := strings.Fields(prod)
		if len(syms) < 2 {
			return refuse("mana %q", prod)
		}
		var cnt [6]int
		for _, s := range syms {
			i := strings.Index("WUBRGC", s)
			if len(s) != 1 || i < 0 {
				return refuse("mana %q", prod)
			}
			cnt[i]++
		}
		if _, ok := ps.m["Amount"]; ok {
			return refuse("mana %q with Amount$", prod)
		}
		cols, n := "", 0
		for i, c := range cnt {
			if c == 0 {
				continue
			}
			if n != 0 && c != n {
				return refuse("mana %q: unequal counts of its colours", prod)
			}
			cols, n = cols+"WUBRGC"[i:i+1], c
		}
		m.Colors = cols
		m.Amount = cst(n)
		m.Each = len(cols) > 1
		return lc.finishMana(sa, ps, ab, m)
	}
	amt, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	m.Amount = amt
	return lc.finishMana(sa, ps, ab, m)
}

// staticRestrictWords maps a RestrictValid$ Static.<tail> SA-property token to
// the special actions it admits (gorge pay.SpecialHasProperty). The turn-up
// procedures are Forge's MorphUp/ManifestUp/CloakUp and the isTurnFaceUp
// union; Unlock is the Room door special action.
var staticRestrictWords = map[string]kop.SpecialMask{
	"MorphUp":      kop.SpmMorphUp,
	"ManifestUp":   kop.SpmManifestUp,
	"CloakUp":      kop.SpmCloakUp,
	"Unlock":       kop.SpmUnlock,
	"isTurnFaceUp": kop.SpmTurnFaceUp,
}

// staticRestrictUnsupported is every other SA-property token the corpus's
// Static.<tail> alternatives can name. A special action the kernel does not
// price for a restricted mana batch (foretell, plot) stays refused.
var staticRestrictUnsupported = map[string]bool{"Foretelling": true, "Plotting": true}

// staticRestrictClauses lowers one `Static.<tail>` RestrictValid$ alternative
// into card-filter clauses carrying its special-action mask. Tail tokens are
// '+' separated: the SA-property words set the mask, every other token is the
// paid object's card filter (Tin Street Gossip's `isTurnFaceUp+Creature`).
func (lc *lowerCtx) staticRestrictClauses(term string) ([]lClause, error) {
	tail := strings.TrimPrefix(term, "Static")
	tail = strings.TrimPrefix(tail, ".")
	if tail == "" {
		// bare Static: every special action (gorge's AbilityStatic root)
		return []lClause{{Special: kop.SpmTurnFaceUp | kop.SpmUnlock}}, nil
	}
	mask := kop.SpecialMask(0)
	var card []string
	for _, tok := range strings.Split(tail, "+") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if m, ok := staticRestrictWords[tok]; ok {
			mask |= m
			continue
		}
		if staticRestrictUnsupported[tok] {
			return nil, refuse("mana restriction %q: Static.%s special action", term, tok)
		}
		card = append(card, tok)
	}
	if mask == 0 {
		return nil, refuse("mana restriction %q", term)
	}
	if len(card) == 0 {
		return []lClause{{Special: mask}}, nil
	}
	f, err := lc.filter("Card." + strings.Join(card, "+"))
	if err != nil {
		return nil, err
	}
	if f.Players != 0 {
		return nil, refuse("mana restriction %q: player filter", term)
	}
	for i := range f.Clauses {
		f.Clauses[i].Special = mask
	}
	return f.Clauses, nil
}

// manaRestrict reads a mana ability's RestrictValid$ rv into m (Restrict for
// the Spell. and Static. alternatives, RestrictAb for the Activated ones,
// NonSpell for CantCastNonArtifactSpells); shared by finishMana and the
// resolving DB$ Mana (lower_dbmanarestr.go). A Static.<tail> alternative is a
// card-filter clause carrying the special actions it admits (kop.Filter.Special);
// the kernel's spell matcher skips those clauses and its special-action payment
// matches them instead.
func (lc *lowerCtx) manaRestrict(rv string, m *lMana) error {
	// the cast-provenance words (wasCastFromExile, !wasCastFromYourHand)
	// read the spell being paid for: allowed on this filter only, never
	// the Activated one (gorge restrictValidTermMatches)
	save := lc.spellProvOK
	lc.spellProvOK = true
	defer func() { lc.spellProvOK = save }()
	var spellClauses, staticClauses []lClause
	var act []string
	for _, alt := range strings.Split(rv, ",") {
		alt = strings.TrimSpace(alt)
		switch {
		case strings.HasPrefix(alt, "Spell."):
			spec := strings.TrimPrefix(alt, "Spell.")
			if spec == "isCastFaceDown" {
				// Forge's Spell.isCastFaceDown is an SA-property word, not a
				// card filter: gorge's RestrictValidMatches falls through to a
				// filter match that no card satisfies (fail-closed). Mirror
				// that exact reading so Tin Street Gossip's mana pays what
				// gorge's pays, not the printed oracle text.
				spellClauses = append(spellClauses, lClause{NotTypes: tAnyCard})
				continue
			}
			f, err := lc.filter("Card." + spec)
			if err != nil {
				return err
			}
			if f.Players != 0 {
				return refuse("mana restriction %q: player filter", rv)
			}
			spellClauses = append(spellClauses, f.Clauses...)
		case strings.HasPrefix(alt, "Activated."):
			act = append(act, "Card."+strings.TrimPrefix(alt, "Activated."))
		case alt == "Activated":
			// Gorge restrictValidTermMatches (pay/payment.go:606-607):
			// an undotted Activated is d.Class == PurposeActivated with
			// no card filter, every activated-ability payment from any
			// source in any zone. The any-card filter subsumes a
			// sibling Activated.<spec> term.
			act = append(act, "Card")
		case alt == "CantCastNonArtifactSpells":
			// Gorge restrictValidTermMatches: a spell whose face is an
			// artifact (Powerstone tokens).
			spellClauses = append(spellClauses, lClause{Types: tArtifact})
			m.NonSpell = true
		case alt == "CantCastSpellFromHand":
			// Gorge restrictValidTermMatches: every payment but a spell,
			// and a spell that was not cast from hand (the same pending-
			// guarded read as Spell.!wasCastFromYourHand).
			f, err := lc.filter("Card.!wasCastFromYourHand")
			if err != nil {
				return err
			}
			spellClauses = append(spellClauses, f.Clauses...)
			m.NonSpell = true
		case alt == "Static" || strings.HasPrefix(alt, "Static."):
			cl, err := lc.staticRestrictClauses(alt)
			if err != nil {
				return err
			}
			staticClauses = append(staticClauses, cl...)
		default:
			return refuse("mana restriction %q", rv)
		}
	}
	lc.spellProvOK = save
	m.Restrict = nil
	if len(spellClauses)+len(staticClauses) > 0 {
		m.Restrict = &lFilter{Clauses: append(spellClauses, staticClauses...)}
	}
	for _, c := range spellClauses {
		if c.CmpStat == cmpWasCastSpell && c.CmpN&15 == spellCast {
			// gorge's bare wasCast is no pending-guarded family:
			// its off-stack read can see an earlier cast
			return refuse("mana restriction %q: bare wasCast", rv)
		}
	}
	var err error
	if len(act) > 0 {
		if m.RestrictAb, err = lc.filter(strings.Join(act, ",")); err != nil {
			return err
		}
	}
	return nil
}

// finishMana reads a mana ability's spend restriction and chain.
func (lc *lowerCtx) finishMana(sa *cards.SA, ps *params, ab *lAbility, m *lMana) error {
	amt := m.Amount
	if rv, ok := ps.get("RestrictValid"); ok {
		if ab.Cost.PayLife > 0 {
			// gorge plans a restricted {T}, PayLife source as a
			// TierLastResort unit whose life is a consequence (Memory of
			// Toshiro); the kernel prices no restricted last-resort unit
			return refuse("mana restriction on a PayLife source: gorge plans it last-resort")
		}
		if !ab.Loyalty && !plainTapCost(ab.Cost) && !noCost(ab.Cost) {
			// the kernel's window prices a restricted mana ability only when
			// its cost is a plain {T}; gorge's cast-window probe also prices
			// one that sacrifices, pays mana or taps another permanent
			// (Jeweled Lotus' sacrifice), so the census would run short
			return refuse("mana restriction on a source whose cost goes beyond {T}")
		}
		if err := lc.manaRestrict(rv, m); err != nil {
			return err
		}
	}
	ab.Mana = m
	ps.consume("SubAbility")
	if stack := lc.stackManaAbility(sa, ab); ab.Loyalty || stack {
		// A loyalty ability is never a mana ability (CR 605.1a): Chandra's
		// +2 adds its mana on resolution. Nor is an AB$ Mana whose cost or
		// chain draws or mills (movesLibraryCard: Chromatic Sphere, the
		// eggs): gorge puts it on the stack and its resolution adds the mana
		// and runs the chain (lower_stackmana.go).
		if m.RestrictAb != nil || m.NonSpell {
			// the resolution's AddMana floats a spell restriction only
			// (State.rb): an activated-ability or non-spell one has no carrier
			return refuse("loyalty mana ability with an activated-ability or non-spell restriction")
		}
		ab.Kind = akActivated
		ab.Mana = nil
		if m.Each && !stack {
			// Lukka's +1 {R}{G}: the resolution's AddMana floats one colour
			// of a restriction (State.rb); several at once have no carrier
			return refuse("loyalty ability adding several colours at once")
		}
		if m.Each {
			// an unrestricted all-at-once production: one AddMana per colour
			if m.Restrict != nil || !amt.isConst() || amt.N < 1 {
				return refuse("stack mana ability adding several colours under a restriction")
			}
			for i := 0; i < len(m.Colors); i++ {
				lc.emit(lInstr{Op: "AddMana", Obj: lRef{Kind: rYou}, A: amt, CS: m.Colors[i : i+1]})
			}
		} else if m.Choice || len(m.Colors) > 1 {
			// a colour choice at resolution (gorge askManaChoice): one pick
			// per mana of a Combo, one for the whole amount of Any; the
			// kernel's AddMana takes the mask's last colour, no choice
			if m.Colors == "Chosen" || m.Special != "" || m.Colors == "" || strings.Trim(m.Colors, "WUBRG") != "" {
				return refuse("loyalty mana choice over %q", m.Colors)
			}
			if !amt.isConst() || amt.N < 1 || amt.N > 7 || len(m.Colors) < 2 || m.Restrict != nil || m.RestrictAb != nil {
				return refuse("loyalty mana choice amount or restriction")
			}
			mode := 0
			if !m.Choice {
				mode = 1
			}
			lc.emit(lInstr{Op: "ManaChoice", A: amt, C: mode, CS: m.Colors})
		} else {
			// F: a spell restriction (CR 106.6), floated as restricted
			// mana (State.rb) like gorge's ManaAdd restriction (Chandra, Chill
			// of Compliance's noncreature-only {U})
			lc.emit(lInstr{Op: "AddMana", Obj: lRef{Kind: rYou}, A: amt, CS: m.Colors, F: m.Restrict})
		}
	}
	if sa.Sub != nil && m.Split == nil { // (a Condition-split's sub is folded into m.Split)
		// Chandra's +2: mana, then the rest of the chain.
		if err := lc.chain(sa.Sub, nil); err != nil {
			return err
		}
	}
	if ab.Kind == akMana && len(ab.Prog) > 0 && !painRider(ab) && !ctrRiderMana(ab) { // (lower_manarider.go)
		// CR 605.3b: a mana ability's rider resolves with it at activation.
		// The bounded shapes the kernel runs are a draw, a self-sacrifice
		// once the source's last counter is gone, and each opponent gaining
		// life (riderMana); a conditional rider (Avid Reclaimer's Nissa
		// clause) is still refused rather than lowered without its rider.
		kind, ctr, n, ok := riderMana(ab)
		if !ok {
			return refuse("mana ability with a conditional or unbounded rider program")
		}
		m.Rider, m.RiderCtr, m.RiderN = kind, ctr, n
	}
	if err := lc.manaSpentRiders(ps, ab, m); err != nil {
		return err
	}
	if left := ps.unread(); len(left) > 0 {
		return refuse("unread params %v", left)
	}
	return nil
}

// rememberedFromOwnSelfExile reports whether this resolution already exiled
// the source itself with RememberChanged$ True, so a Defined$ Remembered read
// names that same card. The kernel's remembered set lives for one resolution;
// gorge's persists on the source, so the self-exile must be in this chain.
func (lc *lowerCtx) rememberedFromOwnSelfExile() bool {
	for i := len(lc.ab.Prog) - 1; i >= 0; i-- {
		in := &lc.ab.Prog[i]
		if in.Op == "ClearRemembered" {
			return false
		}
		if in.Op == "Exile" && in.Obj.Kind == rSelf && strings.Contains(in.CS, "Remember") {
			return true
		}
	}
	return false
}

// rememberLKIGuard checks a ChangeZone RememberLKI$ the kernel can capture:
// gorge captures each moved object's controller, the kernel only a
// targeted move's (one g.tli record). It reports whether a later node of
// the chain reads a controller (refused without a single target capture)
// and refuses an owner read (the kernel's off-battlefield match tests the
// controller and the owner against one player).
func (lc *lowerCtx) rememberLKIGuard(ps *params) (bool, error) {
	ctrl := lc.restReadsAny("Ctrl", "Controller")
	if lc.restReadsAny("Own") {
		return false, refuse("RememberLKI$ with an owner read")
	}
	if !ctrl {
		return false, nil
	}
	if _, tgt := ps.m["ValidTgts"]; !tgt || ps.m["ChangeType"] != "" {
		return true, refuse("RememberLKI$ controller read without a target capture")
	}
	if tm := strings.TrimSpace(ps.m["TargetMax"]); tm != "" && tm != "1" {
		return true, refuse("RememberLKI$ on a move of two or more targets whose chain reads a controller")
	}
	return true, nil
}

// restReadsAny reports whether a node after the one being lowered in its
// chain has a parameter value containing any of subs.
func (lc *lowerCtx) restReadsAny(subs ...string) bool {
	if lc.curNode == nil {
		return false
	}
	for n := lc.curNode.Sub; n != nil; n = n.Sub {
		for _, v := range n.Params {
			for _, s := range subs {
				if strings.Contains(v, s) {
					return true
				}
			}
		}
	}
	return false
}

// restReadsRemembered reports whether a node after the one being lowered in
// its chain names the remembered set (Defined$ Remembered and the like).
func (lc *lowerCtx) restReadsRemembered() bool {
	if lc.curNode == nil {
		return false
	}
	for n := lc.curNode.Sub; n != nil; n = n.Sub {
		for _, v := range n.Params {
			if strings.Contains(v, "Remembered") {
				return true
			}
		}
		if n.API == "DelayedTrigger" {
			// the delayed trigger names the set only in its Execute$ SVar
			// (Mobilize's RememberChain$ False group program)
			if sv := cards.ResolveSVar(lc.face.SVars, n.Params["Execute"]); sv != nil {
				for _, v := range sv.Params {
					if strings.Contains(v, "Remembered") {
						return true
					}
				}
			}
		}
	}
	return false
}

// playLoneCondition reports whether a Play node's only condition parameter
// is ConditionPresent$ (no ConditionDefined$ / ConditionCompare$ beside it).
func playRememberedEmptyKey(ps *params, k string) bool {
	switch k {
	case "ConditionDefined", "ConditionPresent", "ConditionCompare":
	default:
		return false
	}
	for _, other := range sortedParamKeys(ps.m) {
		if strings.HasPrefix(other, "Condition") && other != "ConditionDefined" && other != "ConditionPresent" && other != "ConditionCompare" {
			return false
		}
	}
	return ps.peek("ConditionDefined") == "Remembered" && ps.peek("ConditionPresent") == "Card" && ps.peek("ConditionCompare") == "EQ0"
}

func playLoneCondition(ps *params) bool {
	for k := range ps.m {
		if strings.HasPrefix(k, "Condition") && k != "ConditionPresent" {
			return false
		}
	}
	return true
}

// twoPilesSac is the battlefield TwoPiles shape whose chosen pile is
// sacrificed (Liliana of the Veil's -6; gorge effTwoPiles' battlefield
// arm, cr-twopiles-battlefield-validcards): the controller (Separator$
// You) splits the permanents matching ValidCards$ that the target player
// controls into two piles, that player (no Chooser$) picks one, and the
// ChosenPile$ body, SacrificeAll ValidCards$ Permanent.IsRemembered,
// sacrifices it. OpExt ExtTwoPilesSac.
func (lc *lowerCtx) twoPilesSac(ps *params) error {
	if _, ok := ps.get("Defined"); ok {
		return refuse("TwoPiles battlefield Defined$")
	}
	if _, ok := ps.get("ValidTgts"); !ok {
		return refuse("TwoPiles battlefield without a target player")
	}
	if ps.str("Separator") != "You" {
		return refuse("TwoPiles battlefield Separator$ %q", ps.str("Separator"))
	}
	for _, p := range []string{"Chooser", "UnchosenPile", "DefinedCards", "RememberChosen", "LeftRightPile", "FaceDown", "DefinedPiles"} {
		if _, ok := ps.get(p); ok {
			return refuse("TwoPiles battlefield %s$", p)
		}
	}
	vc := ps.str("ValidCards")
	switch vc {
	case "Permanent", "Creature", "Land", "Artifact", "Enchantment", "Planeswalker":
	default:
		return refuse("TwoPiles battlefield ValidCards$ %q", vc)
	}
	chosen := cards.ResolveSVar(lc.face.SVars, ps.str("ChosenPile"))
	if chosen == nil || chosen.API != "SacrificeAll" || chosen.Sub != nil || len(chosen.Params) != 1 ||
		chosen.Params["ValidCards"] != "Permanent.IsRemembered" {
		return refuse("TwoPiles battlefield pile action")
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	if who.Kind != rTarget {
		return refuse("TwoPiles battlefield pool %v", who)
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "TwoPilesSac", Obj: who, F: f, Note: "separate the target player's permanents into two piles; they sacrifice the pile of their choice"})
	return nil
}

// animateRemoveKeywords reads an Animate's RemoveKeywords$ (gorge
// animateGrant.removeKeywords: layer-6 keyword removal, CR 613.1f) and its
// staticAbilities$ grant, read by one of four routes: a comma list of
// permission/land-play statics, or a single Mode$ Continuous body over other
// permanents (an anthem, keyword grant or equipped pump), each lowered as an
// OvGate-gated static of the source (animateGrantStatics); a single
// Mode$ Continuous body that only sets the object's own P/T, the live P/T
// grant (animateLivePT); and the plain "can't be blocked" static
// (Mode$ CantBlockBy | ValidAttacker$ Creature.Self, Surge Engine), the
// kernel's KwUnblockable, where a host-bound duration refuses (the override's
// WhileHost sets no keyword record).
func (lc *lowerCtx) animateRemoveKeywords(ps *params, o *lOverride, obj lRef) error {
	if v, ok := ps.get("RemoveKeywords"); ok {
		// Only Surge Engine's shape (beside a CantBlockBy grant) has a
		// lockstep for single-target Animate; a bare RemoveKeywords$ stays
		// refused there. AnimateAll reads it bare.
		if _, has := ps.get("staticAbilities"); !has && !lc.animAll {
			return refuse("Animate RemoveKeywords$ without staticAbilities$")
		}
		ks, err := removeKeywords(v)
		if err != nil {
			return err
		}
		o.RemoveKw = append(o.RemoveKw, ks...)
	}
	v, ok := ps.get("staticAbilities")
	if !ok {
		return nil
	}
	name := strings.TrimSpace(v)
	if strings.Contains(name, ",") {
		return lc.animateGrantStatics(ps, name, o, false)
	}
	ss, ok := cards.ParseStaticLines(lc.face.SVars[name])
	if ok && len(ss) == 1 && ss[0].Mode == "Continuous" {
		if aff, has := ss[0].Params["Affected"]; has && strings.TrimSpace(aff) != "Card.Self" {
			// The static reaches other permanents (an anthem, keyword grant
			// or equipped pump): the animated object HAS it, so lower it as
			// an ordinary static of the source switched on by the override
			// record (animateGrantStatics). A self P/T setter stays the live
			// P/T grant.
			return lc.animateGrantStatics(ps, name, o, true)
		}
		return lc.animateLivePT(name, ss[0], o, obj)
	}
	if ok && len(ss) == 1 && ss[0].Mode == "CombatDamageToughness" {
		return lc.animateGrantedStatic(ps, name, ss[0], o, obj)
	}
	if !ok || len(ss) != 1 || ss[0].Mode != "CantBlockBy" {
		return refuse("Animate staticAbilities$ %q", name)
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode", "Description")
	if sp.str("ValidAttacker") != "Creature.Self" || sp.str("ValidBlocker") != "" {
		return refuse("Animate staticAbilities$ %q shape", name)
	}
	if u := sp.unread(); len(u) > 0 {
		return refuse("Animate staticAbilities$ %q unread params %v", name, u)
	}
	if o.WhileHost {
		return refuse("Animate staticAbilities$ with a host-bound duration")
	}
	o.AddKw = append(o.AddKw, "Unblockable")
	return nil
}

// animateGrantedStatic lowers an Animate staticAbilities$ grant whose one
// body is Mode$ CombatDamageToughness (Streetwise Negotiator's Backup rider:
// SVar:BackupAbility = DB$ Animate | staticAbilities$ CDT, over CDT =
// Mode$ CombatDamageToughness | ValidCard$ Card.Self; CR 702.165: the target
// gains the following ability until end of turn). The static lowers as the
// source's own assignment static (kop.StatCombatDmgToughness) with an OvGate
// bit, and the Animate record on the animated object carries the matching
// kop.Override.GrantStatics bit: the kernel reads it on the record's holder
// (static_ovgate.go grantedStaticFlag) while the record is live. gorge
// registerAnimateStaticAbilities registers the granted assignment static
// scoped to the animated object, riding the animation's own lifetime, so
// every duration the Animate accepts rides the record unchanged.
func (lc *lowerCtx) animateGrantedStatic(ps *params, name string, st cards.Static, o *lOverride, obj lRef) error {
	if obj.Kind != rSelf && obj.Kind != rTarget {
		return refuse("Animate staticAbilities$ %q on a non-permanent object", name)
	}
	sp := newParams(st.Params)
	sp.consume("Mode", "Description")
	if sp.str("ValidCard") != "Card.Self" {
		return refuse("Animate staticAbilities$ %q ValidCard$ %q", name, sp.str("ValidCard"))
	}
	if u := sp.unread(); len(u) > 0 {
		return refuse("Animate staticAbilities$ %q unread params %v", name, u)
	}
	gated := 0
	for _, ost := range lc.out.Statics {
		if ost.OvGate != 0 {
			gated++
		}
	}
	if gated >= 8 {
		return refuse("Animate staticAbilities$ grants more than 8 statics")
	}
	before := len(lc.out.Statics)
	if err := lc.lowerStatic(st); err != nil {
		return err
	}
	if len(lc.out.Statics) != before+1 {
		return refuse("Animate staticAbilities$ %q lowered to %d statics", name, len(lc.out.Statics)-before)
	}
	bit := uint8(1) << gated
	lc.out.Statics[before].OvGate = bit
	o.GrantStatics |= bit
	return nil
}

// animateGrantStatics lowers an Animate's staticAbilities$ grants: the
// permission and land-play statics (Nog, Ambitious Cadet's "You may play
// lands from your graveyard" and "You may play an additional land on each of
// your turns") and, for a single non-self body, an anthem, keyword grant or
// equipped pump (Tenth District Hero's "Other creatures you control have
// indestructible", The Irencrag's "Equipped creature gets +3/+3"); gorge
// registerAnimateStaticAbilities registers each body as a continuous effect
// of the animated object for the animation's lifetime. Each body lowers as
// the printed static it reads as on the source card, switched on by a bit of
// the override record the Animate applies (kop.Static.OvGate,
// kop.Override.GrantStatics), so the grant is source-scoped and lasts as long
// as the record. Only a permanent grant on Self reaches the source.
func (lc *lowerCtx) animateGrantStatics(ps *params, names string, o *lOverride, single bool) error {
	d := ps.str("Defined")
	if ps.str("Duration") != "Permanent" || d != "Self" && !(single && d == "") {
		return refuse("Animate staticAbilities$ %q beside a duration other than Permanent on Self", names)
	}
	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		ss, ok := cards.ParseStaticLines(lc.face.SVars[name])
		if !ok || len(ss) != 1 || ss[0].Mode != "Continuous" {
			return refuse("Animate staticAbilities$ %q", name)
		}
		_, may := ss[0].Params["MayPlay"]
		_, land := ss[0].Params["AdjustLandPlays"]
		if may == land && (!single || !animatePumpOrKeywordStatic(ss[0])) {
			return refuse("Animate staticAbilities$ %q shape", name)
		}
		gated := 0
		for _, st := range lc.out.Statics {
			if st.OvGate != 0 {
				gated++
			}
		}
		if gated >= 8 {
			return refuse("Animate staticAbilities$ grants more than 8 statics")
		}
		before := len(lc.out.Statics)
		if err := lc.lowerStatic(ss[0]); err != nil {
			return err
		}
		if len(lc.out.Statics) != before+1 {
			return refuse("Animate staticAbilities$ %q lowered to %d statics", name, len(lc.out.Statics)-before)
		}
		// A single body must lower to the layer P/T/keyword kind, whose
		// keyword/P/T walks honour OvGate (statCond); a player-hexproof or
		// stack-keyword static reads no gate and would be live unanimated.
		// Two walks read no OvGate: the attached-grant walk (grantsBetween)
		// now reads statCond, but its StatOverride arm still does not, so a
		// granted ability or trigger (AddAbility$/AddTrigger$) is refused
		// here until that is checked; and the layer walk's lfxApplies gates a
		// collected subtype grant (AddType$ subtype or ChosenType) on Cond2
		// only, so it would apply unanimated whenever the walk is on: refuse
		// both.
		if k := lc.out.Statics[before].Kind; single && !may && !land && k != "Continuous" {
			return refuse("Animate staticAbilities$ %q lowers to ungated static kind %s", name, k)
		}
		if single && !may && !land && len(lc.out.Statics[before].Grant) != 0 {
			return refuse("Animate staticAbilities$ %q grants an ability the kernel does not gate", name)
		}
		if st := &lc.out.Statics[before]; single && !may && !land && (len(st.AddSub) != 0 || st.AddChosen) {
			return refuse("Animate staticAbilities$ %q grants a subtype the kernel does not gate", name)
		}
		bit := uint8(1) << gated
		lc.out.Statics[before].OvGate = bit
		o.GrantStatics |= bit
	}
	return nil
}

// animatePumpOrKeywordStatic reports whether an Animate staticAbilities$
// grant single body is a Continuous static lowerStatic lowers as an anthem,
// keyword grant or equipped pump (AddKeyword$, AddPower$/AddToughness$)
// rather than a permission.
func animatePumpOrKeywordStatic(st cards.Static) bool {
	if strings.TrimSpace(st.Params["AddKeyword"]) != "" {
		return true
	}
	return strings.TrimSpace(st.Params["AddPower"]) != "" || strings.TrimSpace(st.Params["AddToughness"]) != ""
}

// animateGraveMayPlay lowers an Animate over Defined$ ValidGraveyard <spec>
// whose only grant is a self may-play static from the graveyard, until end
// of turn (Case of the Uneaten Feast: "creature cards in your graveyard
// gain 'You may cast this card from your graveyard' until end of turn";
// gorge effAnimate over the graveyard cards matching spec at resolution,
// each card's granted static then a self may-play grant): OpExt
// ExtMayPlayGrave, one DurEOT permission record per matching card (B
// MayPlayNonLand for Card.Self+nonLand)..
func (lc *lowerCtx) animateGraveMayPlay(ps *params, spec string) error {
	for _, k := range [...]string{"Duration", "Power", "Toughness", "Types", "Keywords", "Abilities", "Triggers", "Colors", "RemoveAllAbilities", "sVars", "Permanent"} {
		if _, ok := ps.get(k); ok {
			return refuse("graveyard Animate %s$", k)
		}
	}
	names := strings.Split(ps.str("staticAbilities"), ",")
	if len(names) != 1 {
		return refuse("graveyard Animate statics %q", ps.str("staticAbilities"))
	}
	line, ok := lc.face.SVars[strings.TrimSpace(names[0])]
	if !ok {
		return refuse("graveyard Animate static %q", names[0])
	}
	fl := 0
	want := map[string]string{"Mode": "Continuous", "AffectedZone": "Graveyard", "EffectZone": "Graveyard", "MayPlay": "True"}
	seen := 0
	for _, part := range strings.Split(line, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "$")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "Description":
			continue
		case "Affected":
			switch v {
			case "Card.Self":
				// a land from the graveyard (Kethis): no lockstep, refused
				return refuse("graveyard Animate static Affected$ %q (lands playable)", v)
			case "Card.Self+nonLand", "Card.nonLand+Self":
				fl |= kop.MayPlayNonLand
			default:
				return refuse("graveyard Animate static Affected$ %q", v)
			}
			continue
		}
		if want[k] != v {
			return refuse("graveyard Animate static param %s", k)
		}
		seen++
	}
	if seen != len(want) {
		return refuse("graveyard Animate static shape")
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "MayPlayGrave", F: f, B: cst(fl), Note: "each matching graveyard card may be cast from the graveyard this turn"})
	return nil
}

// partitionMayPlay lowers Soul Partition's permanent permission:
// Duration$ Permanent, RememberObjects$ Remembered, ForgetOnMoved$ Exile and
// two statics -- MayPlay over Card.IsRemembered in exile for the card's
// owner (MayPlayPlayer$ CardOwner) and an optional RaiseCost$ of a literal
// {N} on a spell cast that way: by an opponent of the effect's controller
// (Activator$ Opponent) or, with the Activator$ left out, by whoever casts
// it (Zero, Incorporeal Sensate -- the permission admits only the owner, so
// the owner pays).
// It emits one MayPlay over the remembered exiled card with the WhileExiled
// duration, kop.MayPlayOwner and the surcharge (B >> kop.MayPlayRaiseShift).
// It reports whether the shape was this one.
func (lc *lowerCtx) partitionMayPlay(ps *params, ss []cards.Static) (bool, error) {
	obj := lRef{Kind: rRemembered}
	if lc.exileLandStage == 1 && ps.str("RememberObjects") == "Imprinted" && len(ss) == 1 {
		// Ensign Pavel Chekov: the permission is over the imprint, the
		// chosen set (lower_repeatexilechoice.go)
		obj = lRef{Kind: rChosen}
		lc.exileLandStage = 2
	} else if ps.str("RememberObjects") != "Remembered" {
		return false, nil
	}
	if ps.str("ForgetOnMoved") != "Exile" || len(ss) < 1 || len(ss) > 2 {
		return false, nil
	}
	may := newParams(ss[0].Params)
	// without MayPlayPlayer$ the permission is the effect
	// controller's (Outrageous Robbery, Decadent Dragon, Black Cat: the
	// cards a face-down Dig exiled), alone in the Effect, with the
	// MayPlayIgnoreType$ rider as an any-type record
	owner := may.str("MayPlayPlayer") == "CardOwner"
	affected := may.str("Affected")
	_, checked := may.get("CheckSVar")
	nonLand := affected == "Card.IsRemembered+nonLand" && !owner && !checked && ps.str("Duration") == "Permanent"
	if ss[0].Mode != "Continuous" || !may.is("MayPlay", "True") || affected != "Card.IsRemembered" && !nonLand ||
		may.str("AffectedZone") != "Exile" || (!owner && (may.str("MayPlayPlayer") != "" || len(ss) != 1)) {
		return false, nil
	}
	may.consume("Mode", "Description", "MayPlay", "Affected", "AffectedZone", "MayPlayPlayer")
	flags := 0
	if lc.blind != nil && !owner && may.is("MayLookAt", "True") {
		// the look the blind Dig's exile leaves to this static
		// (lower_blinddig.go): the kernel's looker is the exiler
		may.consume("MayLookAt")
		if lc.blind.exilerOther {
			flags |= kop.MayPlayOpponent
		}
		lc.blind = nil
	}
	if owner {
		flags = kop.MayPlayOwner
	}
	if v, ok := may.get("MayPlayIgnoreType"); ok {
		if strings.TrimSpace(v) != "True" || owner {
			return true, refuse("Effect MayPlayIgnoreType %q beside a %s permission", v, map[bool]string{true: "card-owner", false: "controller"}[owner])
		}
		flags |= kop.MayPlayAnyType
	}
	if v, ok := may.get("MayPlayIgnoreColor"); ok {
		// "you may spend mana as though it were mana of any color to cast
		// it" (Brainstealer Dragon, Siphon Insight): the record's any-colour
		// rider, as the end-of-turn grant above carries it
		if strings.TrimSpace(v) != "True" || owner {
			return true, refuse("Effect MayPlayIgnoreColor %q beside a %s permission", v, map[bool]string{true: "card-owner", false: "controller"}[owner])
		}
		flags |= kop.MayPlayIgnoreColor
	}
	var valid *lFilter
	if name, ok := may.get("CheckSVar"); ok && !owner {
		spec, isValid := strings.CutPrefix(lc.face.SVars[name], "Count$Valid ")
		if _, compared := may.get("SVarCompare"); !isValid || compared {
			return true, refuse("Effect MayPlay over remembered cards with CheckSVar$ %q", lc.face.SVars[name])
		}
		f, err := lc.filter(spec)
		if err != nil {
			return true, err
		}
		valid = f
	} else if nonLand {
		flags |= kop.MayPlayNonLand // the remembered card's type, read once as the record is made
	}
	if left := may.unread(); len(left) > 0 {
		if !owner {
			return false, nil // another rider: the emblem lowering keeps it
		}
		return true, refuse("Effect MayPlay unread %v", left)
	}
	raise := 0
	if len(ss) == 2 {
		rc := newParams(ss[1].Params)
		if ss[1].Mode != "RaiseCost" || rc.str("ValidCard") != "Card.IsRemembered+CastSa Spell.MayPlaySource" ||
			rc.str("AffectedZone") != "Exile" || rc.str("Type") != "Spell" {
			return true, refuse("Effect MayPlay beside a RaiseCost shape")
		}
		// the surcharge's actor: Soul Partition names the opponent of the
		// effect's controller (Activator$ Opponent); Zero, Incorporeal
		// Sensate names none, so gorge's costActorMatches lets every
		// actor pay it -- and the paired MayPlayPlayer$ CardOwner
		// permission is the only way to cast the remembered card, so the
		// owner pays it.
		switch act := rc.str("Activator"); act {
		case "", "Opponent":
		default:
			return true, refuse("Effect RaiseCost Activator$ %q", act)
		}
		n, err := strconv.Atoi(rc.str("Amount"))
		if err != nil || n < 1 || n > 7 {
			return true, refuse("Effect RaiseCost Amount %q", rc.str("Amount"))
		}
		rc.consume("Mode", "Description", "ValidCard", "AffectedZone", "Type", "Activator", "Amount")
		if left := rc.unread(); len(left) > 0 {
			return true, refuse("Effect RaiseCost unread %v", left)
		}
		raise = n
	}
	ps.consume("Duration", "RememberObjects", "ForgetOnMoved", "StaticAbilities")
	in := lInstr{Op: "MayPlay", Obj: obj, C: zExile, CS: "WhileExiled"}
	if valid != nil {
		in.CS, in.F = "WhileValid", valid
	}
	in.B = cst(flags | raise<<kop.MayPlayRaiseShift)
	lc.emit(in)
	return true, nil
}

// animateLivePT reads an Animate staticAbilities$ grant that is one
// Mode$ Continuous static over the source itself setting its power and/or
// toughness (Beorn's Hospitality's BeornStatic, Ms. Marvel's MsMarvelStatic,
// Myth Realized's Static; gorge registerAnimateStaticAbilities). Unlike
// Animate Power$/Toughness$, which gorge reads once at resolution, the granted
// static's values are read live (layer 7b, every check): the override's
// LivePT. The Animate must reach Self, set no P/T of its own and not be
// host-bound (anything else refuses). The body's EffectZone$ is the zone the
// static would live in; gorge's self-grant whitelist admits the key and reads
// it unread (the animated object is where the static applies), so the lowering
// consumes it.
func (lc *lowerCtx) animateLivePT(name string, st cards.Static, o *lOverride, obj lRef) error {
	if obj.Kind != rSelf {
		return refuse("Animate staticAbilities$ %q on a non-Self object", name)
	}
	if o.WhileHost {
		return refuse("Animate staticAbilities$ with a host-bound duration")
	}
	sp := newParams(st.Params)
	sp.consume("Mode", "Description", "EffectZone")
	if d, ok := sp.get("AffectedDefined"); ok && d != "Self" {
		return refuse("Animate staticAbilities$ %q AffectedDefined$ %q", name, d)
	}
	if a, ok := sp.get("Affected"); ok && a != "Card.Self" {
		return refuse("Animate staticAbilities$ %q Affected$ %q", name, a)
	}
	pS, okP := sp.get("SetPower")
	tS, okT := sp.get("SetToughness")
	if !okP && !okT {
		return refuse("Animate staticAbilities$ %q sets no P/T", name)
	}
	if u := sp.unread(); len(u) > 0 {
		return refuse("Animate staticAbilities$ %q unread params %v", name, u)
	}
	pv, tv := cst(0), cst(0)
	var err error
	if okP {
		if pv, err = lc.value(pS); err != nil {
			return refuse("Animate staticAbilities$ %q SetPower$ %q: %v", name, pS, err)
		}
	}
	if okT {
		if tv, err = lc.value(tS); err != nil {
			return refuse("Animate staticAbilities$ %q SetToughness$ %q: %v", name, tS, err)
		}
	}
	o.HasSetPT, o.LivePT, o.KeepP, o.KeepT = true, true, !okP, !okT
	o.PVal, o.TVal = pv, tv
	return nil
}

// plainTapCost reports whether c is a bare {T}: no mana, no other part.
func plainTapCost(c lCost) bool {
	m := c.Mana
	if m.Generic != 0 || m.Pips != [5]int{} || len(m.Hybrid) != 0 || len(m.Twobrid) != 0 || len(m.Phyrexian) != 0 || m.X {
		return false
	}
	c.Tap, c.Mana = false, manaCost{}
	return reflect.DeepEqual(c, lCost{})
}

// noCost reports whether c has no part at all (a spell's or a resolving
// effect's mana, which has no activation cost).
func noCost(c lCost) bool {
	return !c.Tap && plainTapCostIgnoringTap(c)
}

func plainTapCostIgnoringTap(c lCost) bool {
	c.Tap = true
	return plainTapCost(c)
}
