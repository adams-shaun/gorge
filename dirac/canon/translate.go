package canon

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Translator reads kernel actions and decisions as canonical ones. S, D and KP
// are the position being read and its posed decision.
type Translator struct {
	S   *dirac.State
	D   *dirac.Decision
	KP  *dirac.Projection
	Tbl *kop.Table
	// Reg resolves a card's printed ward trigger; GName is each table card's
	// corpus name. Without them a ward trigger has no face index.
	Reg   *cards.Registry
	GName []string
	// Origin, when set, maps a card's kop ability index to its face index first.
	Origin func(dirac.CardID, uint8) (uint8, bool)
	// Generated marks a kgen table, whose ChGen* asks have their own canonical forms.
	Generated bool
}

func (r *Translator) DKind() DKind {
	if r.D.Kind == dirac.DkChoose && r.Generated {
		if k, ok := GeneratedKind(r.D.Sub); ok {
			return k
		}
	}
	return KernelDKind(r.D)
}

// KernelDKind is the canonical kind of kernel decision d (the
// generated-table sub-kinds aside: GeneratedKind).
func KernelDKind(d *dirac.Decision) DKind {
	switch d.Kind {
	case dirac.DkNone:
		return DNone
	case dirac.DkPriority:
		return DPriority
	case dirac.DkAttackers:
		return DAttackers
	case dirac.DkBlockers:
		return DBlockers
	case dirac.DkTarget:
		return DTarget
	case dirac.DkModes:
		return DModes
	case dirac.DkChoose:
		switch d.Sub {
		case dirac.ChBottom:
			return DMulligan
		case dirac.ChArrange:
			return DArrange
		case dirac.ChDiscard:
			return DModes // gorge poses an effect discard as KModes "discard"
		case dirac.ChOptional:
			return DTriggerOptional
		case dirac.ChCommanderZone:
			return DCommanderZone // CR 903.9a: the owner's put-it-there/leave ask
		}
		return DChoose
	case dirac.DkTriggerOrder:
		return DTriggerOrder
	case dirac.DkMulligan:
		return DMulligan
	}
	return DKind(0xFF)
}

// GorgeDKind is the canonical kind GorgeKind names for kernel decision d.
func GorgeDKind(d *dirac.Decision, gen bool) DKind {
	if gen && d.Kind == dirac.DkChoose {
		if k, ok := GeneratedKind(d.Sub); ok {
			return k
		}
	}
	return KernelDKind(d)
}

// GorgeKind is the gorge decision kind (decision.Kind's string) gorge poses
// kernel decision d as; gen: the game plays a generated table
// (SetSide), whose own asks gorge poses as modes.
func GorgeKind(d *dirac.Decision, gen bool) string {
	return GorgeDKind(d, gen).String()
}

// Action is a kernel action's canonical form.
func (r *Translator) Action(a dirac.Action) Canon {
	src, tgt, mode := Ref(a.Source()), Ref(a.Target()), a.Mode()
	switch a.Kind() {
	case dirac.AkPass:
		return Pack(ActPass, RefNone, RefNone, 0)
	case dirac.AkPlayLand:
		return Pack(ActPlayLand, src, RefNone, mode) // mode: the face played (a modal DFC's "modal_land" is 1)
	case dirac.AkPlot:
		if mode == dirac.FeatPlotForetell {
			cm, _ := CastModeCode("foretell") // the foretell special action is gorge's "foretell" cast option (CR 702.143a; kw_foretell.go)
			return Pack(ActCast, src, RefNone, cm)
		}
		if mode == dirac.FeatPlotSuspend {
			// the suspend special action is gorge's "suspend" cast option
			// (CR 702.62a; kw_suspend.go): the 16-entry cast-mode table is
			// full, so it canonicalises to CastModeOther, like "harmonize"
			cm, _ := CastModeCode("suspend")
			return Pack(ActCast, src, RefNone, cm)
		}
		cm, _ := CastModeCode("plot") // the hand plot special action is gorge's "plot" cast option (CR 702.170a)
		return Pack(ActCast, src, RefNone, cm)
	case dirac.AkCastCasualty:
		if r.S.FeatTeamworkCard(dirac.Obj(src)) {
			// the teamworked cast is gorge's "teamworked" cast option, in
			// CastModeOther like "bargained" (kw_teamwork.go)
			cm, _ := CastModeCode("teamworked")
			return Pack(ActCast, src, RefNone, cm)
		}
		if r.S.FeatBargainCard(dirac.Obj(src)) {
			// the bargained cast is gorge's ordinary "cast" option with
			// Mode "bargained" (kw_bargain.go); the cast-mode
			// nibble is full, so CastModeCode maps it to CastModeOther,
			// exactly as CastOptCanon does for gorge's option
			cm, _ := CastModeCode("bargained")
			return Pack(ActCast, src, RefNone, cm)
		}
		return Pack(ActCastCasualty, src, RefNone, 0) // the casualty cast (kw_casualty.go; gorge Mode "casualty")
	case dirac.AkSneak:
		return Pack(ActCastFace, src, RefNone, FaceCodeSneak) // gorge's "sneak" cast option (CR 702.190a)
	case dirac.AkWebSling:
		// gorge's "web-slinging" cast option (CR 702.188a;
		// kw_websling.go): neither the cast-mode table nor the face
		// table has room, so it canonicalises to CastModeOther, like
		// "harmonize"
		return Pack(ActCast, src, RefNone, CastModeOther)
	case dirac.AkMayFlash:
		// gorge's "mayflash" cast option (CR 702.8; kw_mayflash.go): the
		// cast-mode table is full, so it canonicalises to CastModeOther,
		// like "web-slinging"
		return Pack(ActCast, src, RefNone, CastModeOther)
	case dirac.AkCastConspire:
		// gorge's "conspired" cast option (CR 702.78a; kw_conspire.go): the
		// cast-mode table is full, so it canonicalises to CastModeOther with
		// the top alt-cost nibble set: a granted conspire pairs with a
		// bargain/teamwork cast of one card, which owns the plain CastModeOther
		return Pack(ActCast, src, RefNone, ConspiredCastCode)
	case dirac.AkBestow:
		cm, _ := CastModeCode("bestowed") // gorge's "bestowed" cast option (CR 702.103a; kw_bestow.go)
		return Pack(ActCast, src, RefNone, cm)
	case dirac.AkCast:
		if mode&dirac.FeatCastFaceDown != 0 {
			return Pack(ActCastFace, src, RefNone, FaceDownCastCanon(r.S.FeatFaceDownFamily(dirac.Obj(src))))
		}
		if mode == dirac.FeatCastAirbend {
			// an airbent card cast from exile for {2} (CR 701.65a;
			// act_airbend.go): gorge's "airbend_cast"
			return Pack(ActCastAirbend, src, RefNone, 0)
		}
		if mode&dirac.FeatCastWarpRecast != 0 {
			// a warp-exiled card cast from exile (CR 702.185a): gorge's
			// "warp_recast" face route (kw_warp.go)
			fc, _ := FaceCastCode("warp_recast")
			return Pack(ActCastFace, src, RefNone, fc)
		}
		if mode == dirac.FeatCastPlotted && r.S.FeatParadigmCard(dirac.Obj(src)) && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneExile {
			// a Paradigm card's free cast from exile (kw_paradigm.go):
			// gorge offers it through its may-play walk (paradigmMayPlay)
			cm, _ := CastModeCode("mayplay")
			return Pack(ActCast, src, RefNone, cm)
		}
		if mode&(dirac.FeatCastPlotted|dirac.FeatCastMayPlay) == dirac.FeatCastPlotted {
			// a plotted card cast from exile (CR 702.170b): gorge's
			// "plot_cast" face route (kw_plot.go)
			fc, _ := FaceCastCode("plot_cast")
			return Pack(ActCastFace, src, RefNone, fc)
		}
		if mode&dirac.FeatCastFace != 0 {
			return Pack(ActCastFace, src, RefNone, r.faceCastCanon(uint8(src), uint8(tgt))) // faces.go
		}
		if mode == 0 && tgt == 2 && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneHand {
			// the impending cast (CR 702.176a; kernel
			// kw_impending.go castImpTgt): gorge's
			// "impended" cast option, AltCostIndex 0
			return Pack(ActCastImpended, src, RefNone, 0)
		}
		if mode == 0 && tgt == 3 && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneHand {
			// the evoke cast (CR 702.74a; kernel kw_evoke.go
			// castEvokeTgt): gorge's "evoked" cast option, AltCostIndex 0
			cm, _ := CastModeCode("evoked")
			return Pack(ActCast, src, RefNone, cm)
		}
		if mode == 0 && tgt == 4 && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneHand {
			// the dash cast (CR 702.109a; kernel kw_dash.go castDashTgt):
			// gorge's "dashed" cast option, AltCostIndex 0
			cm, _ := CastModeCode("dashed")
			return Pack(ActCast, src, RefNone, cm)
		}
		if mode == 0 && tgt == dirac.FeatCastForetellTgt && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneExile {
			// a foretold card cast from exile (CR 702.143a; kw_foretell.go):
			// gorge's "foretell_cast" option, which the full cast-mode table
			// maps to CastModeOther, AltCostIndex 0. The target byte 5 is
			// shared with the overloaded hand cast (kw_overload.go); the
			// origin zone tells them apart.
			cm, _ := CastModeCode("foretell_cast")
			return Pack(ActCast, src, RefNone, cm)
		}
		if mode == 0 && tgt == dirac.FeatCastSurgeTgt && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneHand {
			// the surged cast (CR 702.117a; kernel kw_surge.go
			// castSurgeTgt): gorge's "surged" cast option, AltCostIndex 0
			cm, _ := CastModeCode("surged")
			return Pack(ActCast, src, RefNone, cm)
		}
		if mode == 0 && tgt == 5 && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneHand {
			// the overloaded cast (CR 702.96a; kernel kw_overload.go
			// castOverloadTgt): gorge's "overloaded" cast option, AltCostIndex
			// 0; the 16-entry cast-mode table is full, so CastModeOther
			return Pack(ActCast, src, RefNone, CastModeOther)
		}
		var cm uint8
		switch {
		case mode == 0 && tgt == 1 && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneHand:
			// the card's own mana alternative cost (Blasphemous Edict's {B};
			// kernel cast_altfree.go castAltTgt): gorge's
			// alternativeCosts cast, mode "", AltCostIndex 1
			cm = 1 << 4
		case mode&dirac.FeatCastWarp != 0:
			cm, _ = CastModeCode("warped") // a hand cast for its warp cost (kw_warp.go)
			cm |= tgt << 4                 // the warp instance, gorge's AltCostIndex (kw_warpgrant.go)
		case mode&1 != 0 && r.S.FeatOptCostCard(dirac.Obj(src)):
			cm = CastModeOther | 1<<4 // gorge's "optionalcost" cast, AltCostIndex 1 (CR 601.2b; cost_optional.go)
		case mode&1 != 0 && r.S.FeatOffspringCast(dirac.Obj(src)):
			cm, _ = CastModeCode("offspring") // the kicked cast of an Offspring card (kw_offspring.go)
		case mode&1 != 0 && r.S.FeatBuybackCast(dirac.Obj(src)):
			cm, _ = CastModeCode("buyback") // the kicked cast of a Buyback card (kw_buyback.go): CastModeOther, as "offspring"
		case mode&1 != 0 && r.S.FeatEntwineCast(dirac.Obj(src)):
			cm, _ = CastModeCode("entwined") // the kicked cast of an Entwine card (kw_entwine.go): CastModeOther, as "buyback"
		case mode&1 != 0:
			cm, _ = CastModeCode("kicked")
		case mode&2 != 0 && tgt == dirac.FeatEscapeTgt:
			// CR 702.138a: the escape cast a StatEscapeGrant static gives
			// (kw_escape.go); gorge's "escape" cast option, CastModeOther
			// like "mayhem" and the printed escape cast below.
			cm = CastModeOther
		case mode&2 != 0 && r.S.FeatMayhemCard(dirac.Obj(src)):
			cm, _ = CastModeCode("mayhem") // CR 702.187a: CastModeOther, as "harmonize"
		case mode&2 != 0 && r.S.FeatHarmonizeCast(dirac.Obj(src)):
			cm, _ = CastModeCode("harmonize") // CR 702.180a: CastModeOther, as "bargained"
		case mode&2 != 0 && tgt == dirac.FeatCastEscapeTgt:
			// CR 702.138a: the mode-2 escape cast (kw_escape.go); gorge's
			// "escape" cast option, CastModeOther as "mayhem". The target
			// byte marks it apart from a granted flashback on the same card.
			cm = CastModeOther
		case mode&2 != 0 && tgt == dirac.FeatRetraceTgt && r.S.FeatRetraceCard(dirac.Obj(src)):
			// the retrace cast (CR 702.81a; kw_retrace.go): the printed cost
			// plus the discard-a-land additional cost; gorge's "retrace"
			// option, CastModeOther as "harmonize". The target byte
			// separates it from a granted flashback instance of the same
			// card, which stays the plain mode-2 cast below.
			cm, _ = CastModeCode("retrace")
			if r.S.FeatJumpStartCard(dirac.Obj(src)) {
				cm, _ = CastModeCode("jumpstart") // CR 702.84a: gorge's "jumpstart" option, CastModeOther as "retrace" (kw_jumpstart.go)
			}
		case mode&2 != 0 && tgt != 0:
			cm, _ = CastModeCode("beam_me_up") // CastModeOther, as "harmonize"
			if tgt == dirac.FeatBeamGrantTgt && r.S.FeatBeamPrinted(dirac.Obj(src)) {
				cm |= 1 << 4 // a card's second beam me up permission, the grant (BeamAlt)
			}
		case mode&2 != 0:
			cm, _ = CastModeCode("flashback")
		case mode&0x18 == 0x18 && r.S.ObjZone(dirac.Obj(src)) == kop.ZoneHand:
			// a free cast from the hand (Omniscience): gorge offers it as
			// the card's alternative cost (rules mayplay.go: "casting
			// without paying the mana cost is itself an alternative cost",
			// CR 118.9), after its other alternative costs (none in FDN
			// the kernel offers): mode "", AltCostIndex 1
			cm = 1 << 4
		case mode&0x18 == 0x18 && r.S.FeatMayPayLifeCast(r.D.Player, dirac.Obj(src)):
			// a permission whose alternative cost is life (CR 118.9): gorge's
			// "mayplay" option at AltCostIndex 1 (cast_staticpaylife.go); the
			// plain cast beside it (no castFree) is AltCostIndex 0 below
			// (the target is the 1-based alternative when several statics
			// offer one each; 0 is the sole one)
			cm, _ = CastModeCode("mayplay")
			idx := tgt
			if idx == 0 {
				idx = 1
			}
			cm |= idx << 4
		case mode&8 != 0:
			cm, _ = CastModeCode("mayplay") // a generated table's play permission (castMayPlay)
		}
		return Pack(ActCast, src, RefNone, cm)
	case dirac.FeatRoomUnlock:
		c, _ := SpecialCode("unlock")
		return Pack(ActSpecial, src, RefNone, c|(mode+1)<<4) // mode = the door (face_rooms.go)
	case dirac.AkTurnFaceUp:
		c, _ := SpecialCode("turn_face_up")
		return Pack(ActSpecial, src, RefNone, c|mode<<4) // mode = the turn-up procedure
	case dirac.AkActivate:
		if r.stationAbility(uint8(src), mode) {
			c, _ := SpecialCode("station") // gorge's priority option (rules/station.go)
			return Pack(ActSpecial, src, RefNone, c)
		}
		if mode&0x80 != 0 && r.FaceAbility(uint8(src), mode, false) == 255 {
			return Pack(ActActivate, src, tgt, 255) // a granted ability, told apart by its grantor
		}
		return Pack(ActActivate, src, RefNone, r.FaceAbility(uint8(src), mode, false))
	case dirac.AkAttack:
		if dirac.IsObjTarget(dirac.TCode(tgt)) {
			return Pack(ActAttack, src, Ref(tgt), 0) // a planeswalker
		}
		if dirac.TCode(tgt) >= dirac.PlayerTarget0 && dirac.TCode(tgt) <= dirac.PlayerTarget0+(dirac.MaxSeats-1) {
			return Pack(ActAttack, src, RefPlayer+Ref(dirac.CodeSeat(dirac.TCode(tgt))), 0) // the defending player
		}
		return Pack(ActAttack, src, RefPlayer+Ref(1-r.D.Player), 0)
	case dirac.AkBlock:
		return Pack(ActBlock, src, tgt, 0)
	case dirac.AkDone:
		return Pack(ActDone, RefNone, RefNone, 0)
	case dirac.AkTarget:
		if dirac.TCode(tgt) == dirac.NoTarget {
			return Pack(ActDone, RefNone, RefNone, 0) // declining an "up to one" target
		}
		// the mode byte is the occurrence of a charm's combined target ask
		// (an object listed in two groups); 0 for every other target
		return Pack(ActTarget, RefNone, tgt, mode)
	case dirac.AkMode:
		// gorge lists only the modes that can be chosen (CR 700.2a: a mode
		// whose mandatory targets have no candidate can't), indexed densely:
		// the index is the mode's place among the offered and picked ones
		n := uint8(0)
		picked := r.S.BuildModes()
		for j := uint8(0); j < mode; j++ {
			if picked&(1<<j) != 0 {
				n++
				continue
			}
			for _, o := range r.D.Options[:r.D.N] {
				if o.Kind() == dirac.AkMode && o.Mode() == j {
					n++
					break
				}
			}
		}
		return Pack(ActMode, RefNone, RefNone, n)
	case dirac.AkKeep:
		return Pack(ActKeep, RefNone, RefNone, 0)
	case dirac.AkMulligan:
		return Pack(ActMulligan, RefNone, RefNone, 0)
	case dirac.AkOrder:
		// source = the trigger's source, target = pending index, mode =
		// its occurrence among pending triggers from that source.
		return Pack(ActOrder, src, RefNone, mode)
	case dirac.AkChoose:
		if r.Generated {
			if c, ok := r.generatedChoose(a); ok {
				return c
			}
		}
		ch := func(kind string, src Ref, m uint8) Canon {
			code, _ := ChooseCode(kind)
			return Pack(ActChoose+CanonKind(code), src, RefNone, m)
		}
		switch r.D.Sub {
		case dirac.ChX:
			return ch("x", RefNone, mode)
		case dirac.ChMultikick:
			// CR 702.43's count pick (gorge KChoose "multikick"): the
			// canonical option carries the count as its mode
			return ch("multikick", RefNone, mode)
		case dirac.ChCostDiscard, dirac.ChDiscard, dirac.ChCleanupDiscard:
			return ch("discard", src, 0)
		case dirac.ChDrawUpto:
			// gorge's draw_upto KChoose: option kind "card" over a top
			// library card (a pick is only counted)
			return ch("card", src, 0)
		case dirac.ChVoteCard:
			// gorge effCardVote's KChoose option kind "vote_card" over the
			// permanent voted for (DChoose, the default KernelDKind arm)
			return ch("vote_card", src, 0)
		case dirac.ChRecruitDiscard:
			// gorge effRecruit's KChoose option kind
			// "discard" (its canonical kind is DChoose, the default
			// KernelDKind arm; not the DModes effect-discard kind)
			return ch("discard", src, 0)
		case dirac.ChCostSacrifice, dirac.ChSacrifice:
			return ch("sacrifice", src, 0)
		case dirac.ChCostTap:
			return ch("tapcost", src, 0)
		case dirac.ChGenPayTap, dirac.ChWindowTap:
			return ch("tapcost", src, 0)
		case dirac.ChStation:
			return ch("station", src, 0)
		case dirac.ChGenDelve:
			return ch("exile", src, 0)
		case dirac.ChGenConvoke:
			return ch("convoke", src, mode)
		case dirac.ChWaterbend:
			return ch("waterbend_generic", src, 0)
		case dirac.ChSuspendCast:
			// gorge startSuspendedCast: suspend_cast_yes / suspend_cast_no,
			// both naming the suspended card (CR 702.62a)
			if mode == 1 {
				return ch("suspend_cast_yes", src, 0)
			}
			return ch("suspend_cast_no", src, 0)
		case dirac.ChDefeatCast:
			// gorge defeatedCastAsk: defeat_cast_yes / defeat_cast_no, both
			// naming the exiled battle (CR 310.11)
			if mode == 1 {
				return ch("defeat_cast_yes", src, 0)
			}
			return ch("defeat_cast_no", src, 0)
		case dirac.ChGenDiscUnless:
			// gorge effDiscard's discard_unless election: kinds "unless" /
			// "ordinary", no object (CR 701.9b)
			if mode == 1 {
				return ch("unless", RefNone, 0)
			}
			return ch("ordinary", RefNone, 0)
		case dirac.ChCostSubCtr:
			if r.S.FeatPaySubCounter() {
				return ch("trigger_cost_counter", src, mode)
			}
			if r.S.FeatSubCtrWildcard() {
				return ch("subcounter", src, mode)
			}
			return ch("subcounter", src, 0)
		case dirac.ChCostBlight:
			return ch("blightcost", src, 0)
		case dirac.ChGenBlightPick:
			return ch("blight", src, 0)
		case dirac.ChBottom:
			return Pack(ActBottom, src, RefNone, 0)
		case dirac.ChArrange:
			return Pack(ActArrange, src, RefNone, 0)
		case dirac.ChOptional:
			k := ActNo
			if mode == 1 {
				k = ActYes
			}
			return Pack(k, Ref(r.S.FeatSource()), RefNone, 0)
		case dirac.ChOptDiscMay:
			// gorge's may-election (KChoose kinds "yes"/"no", no object)
			if mode == 1 {
				return ch("yes", RefNone, 0)
			}
			return ch("no", RefNone, 0)
		case dirac.ChGenDigOpt:
			// gorge effDig's allOrNothing election (KChoose kinds "yes"/"no",
			// no object): ChangeNum$ All beside Optional$.
			if mode == 1 {
				return ch("yes", RefNone, 0)
			}
			return ch("no", RefNone, 0)
		case dirac.ChLegend:
			return ch("keep", src, 0)
		case dirac.ChCommanderZone:
			// gorge's KCommanderZone packs the commander as source and the
			// option index as mode (0 command_zone, 1 leave).
			return Pack(ActCommanderZone, src, RefNone, mode)
		case dirac.ChDivide:
			return ch("division", src, mode)
		case dirac.ChAltAdd:
			return ch("altaddcost", RefNone, mode)
		case dirac.ChCostForage:
			// gorge forageAsk: forage_exile (no object), forage_food
			if mode == 1 {
				return ch("forage_food", src, 0)
			}
			return ch("forage_exile", RefNone, 0)
		case dirac.ChCostExile:
			return ch("exilecost", src, 0)
		case dirac.ChCostReturn:
			return ch("returncost", src, 0)
		case dirac.ChCostPutToLib:
			return ch("puttolibcost", src, 0)
		case dirac.ChGenCasualty:
			return ch("casualty", src, 0) // gorge optionalSacrificeAsk: Option.Kind "casualty"
		case dirac.ChGenBargain:
			return ch("bargain", src, 0) // gorge optionalSacrificeAsk: Option.Kind "bargain" (CR 702.166)
		case dirac.ChGenConspire:
			return ch("conspire", src, 0) // gorge conspireAsk: Option.Kind "conspire" (CR 702.78a)
		case dirac.ChGenTeamwork:
			return ch("teamwork", src, 0) // gorge teamwork ask: Option.Kind "teamwork" (CR 702.194a)
		case dirac.ChGenGift:
			// gorge giftAsk: gift_decline, then one gift_promise per
			// opponent in AliveFrom order, canonical by that position
			// (the oracle's gift_promise arm); mode is 1 + the position
			if mode >= 1 {
				return ch("gift_promise", RefNone, mode-1)
			}
			return ch("gift_decline", RefNone, 0)
		case dirac.ChGenManifestDread:
			return ch("manifest_dread", src, 0)
		}
		return ch("number", RefNone, mode)
	}
	return Pack(CanonKind(0xFF), src, tgt, mode)
}

// FaceAbility maps a kernel CardDef ability index of object o's card to
// gorge's: Face.Abilities index for an activated or mana ability (gorge
// lists both as AB lines), 0x80 | Face.Triggers index for a triggered one;
// the kernel-native ward/prowess indices (253, 254) and 255 pass through.
func (r *Translator) FaceAbility(o uint8, idx uint8, _ bool) uint8 {
	if idx >= 253 || int(o) >= int(r.KP.NObj) {
		return idx
	}
	if c := r.KP.Objs[o].Card; int(c) < len(r.Tbl.Cards) && int(idx) < len(r.Tbl.Cards[c].Abilities) && r.Tbl.Cards[c].Abilities[idx].Gate.Ctr == kop.CtrMaxSpeed {
		return MaxSpeedCode(r.Tbl.Cards[c].Abilities, int(idx))
	}
	return r.CardAbility(r.KP.Objs[o].Card, idx)
}

// stationAbility reports whether ability idx of object o is a Station
// ability (kop.AfStation, kcounters).
func (r *Translator) stationAbility(o uint8, idx uint8) bool {
	if int(o) >= int(r.KP.NObj) || int(r.KP.Objs[o].Card) >= len(r.Tbl.Cards) {
		return false
	}
	abs := r.Tbl.Cards[r.KP.Objs[o].Card].Abilities
	return int(idx) < len(abs) && abs[idx].AFlags&kop.AfStation != 0
}

func (r *Translator) CardAbility(card dirac.CardID, idx uint8) uint8 {
	if int(card) < len(r.Tbl.Cards) {
		if cd := &r.Tbl.Cards[card]; cd.FaceMode == kop.FaceRoom && int(idx) < len(cd.Abilities) {
			// a Room state def's other door (kgen lower_rooms.go): gorge's
			// abilityIndex reads the FaceIdx face's lists only; an
			// UnlockDoor trigger is gorge's delayed-shaped queue entry
			// (rules face_rooms.go queueUnlockTriggers), no face index either
			if idx >= cd.RoomSplit || cd.Abilities[idx].Kind == kop.AbTriggered && cd.Abilities[idx].Trig.Event == kop.TrigUnlockDoor {
				return 255
			}
		}
	}
	if int(card) < len(r.Tbl.Cards) && ImpendTickAb(&r.Tbl.Cards[card], idx) {
		return 255 // Impending's end-step removal: gorge grants it at entry, no face index (kgen lowerImpending)
	}
	if r.Origin != nil && int(card) < len(r.Tbl.Cards) {
		if o, ok := r.Origin(card, idx); ok {
			return o
		}
	}
	if idx == 250 {
		// the kernel-native storm trigger (dirac abStorm, CR 702.40a): the
		// printed K:Storm expansion's face trigger when the card prints the
		// keyword (gorge kwStorm's addKeywordTrigger line, the same line
		// kgen's CardDef.Storm lowers to the flag), 255 when it does not —
		// a StatStackKw Storm grant's trigger is gorge's synthesized
		// StormGrantedBody (rules/storm_granted.go), which is in no face
		// list, and that walk skips a face that prints Storm, so the two
		// cannot share a card
		return r.stormAbility(card)
	}
	if idx >= 253 {
		if idx == 253 || idx == 254 {
			return r.nativeAbility(card, idx)
		}
		return idx
	}
	if int(card) >= len(r.Tbl.Cards) {
		return idx
	}
	abs := r.Tbl.Cards[card].Abilities
	if int(idx) >= len(abs) {
		return 255
	}
	if abs[idx].Gate.Ctr == kop.CtrMaxSpeed || abs[idx].Gate.Ctr == kop.CtrCharge {
		// a max-speed grant, or a Station threshold's granted trigger
		// gorge's SA is an SVar, in no Face list,
		// so a stack object reads 255 (an activation is MaxSpeedCode)
		return 255
	}
	trig := abs[idx].Kind == kop.AbTriggered
	n := 0
	for i := 0; i < int(idx); i++ {
		k := abs[i].Kind
		if abs[i].Gate.Ctr == kop.CtrMaxSpeed || abs[i].Gate.Ctr == kop.CtrCharge {
			continue
		}
		if trig && k == kop.AbTriggered || !trig && (k == kop.AbActivated || k == kop.AbMana) {
			n++
		}
	}
	if trig {
		return 0x80 | uint8(n)
	}
	return uint8(n)
}

// nativeAbility maps the kernel's synthetic ward/prowess trigger markers
// (abWard=253, abProwess=254) to the gorge face trigger index, 0x80|i. A
// keyword-minted trigger is only in the face's trigger list when the card
// prints the keyword: a face-down Disguise permanent's ward is not, and
// gorge reports it as 255 (no face trigger). The Prowess arm is always 255:
// abProwess is only a granted prowess (a printed one is the face's own
// trigger), and gorge's granted trigger carries Idx -1 even on a card that
// prints prowess.
func (r *Translator) nativeAbility(card dirac.CardID, idx uint8) uint8 {
	if r.Reg == nil || int(card) >= len(r.GName) {
		return 255
	}
	c, ok := r.Reg.Lookup(r.GName[card])
	if !ok || len(c.Faces) == 0 || c.Faces[0] == nil {
		return 255
	}
	if idx == 254 {
		return 255
	}
	want := "Ward"
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		for i, t := range f.Triggers {
			if t.Params["Keyword"] == want {
				if i < 0x7F {
					return 0x80 | uint8(i)
				}
				return 255
			}
		}
	}
	return 255
}

// stormAbility is the projected ability index of the kernel's native storm
// trigger (abStorm): the printed K:Storm expansion's face trigger (0x80|i)
// when the spell's card prints the keyword, 255 when it does not — the
// StatStackKw Storm grant's trigger is gorge's synthesized StormGrantedBody
// (rules/storm_granted.go), which is in no face list, and that walk skips a
// face that prints Storm, so the two cannot share a card. The trigger shape
// is gorge kwStorm's addKeywordTrigger line (the one kgen's lower_storm.go
// validates), not any Keyword$ Storm trigger.
func (r *Translator) stormAbility(card dirac.CardID) uint8 {
	if r.Reg == nil || int(card) >= len(r.GName) {
		return 255
	}
	c, ok := r.Reg.Lookup(r.GName[card])
	if !ok || len(c.Faces) == 0 || c.Faces[0] == nil {
		return 255
	}
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		for i, t := range f.Triggers {
			if isGorgeStormTrigger(&t) {
				if i < 0x7F {
					return 0x80 | uint8(i)
				}
				return 255
			}
		}
	}
	return 255
}

// isGorgeStormTrigger reports whether t is the printed K:Storm expansion's
// trigger: gorge kwStorm's addKeywordTrigger shape, whose body is the
// __kwStorm SVar (kgen lower_storm.go validates the same shape).
func isGorgeStormTrigger(t *cards.Trigger) bool {
	return t.Mode == "SpellCast" && t.Effect != nil &&
		strings.TrimSpace(t.Params["Keyword"]) == "Storm" &&
		strings.TrimSpace(t.Params["ValidCard"]) == "Card.Self" &&
		strings.TrimSpace(t.Params["TriggerZones"]) == "Stack" &&
		t.Effect.Kind == "DB" && t.Effect.API == "CopySpellAbility" &&
		t.Effect.Params["Defined"] == "TriggeredSpellAbility" &&
		t.Effect.Params["Amount"] == "TriggerCount$CurrentStormCount/Minus1" &&
		t.Effect.Params["MayChooseTarget"] == "True"
}

// sameRoomFace reports whether a and b are Room defs of the same printed
// card face: a Room's CardID carries only which doors are unlocked, so every
// state def carries the same Faces table (and the FaceIdx face's lists).
func (r *Translator) sameRoomFace(a, b dirac.CardID) bool {
	if int(a) >= len(r.Tbl.Cards) || int(b) >= len(r.Tbl.Cards) {
		return false
	}
	ca, cb := &r.Tbl.Cards[a], &r.Tbl.Cards[b]
	if ca.FaceMode != kop.FaceRoom || cb.FaceMode != kop.FaceRoom ||
		ca.Face != cb.Face || len(ca.Faces) == 0 || len(cb.Faces) == 0 {
		return false
	}
	return ca.Faces[0] == cb.Faces[0]
}

// LKIAbility is gorge's ability index for a stack entry whose source card
// has changed since the ability was queued. A Room's door state is another
// def of the same printed face (CR 709.5), so gorge's abilityIndex still
// finds the queued SA in the unchanged src.Face(); the LKI def's origin is
// that face's index. Any other change is a face change (a transforming DFC
// token), whose current face no longer has the SA.
func (r *Translator) LKIAbility(srcCard, card dirac.CardID, ab uint8) uint8 {
	if r.sameRoomFace(srcCard, card) {
		return r.CardAbility(card, ab)
	}
	return 255
}

// StackAbility is the projected ability index of a stack entry whose source
// currently shows srcCard while the queued ability's card and index are card
// and ab.
func (r *Translator) StackAbility(srcCard, card dirac.CardID, ab uint8) uint8 {
	if srcCard != card {
		return r.LKIAbility(srcCard, card, ab)
	}
	return r.CardAbility(card, ab)
}

// MaxSpeedBase is the first canonical ability code of a max-speed grant
// (CR 702.179e): the n-th activated ability a card's "Max speed" statics
// grant is MaxSpeedBase+n. Gorge offers it as a "granted" option naming the
// SVar (GorgeSide), the kernel as the card's Gate{CtrMaxSpeed} ability.
const MaxSpeedBase = 0xE0

// MaxSpeedCode is the canonical code of the max-speed activated ability abs[idx].
func MaxSpeedCode(abs []kop.Ability, idx int) uint8 {
	n := 0
	for i := 0; i < idx; i++ {
		if abs[i].Gate.Ctr == kop.CtrMaxSpeed && abs[i].Kind == kop.AbActivated {
			n++
		}
	}
	return MaxSpeedBase + uint8(n)
}

// ImpendTickAb reports whether ability idx of cd is Impending's synthesized
// end-step tick (kgen lower.go impendingTick: one RemoveCounter of a time
// counter from itself).
func ImpendTickAb(cd *kop.CardDef, idx uint8) bool {
	if cd.Impending == 0 || int(idx) >= len(cd.Abilities) {
		return false
	}
	a := &cd.Abilities[idx]
	return a.Kind == kop.AbTriggered && len(a.Prog) == 1 && a.Prog[0].Op == kop.OpRemoveCounter && kop.CounterKind(a.Prog[0].C) == kop.CtrTime
}

// generatedChoose canonicalises the generated table's resolution asks as gorge
// poses them (doc.go's grammar; option kinds from gorge's KChoose).
func (r *Translator) generatedChoose(a dirac.Action) (Canon, bool) {
	kind := func(k string) CanonKind {
		c, _ := ChooseCode(k)
		return ActChoose + CanonKind(c)
	}
	src, mode := a.Source(), a.Mode()
	top := RefNone
	if n := r.KP.NStack; n > 0 {
		top = Ref(r.KP.Stack[n-1].Source)
	}
	switch r.D.Sub {
	case dirac.ChWardPay:
		// gorge poses a ward cost as modes [pay, decline] (decline alone
		// when it can't be paid)
		if r.D.N == 1 || mode == 1 {
			return Pack(ActMode, RefNone, RefNone, 0), true
		}
		return Pack(ActMode, RefNone, RefNone, 1), true
	case dirac.ChGenSearch:
		return Pack(kind("search"), Ref(src), RefNone, 0), true
	case dirac.ChGenKczZone:
		// gorge's originAlternativeZones pick: the option index (0 the
		// library, 1 the alternative zone, 2 the Graveyard,Hand pair's hand)
		return Pack(kind("zone"), RefNone, RefNone, mode), true
	case dirac.ChGenHiddenPick:
		return Pack(kind("hidden_pick"), Ref(src), RefNone, 0), true
	case dirac.ChGenHandMove:
		return Pack(kind("hand_move"), Ref(src), RefNone, 0), true
	case dirac.ChGenImprint:
		// gorge zone_change.go's imprint pick (Chrome Mox)
		return Pack(kind("imprint"), Ref(src), RefNone, 0), true
	case dirac.ChGenReveal:
		return Pack(kind("reveal"), Ref(src), RefNone, 0), true
	case dirac.ChGenDig:
		return Pack(kind("dig"), Ref(src), RefNone, 0), true
	case dirac.ChGenBolster:
		return Pack(kind("counter_pick"), Ref(src), RefNone, 0), true
	case dirac.ChGenDistPick:
		// gorge's "counter_dist" pick
		return Pack(kind("counter_dist"), Ref(src), RefNone, 0), true
	case dirac.ChGenProlif:
		// gorge's proliferate pick: a permanent by Obj, a player by Player
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("proliferate"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("proliferate"), Ref(src), RefNone, 0), true
	case dirac.ChCastDivide:
		// gorge's CR 601.2d division ask: a repeatable "divide" pick over
		// the targets, a player target named by Player
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("divide"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("divide"), Ref(src), RefNone, 0), true
	case dirac.ChGenDivide:
		// gorge's resolution division: a repeatable pick over the targets
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("player"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("card"), Ref(src), RefNone, 0), true
	case dirac.ChGenEnterDefender:
		// gorge's CR 508.4 entering_defender ask: a "player" option for a
		// player, a "card" option for a planeswalker or battle (optionAction)
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("player"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("card"), Ref(src), RefNone, 0), true
	case dirac.ChKsrTarget:
		// gorge's effChangeTargets "Choose new target" KChoose: a "player"
		// option for a player, a "card" option for an object or a stack
		// ability
		if src >= dirac.PlayerTarget0 {
			return Pack(kind("player"), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
		}
		return Pack(kind("card"), Ref(src), RefNone, 0), true
	case dirac.ChGenCopyPick:
		// gorge's CopyPermanent Choices$ pick: its options are "permanent"
		return Pack(kind("permanent"), Ref(src), RefNone, 0), true
	case dirac.ChGenTokCopy:
		// gorge's chosen-copy token replacement election: "decline" first, then
		// the creature to copy (poseChosenTokenReplacement)
		if mode == 1 {
			return Pack(kind("creature"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("decline"), RefNone, RefNone, 0), true
	case dirac.ChGenFaceDownPick:
		// gorge's face_down_choice pick: the hand card to put onto the
		// battlefield face down (CR 701.58a)
		return Pack(kind("face_down_choice"), Ref(src), RefNone, 0), true
	case dirac.ChGenChooseCard, dirac.ChGenResTarget, dirac.ChGenChooseEach:
		// (ChGenChooseEach: gorge's ChooseCard per permanent type, Liliana's -9)
		return Pack(kind("card"), Ref(src), RefNone, 0), true
	case dirac.ChGenEnchant:
		return Pack(kind("enchant"), Ref(src), RefNone, 0), true
	case dirac.ChGenClone:
		// gorge's ETB "clone" ask: the templates by object, then "Enter as
		// itself" naming no object
		if mode == 1 {
			return Pack(kind("clone"), RefNone, RefNone, 0), true
		}
		return Pack(kind("clone"), Ref(src), RefNone, 0), true
	case dirac.ChEchoActionPick:
		// gorge echoActionAsk's one choose-exactly-N kind for a discard or a
		// land sacrifice paying an echo cost
		return Pack(kind("echo_action_pick"), Ref(src), RefNone, 0), true
	case dirac.ChGenPayPick:
		if blight, modes := r.S.FeatPayBlight(); blight {
			if modes {
				return Pack(kind("blightcost"), Ref(src), RefNone, 0), true
			}
			return Pack(kind("trigger_cost_blight"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("sacrifice"), Ref(src), RefNone, 0), true
	case dirac.ChGenBlightPick:
		return Pack(kind("blight"), Ref(src), RefNone, 0), true
	case dirac.ChCostBlight:
		return Pack(kind("blightcost"), Ref(src), RefNone, 0), true
	case dirac.ChGenPayReturn:
		return Pack(kind("returncost"), Ref(src), RefNone, 0), true
	case dirac.ChGenPayExile:
		return Pack(kind("exile_cost"), Ref(src), RefNone, 0), true
	case dirac.ChGenPileSplit:
		return Pack(kind("twopiles"), Ref(src), RefNone, 0), true
	case dirac.ChGenPileChoose:
		if mode == 0 {
			return Pack(kind("pile-a"), RefNone, RefNone, 0), true
		}
		return Pack(kind("pile-b"), RefNone, RefNone, 1), true
	case dirac.ChGenTopBottom:
		if mode == 1 {
			// gorge's "primary" option names no player: its zero PlayerID
			return Pack(kind("primary"), RefNone, RefPlayer, 0), true
		}
		return Pack(kind("bottom"), RefNone, RefNone, 0), true
	case dirac.ChGenPayMode:
		// gorge lists [pay, decline]; an unpayable cost leaves decline alone
		if r.D.N == 1 || mode == 1 {
			return Pack(ActMode, RefNone, RefNone, 0), true
		}
		return Pack(ActMode, RefNone, RefNone, 1), true
	case dirac.ChGenPay:
		if mode == 1 {
			return Pack(kind("trigger_cost_pay"), top, RefNone, 0), true
		}
		return Pack(kind("trigger_cost_decline"), top, RefNone, 0), true
	case dirac.ChGenBranch:
		// gorge lists only the payable choices, indexed densely
		n := uint8(0)
		for _, o := range r.D.Options[:r.D.N] {
			if o.Mode() < mode {
				n++
			}
		}
		return Pack(ActMode, RefNone, RefNone, n), true
	case dirac.ChGenCastFree:
		// gorge's modes over the castable cards, in object order, an index
		// kept after a cast
		return Pack(ActMode, RefNone, RefNone, r.S.CastFreeIndex(dirac.Obj(src))), true
	case dirac.ChGenLookAck:
		return Pack(kind("yes"), RefNone, RefNone, 0), true
	case dirac.ChGenDestAlt:
		// gorge's changezone_dest_alt: "destination" options naming the
		// searcher, primary (index 0) then alternate (index 1)
		return Pack(kind("destination"), RefNone, RefPlayer+Ref(r.D.Player), mode), true
	case dirac.ChGenExplore:
		// gorge's explore: a KChoose of "graveyard" / "top" over the revealed card
		if mode == 1 {
			return Pack(kind("top"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("graveyard"), Ref(src), RefNone, 0), true
	case dirac.ChGenEndure:
		// gorge's endure: a KChoose of "endure_counters" / "endure_spirit"
		// over the permanent
		if mode == 1 {
			return Pack(kind("endure_spirit"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("endure_counters"), Ref(src), RefNone, 0), true
	case dirac.ChGenTapOrUntap:
		// gorge's taporuntap: a KChoose of "tap" / "untap" over the permanent
		if mode == 1 {
			return Pack(kind("tap"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("untap"), Ref(src), RefNone, 0), true
	case dirac.ChGenKeepTapped:
		// gorge's untap-step election over the permanent: "untap" first,
		// then "keep_tapped"
		if mode == 1 {
			return Pack(kind("keep_tapped"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("untap"), Ref(src), RefNone, 0), true
	case dirac.ChGenPeekReveal:
		// gorge's reveal_optional over a PeekAndReveal: yes carries the
		// top card as its Obj, no names nothing
		if mode == 1 {
			return Pack(kind("yes"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("no"), RefNone, RefNone, 0), true
	case dirac.ChGenConfirm:
		// gorge's hand_move_confirm: a KChoose of yes / no
		if mode == 1 {
			return Pack(kind("yes"), RefNone, RefNone, 0), true
		}
		return Pack(kind("no"), RefNone, RefNone, 0), true
	case dirac.ChRoom:
		// gorge's venture "room" KChoose: the option index in the room's
		// printed NextRoom$ order
		return Pack(kind("room"), RefNone, RefNone, mode), true
	case dirac.ChCloneCopyPick:
		// gorge's clone_choice KChoose over battlefield permanents
		return Pack(kind("permanent"), Ref(src), RefNone, 0), true
	case dirac.ChCloneBatchPick:
		// gorge's clone_choice KChoose over the cards of an exile pool
		return Pack(kind("card"), Ref(src), RefNone, 0), true
	case dirac.ChHideaway:
		// gorge's hideaway_pick KChoose over the top cards
		return Pack(kind("hideaway"), Ref(src), RefNone, 0), true
	case dirac.ChTrigX:
		// gorge's triggeredCostXAsk: a KChoose of "trigger_cost_x", the
		// option's Amount the X; canonical like the NumTimes count (canon.go),
		// the kernel's X pick (Shanna)
		return Pack(kind("x"), RefNone, RefNone, mode), true
	case dirac.ChWardDiscard:
		// gorge's ward_discard KChoose over the payer's hand
		return Pack(kind("ward_discard"), Ref(src), RefNone, 0), true
	case dirac.ChWardAlt:
		// gorge's ward_alt KChoose: a ward_discard per hand card, then
		// ward_mana for the {N}
		if mode == 1 {
			return Pack(kind("ward_mana"), RefNone, RefNone, 0), true
		}
		return Pack(kind("ward_discard"), Ref(src), RefNone, 0), true
	case dirac.ChWardSac:
		// gorge's ward_sac KChoose over the payer's matching permanents
		return Pack(kind("ward_sac"), Ref(src), RefNone, 0), true
	case dirac.ChCostBehold:
		// gorge's beholdCostAsk KChoose over the battlefield then hand
		// candidates
		return Pack(kind("beholdcost"), Ref(src), RefNone, 0), true
	case dirac.ChCostReveal:
		// gorge's AdvanceUnless "revealcost" pick over the payer's hand
		return Pack(kind("revealcost"), Ref(src), RefNone, 0), true
	case dirac.ChCostEvidence:
		// gorge's evidence KChoose over the payer's graveyard
		return Pack(kind("evidence"), Ref(src), RefNone, 0), true
	case dirac.ChWardWaterbend:
		// gorge's ward_waterbend KChoose over the payer's untapped artifacts and creatures
		return Pack(kind("ward_waterbend"), Ref(src), RefNone, 0), true
	case dirac.ChWardEvidence:
		// gorge's ward_evidence KChoose over the payer's graveyard
		return Pack(kind("ward_evidence"), Ref(src), RefNone, 0), true
	case dirac.ChGenTurnUpPick:
		// gorge setStateChoose's "setstate_pick" KChoose over the Choices$
		// permanents that would turn face up
		return Pack(kind("setstate_pick"), Ref(src), RefNone, 0), true
	case dirac.ChGenUntapPick:
		// gorge's "untap" KChoose over the UntapType$ candidates
		return Pack(kind("untap"), Ref(src), RefNone, 0), true
	case dirac.ChGenDiscUnless:
		// gorge's discard_unless: a KChoose of unless / ordinary, both
		// canonical kinds naming no object
		if mode == 1 {
			return Pack(kind("unless"), RefNone, RefNone, 0), true
		}
		return Pack(kind("ordinary"), RefNone, RefNone, 0), true
	case dirac.ChGenMayShuffle:
		if mode == 1 {
			return Pack(kind("yes"), RefNone, RefNone, 0), true
		}
		return Pack(kind("no"), RefNone, RefNone, 0), true
	case dirac.ChRemCtrKind:
		// gorge's counter_kind pick (removeCounterAny), compared by the
		// counter kind (mode), not gorge's carrier-order position
		return Pack(kind("counter"), RefNone, RefNone, mode), true
	case dirac.ChGenPlayer:
		// gorge effChoosePlayer's "player" option names the seat
		return Pack(kind("player"), RefNone, RefPlayer+Ref(mode), 0), true
	case dirac.ChGenColor:
		// gorge indexes its colour options densely: with an Exclude$
		// colour dropped (Citadel Gate) the index is the colour's rank
		// among the offered ones (identity over all five)
		n := uint8(0)
		for _, o := range r.D.Options[:r.D.N] {
			if o.Mode() < mode {
				n++
			}
		}
		return Pack(kind("color"), RefNone, RefNone, n), true
	case dirac.ChGenManaChoice:
		// a resolving Mana effect's colour pick (Chandra, Hope's
		// Beacon): gorge's "mana" option, valued by its colour
		return Pack(kind("mana"), RefNone, RefNone, mode), true
	case dirac.ChGenType:
		// the vocabulary index in (target, mode), as gorge's labelValue
		return Pack(kind("type"), RefNone, Ref(a.Target()), mode), true
	case dirac.ChGenName:
		// the chosen card's name as gorge's "name" option carries it: the
		// label's canonical 16-bit value in (target, mode). The kernel's mode
		// is the CardID (repl_choosename.go); GName names it.
		if int(mode) >= len(r.GName) {
			return 0, false
		}
		v := NameValue(r.GName[mode])
		var earlier []string
		if r.Tbl != nil {
			names := r.Tbl.NameCards
			if r.D.N > 0 && len(r.Tbl.NameCardsAny) > 0 && len(r.D.Options[:r.D.N]) == len(r.Tbl.NameCardsAny) {
				names = r.Tbl.NameCardsAny // the any-card ask's list (lands included)
			}
			for _, c := range names {
				if int(c) < int(mode) {
					earlier = append(earlier, r.GName[c])
				}
			}
		}
		src := RefNone
		if n := NameOrdinal(v, earlier); n > 0 {
			src = Ref(n) // two names that hash alike stay two options
		}
		return Pack(kind("name"), src, Ref(v>>8), uint8(v)), true
	case dirac.ChGenReplOrder:
		// gorge's KReplacement order ask: one option per replacement, its source
		// and occurrence among that source's options (the kernel mode is the
		// global scan index; two replacements can share a source, e.g. a
		// recipient's prevention and its shield counter)
		occ := uint8(0)
		for _, o := range r.D.Options[:r.D.N] {
			if o.Source() == src && o.Mode() < mode {
				occ++
			}
		}
		return Pack(ActReplacement, Ref(src), RefNone, occ), true
	case dirac.ChGenPip:
		// gorge's pip announcement (rules manaAsk): pay_W..pay_G
		return Pack(kind("pay"), RefNone, RefNone, mode), true
	case dirac.ChGenLeyline:
		if mode == 1 {
			return Pack(kind("opening_yes"), Ref(src), RefNone, 0), true
		}
		return Pack(kind("opening_no"), Ref(src), RefNone, 0), true
	case dirac.ChRoomDoorHalf:
		// gorge effUnlockDoor's "Lock or unlock a door?" KChoose
		if mode == 1 {
			return Pack(kind("lock"), RefNone, RefNone, 0), true
		}
		return Pack(kind("unlock"), RefNone, RefNone, 0), true
	case dirac.ChRoomDoor:
		// gorge effUnlockDoor's "Choose a Room door" KChoose: both doors name
		// the same Room and gorge's disambiguate gives the k-th repeat mode k
		// in option (face) order; the kernel carries that ordinal in Target.
		return Pack(kind("card"), Ref(src), RefNone, uint8(a.Target())), true
	}
	return 0, false
}

func GeneratedKind(sub dirac.ChoiceKind) (DKind, bool) {
	switch sub {
	case dirac.ChGenBranch, dirac.ChGenCastFree, dirac.ChGenPayMode, dirac.ChWardPay:
		return DModes, true
	case dirac.ChGenReplOrder:
		return DReplacement, true
	}
	return 0, false
}

// NameValue is a card name's 16-bit canonical choice value: 0x8000 with the
// low 15 bits of the name's FNV-1a hash. gorge's "name" option and the
// kernel's ChGenName option both carry it in (target, mode).
func NameValue(name string) uint16 {
	h := uint32(2166136261)
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	return 0x8000 | uint16(h&0x7FFF)
}

// NameOrdinal is how many of the names listed before one with canonical value
// v share that value: 0 for the first of a hash collision, k for the k-th
// repeat. Two cards whose names hash alike (Jadzia Dax, Living Legacy and
// Xenobotanist) would otherwise canonicalise to one option, which gorge's
// disambiguate drops and the kernel does not; both sides carry the ordinal
// in the option's otherwise unused source, counted over the same name list in
// the same (sorted) order.
func NameOrdinal(v uint16, earlier []string) uint8 {
	n := 0
	for _, e := range earlier {
		if NameValue(e) == v {
			n++
		}
	}
	return uint8(min(n, 255))
}

// ConspiredCastCode is the cast-mode byte of the conspired cast: CastModeOther
// with the alt-cost nibble at its top value, distinct from the bargained and
// teamworked casts (CastModeOther, nibble 0) and the optional-cost casts
// (nibble i+1 of a short list).
const ConspiredCastCode = CastModeOther | 15<<4
