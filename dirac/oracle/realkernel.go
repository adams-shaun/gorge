package oracle

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// realkernel.go adapts package kernel
// to the oracle's Side: it canonicalises the kernel's Actions (Ak*) into the
// oracle's grammar (doc.go) and converts dirac.Projection into the oracle's
// Projection. Where the kernel's action does not carry what the canonical
// form needs (a trigger-order pick names a pending-trigger index, not the
// trigger's source; a DkChoose pick does not say what is chosen), the
// translation is the best the exported API allows and is listed in
// RealKernelGaps.

// RealKernelGaps lists what the adapter cannot translate 1:1 with the
// kernel's exported API (the API the oracle needs beyond the contract).
var RealKernelGaps = []string{
	"ChOptional names no source: the adapter takes the resolving entry's object (State.FeatSource).",
	"ChWardPay has no gorge counterpart in the canonical grammar yet (mapped to choose:number).",
}

// KernelMatchup builds the gorge matchup for two kernel decks of tbl: each
// card resolved in reg by its kernel name, through names (kernel name ->
// gorge name) when it maps one.
func KernelMatchup(reg *cards.Registry, tbl *kop.Table, labels [2]string, decks [2][]kop.CardID, names map[string]string, mulligans int) (*Matchup, error) {
	var gd [2][]*cards.Card
	for s := 0; s < 2; s++ {
		for _, id := range decks[s] {
			n := tbl.Cards[id].Name
			if g, ok := names[n]; ok {
				n = g
			}
			c, ok := reg.Lookup(n)
			if !ok {
				return nil, fmt.Errorf("kernel card %q: %q not in the gorge corpus", tbl.Cards[id].Name, n)
			}
			gd[s] = append(gd[s], c)
		}
	}
	return NewMatchup(reg, labels, gd, mulligans)
}

// RealKernel is a SideFactory over package kernel playing tbl. The matchup's
// gorge decks are mapped back to tbl's cards by name (names: kernel name ->
// gorge name, as given to KernelMatchup). Elision follows cfg.PassElision
// (dirac.Config.NoElide when off).
func RealKernel(tbl *kop.Table, names map[string]string, cfg Config) SideFactory {
	rev := map[string]kop.CardID{}
	for i, c := range tbl.Cards {
		n := c.Name
		if g, ok := names[n]; ok {
			n = g
		}
		if _, dup := rev[n]; !dup {
			rev[n] = kop.CardID(i)
		}
	}
	return func(m *Matchup, seed uint64) (Side, error) {
		var decks [2][]kop.CardID
		for s := 0; s < 2; s++ {
			for _, c := range m.Decks[s] {
				id, ok := rev[cardName(c)]
				if !ok {
					return nil, fmt.Errorf("realkernel: %q is not in table %s", cardName(c), tbl.Name)
				}
				decks[s] = append(decks[s], id)
			}
		}
		r := &realSide{tbl: tbl, m: m}
		m.chars = newCharVocab(tbl)
		r.gname = make([]string, len(tbl.Cards))
		r.tidx = make([]uint8, len(tbl.Cards))
		for i, c := range tbl.Cards {
			n := c.Name
			if len(c.Faces) > 0 {
				n = tbl.Cards[c.Faces[0]].Name // a face is its card (gorge names the front)
			}
			if g, ok := names[n]; ok {
				n = g
			}
			r.gname[i] = n
			r.tidx[i] = NoCardIndex
			for j, t := range m.Table {
				if t == n {
					r.tidx[i] = uint8(j)
				}
			}
		}
		r.kcfg = dirac.LockstepConfig(cfg.surface())
		r.kcfg.NoElide = !cfg.PassElision
		r.kcfg.Mulligans = uint8(m.Mulligans)
		r.kcfg.RecordChance = cfg.RecordChance
		r.kcfg.ArrangeRestOrder = cfg.ArrangeRestOrder
		r.s.Init(tbl, decks[0], decks[1], seed, r.kcfg)
		return r, nil
	}
}

// surface is the kernel surface cfg's GorgeAutopaySurface selects.
func (cfg Config) surface() dirac.Surface {
	if cfg.GorgeAutopaySurface {
		return dirac.SurfaceAutopay
	}
	return dirac.SurfaceFull
}

type realSide struct {
	s     dirac.State
	kcfg  dirac.Config // what s was built with
	tbl   *kop.Table
	m     *Matchup
	gname []string // per kernel CardID: the gorge card name
	tidx  []uint8  // per kernel CardID: the matchup table index
	d     dirac.Decision
	opts  []Canon
	kp    dirac.Projection
	// nameless: tokens seen as dirac.ProjNameless. Gorge keeps a copied token's
	// blank card after it leaves, and the kernel's object holds the printed one.
	nameless [(MaxInst + 63) / 64]uint64
	// origin, when set, maps a card's kop ability index to gorge's
	// (SetSide: set.AbilityOrigin).
	origin func(dirac.CardID, uint8) (uint8, bool)
	// choose, when set, canonicalises an AkChoose by the decision's Sub
	// first (FDNKernel: the generated table's ChGen* kinds).
	choose func(r *realSide, a dirac.Action) (Canon, bool)
	// gdk, when set, names the decision kind of a DkChoose by its Sub first.
	gdk func(r *realSide) (DKind, bool)
}

// OccupancyHook, when set, sees the kernel's state at every decision the
// lockstep poses (kfuzz -stats). The kernel itself carries no tracking.
var OccupancyHook func(*dirac.State)

// KernelStateOf is the kernel state behind a RealKernel side, the decision its
// last Step posed and that decision's options as Canons, in the decision's order.
// All nil for any other side.
func KernelStateOf(s Side) (*dirac.State, *dirac.Decision, []Canon) {
	r, ok := s.(*realSide)
	if !ok {
		return nil, nil, nil
	}
	return &r.s, &r.d, r.opts
}

func (r *realSide) Step() CDecision {
	r.s.StepInto(&r.d)
	if OccupancyHook != nil {
		OccupancyHook(&r.s)
	}
	r.s.Project(&r.kp)
	r.opts = r.opts[:0]
	for _, a := range r.d.Options[:r.d.N] {
		r.opts = append(r.opts, r.canon(a))
	}
	out := CDecision{Kind: r.dkind(), Player: r.d.Player, Options: append([]Canon(nil), r.opts...)}
	sortCanon(out.Options)
	return out
}

func (r *realSide) Apply(c Canon) {
	for i, x := range r.opts {
		if x == c {
			r.s.Apply(r.d.Options[i])
			return
		}
	}
	panic(fmt.Sprintf("realkernel: %v is not one of the kernel's options", c))
}

func (r *realSide) Terminal() (bool, int8) { return r.s.Terminal() }

// Decided implements Side: the kernel ends the game when it is decided, so
// its decided result is its terminal result.
func (r *realSide) Decided() (bool, int8) { return r.s.Terminal() }

// Dirty implements Side. The kernel has no commit boundary to expose, so it
// reports dirty at every posed decision; mixed runs use the gorge side's
// Dirty as the comparison cadence either way.
func (r *realSide) Dirty() bool { return true }

// Turn implements Side.
func (r *realSide) Turn() int { return r.s.Turn() }

// Err implements Side: the kernel has no deferred internal failure; a
// rejected action panics.
func (r *realSide) Err() error { return nil }

func (r *realSide) dkind() DKind {
	if r.d.Kind == dirac.DkChoose && r.gdk != nil {
		if k, ok := r.gdk(r); ok {
			return k
		}
	}
	return kernelDKind(&r.d)
}

// kernelDKind is the canonical kind of kernel decision d (the
// generated-table sub-kinds aside: fdnSubKind).
func kernelDKind(d *dirac.Decision) DKind {
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
		if k, ok := fdnSubKind(d.Sub); ok {
			return k
		}
	}
	return kernelDKind(d)
}

// GorgeKind is the gorge decision kind (decision.Kind's string) gorge poses
// kernel decision d as; gen: the game plays a generated table
// (SetSide), whose own asks gorge poses as modes.
func GorgeKind(d *dirac.Decision, gen bool) string {
	return GorgeDKind(d, gen).String()
}

// canon is a kernel action's canonical form (RealKernelGaps for the guesses).
func (r *realSide) canon(a dirac.Action) Canon {
	src, tgt, mode := a.Source(), a.Target(), a.Mode()
	switch a.Kind() {
	case dirac.AkPass:
		return Pack(ActPass, RefNone, RefNone, 0)
	case dirac.AkPlayLand:
		return Pack(ActPlayLand, src, RefNone, mode) // mode: the face played (a modal DFC's "modal_land" is 1)
	case dirac.AkPlot:
		cm, _ := castModeCode("plot") // the hand plot special action is gorge's "plot" cast option (CR 702.170a)
		return Pack(ActCast, src, RefNone, cm)
	case dirac.AkCastCasualty:
		if r.s.FeatTeamworkCard(dirac.Obj(src)) {
			// the teamworked cast is gorge's "teamworked" cast option, in
			// castModeOther like "bargained" (kw_teamwork.go)
			cm, _ := castModeCode("teamworked")
			return Pack(ActCast, src, RefNone, cm)
		}
		if r.s.FeatBargainCard(dirac.Obj(src)) {
			// the bargained cast is gorge's ordinary "cast" option with
			// Mode "bargained" (kw_bargain.go); the cast-mode
			// nibble is full, so castModeCode maps it to castModeOther,
			// exactly as castOptCanon does for gorge's option
			cm, _ := castModeCode("bargained")
			return Pack(ActCast, src, RefNone, cm)
		}
		return Pack(ActCastCasualty, src, RefNone, 0) // the casualty cast (kw_casualty.go; gorge Mode "casualty")
	case dirac.AkSneak:
		return Pack(ActCastFace, src, RefNone, faceCodeSneak) // gorge's "sneak" cast option (CR 702.190a)
	case dirac.AkWebSling:
		// gorge's "web-slinging" cast option (CR 702.188a;
		// kw_websling.go): neither the cast-mode table nor the face
		// table has room, so it canonicalises to castModeOther, like
		// "harmonize"
		return Pack(ActCast, src, RefNone, castModeOther)
	case dirac.AkBestow:
		cm, _ := castModeCode("bestowed") // gorge's "bestowed" cast option (CR 702.103a; kw_bestow.go)
		return Pack(ActCast, src, RefNone, cm)
	case dirac.AkCast:
		if mode&dirac.FeatCastFaceDown != 0 {
			return Pack(ActCastFace, src, RefNone, faceDownCastCanon(r.s.FeatFaceDownFamily(dirac.Obj(src))))
		}
		if mode == dirac.FeatCastAirbend {
			// an airbent card cast from exile for {2} (CR 701.65a;
			// act_airbend.go): gorge's "airbend_cast"
			return Pack(ActCastAirbend, src, RefNone, 0)
		}
		if mode&dirac.FeatCastWarpRecast != 0 {
			// a warp-exiled card cast from exile (CR 702.185a): gorge's
			// "warp_recast" face route (kw_warp.go)
			fc, _ := faceCastCode("warp_recast")
			return Pack(ActCastFace, src, RefNone, fc)
		}
		if mode == dirac.FeatCastPlotted && r.s.FeatParadigmCard(dirac.Obj(src)) && r.s.ObjZone(dirac.Obj(src)) == kop.ZoneExile {
			// a Paradigm card's free cast from exile (kw_paradigm.go):
			// gorge offers it through its may-play walk (paradigmMayPlay)
			cm, _ := castModeCode("mayplay")
			return Pack(ActCast, src, RefNone, cm)
		}
		if mode&(dirac.FeatCastPlotted|dirac.FeatCastMayPlay) == dirac.FeatCastPlotted {
			// a plotted card cast from exile (CR 702.170b): gorge's
			// "plot_cast" face route (kw_plot.go)
			fc, _ := faceCastCode("plot_cast")
			return Pack(ActCastFace, src, RefNone, fc)
		}
		if mode&dirac.FeatCastFace != 0 {
			return Pack(ActCastFace, src, RefNone, r.faceCastCanon(src, tgt)) // faces.go
		}
		if mode == 0 && tgt == 2 && r.s.ObjZone(dirac.Obj(src)) == kop.ZoneHand {
			// the impending cast (CR 702.176a; kernel
			// kw_impending.go castImpTgt): gorge's
			// "impended" cast option, AltCostIndex 0
			return Pack(ActCastImpended, src, RefNone, 0)
		}
		if mode == 0 && tgt == 3 && r.s.ObjZone(dirac.Obj(src)) == kop.ZoneHand {
			// the evoke cast (CR 702.74a; kernel kw_evoke.go
			// castEvokeTgt): gorge's "evoked" cast option, AltCostIndex 0
			cm, _ := castModeCode("evoked")
			return Pack(ActCast, src, RefNone, cm)
		}
		var cm uint8
		switch {
		case mode == 0 && tgt == 1 && r.s.ObjZone(dirac.Obj(src)) == kop.ZoneHand:
			// the card's own mana alternative cost (Blasphemous Edict's {B};
			// kernel cast_altfree.go castAltTgt): gorge's
			// alternativeCosts cast, mode "", AltCostIndex 1
			cm = 1 << 4
		case mode&dirac.FeatCastWarp != 0:
			cm, _ = castModeCode("warped") // a hand cast for its warp cost (kw_warp.go)
		case mode&1 != 0 && r.s.FeatOptCostCard(dirac.Obj(src)):
			cm = castModeOther | 1<<4 // gorge's "optionalcost" cast, AltCostIndex 1 (CR 601.2b; cost_optional.go)
		case mode&1 != 0 && r.s.FeatOffspringCast(dirac.Obj(src)):
			cm, _ = castModeCode("offspring") // the kicked cast of an Offspring card (kw_offspring.go)
		case mode&1 != 0:
			cm, _ = castModeCode("kicked")
		case mode&2 != 0 && r.s.FeatMayhemCard(dirac.Obj(src)):
			cm, _ = castModeCode("mayhem") // CR 702.187a: castModeOther, as "harmonize"
		case mode&2 != 0 && r.s.FeatHarmonizeCard(dirac.Obj(src)):
			cm, _ = castModeCode("harmonize") // CR 702.180a: castModeOther, as "bargained"
		case mode&2 != 0:
			cm, _ = castModeCode("flashback")
		case mode&0x18 == 0x18 && r.s.ObjZone(dirac.Obj(src)) == kop.ZoneHand:
			// a free cast from the hand (Omniscience): gorge offers it as
			// the card's alternative cost (rules mayplay.go: "casting
			// without paying the mana cost is itself an alternative cost",
			// CR 118.9), after its other alternative costs (none in FDN
			// the kernel offers): mode "", AltCostIndex 1
			cm = 1 << 4
		case mode&8 != 0:
			cm, _ = castModeCode("mayplay") // a generated table's play permission (castMayPlay)
		}
		return Pack(ActCast, src, RefNone, cm)
	case dirac.FeatRoomUnlock:
		c, _ := specialCode("unlock")
		return Pack(ActSpecial, src, RefNone, c|(mode+1)<<4) // mode = the door (face_rooms.go)
	case dirac.AkTurnFaceUp:
		c, _ := specialCode("turn_face_up")
		return Pack(ActSpecial, src, RefNone, c|mode<<4) // mode = the turn-up procedure
	case dirac.AkActivate:
		if r.stationAbility(src, mode) {
			c, _ := specialCode("station") // gorge's priority option (rules/station.go)
			return Pack(ActSpecial, src, RefNone, c)
		}
		if mode&0x80 != 0 && r.faceAbility(src, mode, false) == 255 {
			return Pack(ActActivate, src, tgt, 255) // a granted ability, told apart by its grantor
		}
		return Pack(ActActivate, src, RefNone, r.faceAbility(src, mode, false))
	case dirac.AkAttack:
		if dirac.IsObjTarget(tgt) {
			return Pack(ActAttack, src, Ref(tgt), 0) // a planeswalker
		}
		return Pack(ActAttack, src, RefPlayer+Ref(1-r.d.Player), 0)
	case dirac.AkBlock:
		return Pack(ActBlock, src, tgt, 0)
	case dirac.AkDone:
		return Pack(ActDone, RefNone, RefNone, 0)
	case dirac.AkTarget:
		if tgt == dirac.NoTarget {
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
		picked := r.s.BuildModes()
		for j := uint8(0); j < mode; j++ {
			if picked&(1<<j) != 0 {
				n++
				continue
			}
			for _, o := range r.d.Options[:r.d.N] {
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
		if r.choose != nil {
			if c, ok := r.choose(r, a); ok {
				return c
			}
		}
		ch := func(kind string, src Ref, m uint8) Canon {
			code, _ := chooseCode(kind)
			return Pack(ActChoose+CanonKind(code), src, RefNone, m)
		}
		switch r.d.Sub {
		case dirac.ChX:
			return ch("x", RefNone, mode)
		case dirac.ChCostDiscard, dirac.ChDiscard, dirac.ChCleanupDiscard:
			return ch("discard", src, 0)
		case dirac.ChRecruitDiscard:
			// gorge effRecruit's KChoose option kind
			// "discard" (its canonical kind is DChoose, the default
			// kernelDKind arm; not the DModes effect-discard kind)
			return ch("discard", src, 0)
		case dirac.ChCostSacrifice, dirac.ChSacrifice:
			return ch("sacrifice", src, 0)
		case dirac.ChCostTap:
			return ch("tapcost", src, 0)
		case dirac.ChGenPayTap:
			return ch("tapcost", src, 0)
		case dirac.ChStation:
			return ch("station", src, 0)
		case dirac.ChGenConvoke:
			return ch("convoke", src, mode)
		case dirac.ChWaterbend:
			return ch("waterbend_generic", src, 0)
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
			if r.s.FeatPaySubCounter() {
				return ch("trigger_cost_counter", src, mode)
			}
			if r.s.FeatSubCtrWildcard() {
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
			return Pack(k, Ref(r.s.FeatSource()), RefNone, 0)
		case dirac.ChOptDiscMay:
			// gorge's may-election (KChoose kinds "yes"/"no", no object)
			if mode == 1 {
				return ch("yes", RefNone, 0)
			}
			return ch("no", RefNone, 0)
		case dirac.ChLegend:
			return ch("keep", src, 0)
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
		case dirac.ChGenCasualty:
			return ch("casualty", src, 0) // gorge optionalSacrificeAsk: Option.Kind "casualty"
		case dirac.ChGenBargain:
			return ch("bargain", src, 0) // gorge optionalSacrificeAsk: Option.Kind "bargain" (CR 702.166)
		case dirac.ChGenTeamwork:
			return ch("teamwork", src, 0) // gorge teamwork ask: Option.Kind "teamwork" (CR 702.194a)
		case dirac.ChGenGift:
			// gorge giftAsk: gift_decline, then gift_promise naming the
			// opponent (Option.Player; no object, so canonical bare)
			if mode == 1 {
				return ch("gift_promise", RefNone, 0)
			}
			return ch("gift_decline", RefNone, 0)
		case dirac.ChGenManifestDread:
			return ch("manifest_dread", src, 0)
		}
		return ch("number", RefNone, mode)
	}
	return Pack(CanonKind(0xFF), src, tgt, mode)
}

// faceAbility maps a kernel CardDef ability index of object o's card to
// gorge's: Face.Abilities index for an activated or mana ability (gorge
// lists both as AB lines), 0x80 | Face.Triggers index for a triggered one;
// the kernel-native ward/prowess indices (253, 254) and 255 pass through.
func (r *realSide) faceAbility(o uint8, idx uint8, _ bool) uint8 {
	if idx >= 253 || int(o) >= int(r.kp.NObj) {
		return idx
	}
	if c := r.kp.Objs[o].Card; int(c) < len(r.tbl.Cards) && int(idx) < len(r.tbl.Cards[c].Abilities) && r.tbl.Cards[c].Abilities[idx].Gate.Ctr == kop.CtrMaxSpeed {
		return maxSpeedCode(r.tbl.Cards[c].Abilities, int(idx))
	}
	return r.cardAbility(r.kp.Objs[o].Card, idx)
}

// stationAbility reports whether ability idx of object o is a Station
// ability (kop.AfStation, kcounters).
func (r *realSide) stationAbility(o uint8, idx uint8) bool {
	if int(o) >= int(r.kp.NObj) || int(r.kp.Objs[o].Card) >= len(r.tbl.Cards) {
		return false
	}
	abs := r.tbl.Cards[r.kp.Objs[o].Card].Abilities
	return int(idx) < len(abs) && abs[idx].AFlags&kop.AfStation != 0
}

func (r *realSide) cardAbility(card dirac.CardID, idx uint8) uint8 {
	if int(card) < len(r.tbl.Cards) {
		if cd := &r.tbl.Cards[card]; cd.FaceMode == kop.FaceRoom && int(idx) < len(cd.Abilities) {
			// a Room state def's other door (kgen lower_rooms.go): gorge's
			// abilityIndex reads the FaceIdx face's lists only; an
			// UnlockDoor trigger is gorge's delayed-shaped queue entry
			// (rules face_rooms.go queueUnlockTriggers), no face index either
			if idx >= cd.RoomSplit || cd.Abilities[idx].Kind == kop.AbTriggered && cd.Abilities[idx].Trig.Event == kop.TrigUnlockDoor {
				return 255
			}
		}
	}
	if int(card) < len(r.tbl.Cards) && impendTickAb(&r.tbl.Cards[card], idx) {
		return 255 // Impending's end-step removal: gorge grants it at entry, no face index (kgen lowerImpending)
	}
	if r.origin != nil && int(card) < len(r.tbl.Cards) {
		if o, ok := r.origin(card, idx); ok {
			return o
		}
	}
	if idx >= 253 {
		if idx == 253 || idx == 254 {
			return r.nativeAbility(card, idx)
		}
		return idx
	}
	if int(card) >= len(r.tbl.Cards) {
		return idx
	}
	abs := r.tbl.Cards[card].Abilities
	if int(idx) >= len(abs) {
		return 255
	}
	if abs[idx].Gate.Ctr == kop.CtrMaxSpeed || abs[idx].Gate.Ctr == kop.CtrCharge {
		// a max-speed grant, or a Station threshold's granted trigger
		// gorge's SA is an SVar, in no Face list,
		// so a stack object reads 255 (an activation is maxSpeedCode)
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
func (r *realSide) nativeAbility(card dirac.CardID, idx uint8) uint8 {
	if r.m == nil || r.m.reg == nil || int(card) >= len(r.gname) {
		return 255
	}
	c, ok := r.m.reg.Lookup(r.gname[card])
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

// CapacityStall names the capacity the kernel's game was stopped at
// (State.StallReason), "" when it was not.
func (r *realSide) CapacityStall() string {
	return dirac.StallName(r.s.StallReason())
}

// StallIsUnmodelled reports whether the game stopped at a rule gap rather than a
// declared capacity (dirac.StallClassOf).
func (r *realSide) StallIsUnmodelled() bool {
	return dirac.StallIsUnmodelled(r.s.StallReason())
}

// sameRoomFace reports whether a and b are Room defs of the same printed
// card face: a Room's CardID carries only which doors are unlocked, so every
// state def carries the same Faces table (and the FaceIdx face's lists).
func (r *realSide) sameRoomFace(a, b dirac.CardID) bool {
	if int(a) >= len(r.tbl.Cards) || int(b) >= len(r.tbl.Cards) {
		return false
	}
	ca, cb := &r.tbl.Cards[a], &r.tbl.Cards[b]
	if ca.FaceMode != kop.FaceRoom || cb.FaceMode != kop.FaceRoom ||
		ca.Face != cb.Face || len(ca.Faces) == 0 || len(cb.Faces) == 0 {
		return false
	}
	return ca.Faces[0] == cb.Faces[0]
}

// lkiAbility is gorge's ability index for a stack entry whose source card
// has changed since the ability was queued. A Room's door state is another
// def of the same printed face (CR 709.5), so gorge's abilityIndex still
// finds the queued SA in the unchanged src.Face(); the LKI def's origin is
// that face's index. Any other change is a face change (a transforming DFC
// token), whose current face no longer has the SA.
func (r *realSide) lkiAbility(srcCard, card dirac.CardID, ab uint8) uint8 {
	if r.sameRoomFace(srcCard, card) {
		return r.cardAbility(card, ab)
	}
	return 255
}

// stackAbility is the projected ability index of a stack entry whose source
// currently shows srcCard while the queued ability's card and index are card
// and ab.
func (r *realSide) stackAbility(srcCard, card dirac.CardID, ab uint8) uint8 {
	if srcCard != card {
		return r.lkiAbility(srcCard, card, ab)
	}
	return r.cardAbility(card, ab)
}

// Project converts the kernel's projection into the oracle's.
func (r *realSide) Project(p *Projection) {
	r.s.Project(&r.kp)
	r.s.ProjectChars(&r.kp)
	kp := &r.kp
	*p = Projection{}
	p.NInst = kp.NObj
	for i := range p.Cards {
		p.Cards[i] = CardProj{Card: NoCardIndex, Zone: ZoneNone, Pos: 255, AttachedTo: RefNone, Blocks: RefNone}
	}
	for i := 0; i < int(kp.NObj) && i < MaxInst; i++ {
		o := &kp.Objs[i]
		c := &p.Cards[i]
		pc := o.Card
		if o.Flags&dirac.ProjBase != 0 {
			pc = o.Base // gorge projects a copy effect's permanent as its printed card
		}
		if int(pc) < len(r.gname) {
			c.NameHash = nameHash(r.gname[pc])
			if o.Flags2&dirac.ProjNameless != 0 {
				r.nameless[i/64] |= 1 << (i % 64)
			}
			if r.nameless[i/64]>>(i%64)&1 != 0 {
				c.NameHash = nameHash("")
			}
			c.Face = r.tbl.Cards[pc].Face
			if !o.Token && o.Flags&dirac.ProjCopy == 0 { // a spell copy is no deck card (gorge project.go)
				c.Card = r.tidx[pc]
			}
		}
		switch {
		case o.Zone == kop.ZoneNone:
			c.Zone = ZoneCeased
		case o.Zone >= kop.ZonePhased:
			c.Zone = uint8(kop.ZoneBattlefield) - 1 // a phased-out permanent is still on the battlefield (CR 702.26d)
		default:
			c.Zone = uint8(o.Zone) - 1
		}
		c.Owner, c.Controller = o.Owner, o.Ctrl
		switch o.Zone {
		case kop.ZoneLibrary, kop.ZoneBattlefield, kop.ZoneGraveyard, kop.ZoneExile, kop.ZonePhased, kop.ZonePhasedInd:
			c.Pos = o.ZoneIdx
		}
		var f uint16
		if o.Flags&dirac.ProjTapped != 0 {
			f |= FTapped
		}
		if o.Flags&dirac.ProjCopy != 0 {
			f |= FCopy
		}
		if o.Flags&dirac.ProjSick != 0 {
			f |= FSummonSick
		}
		if o.Flags&dirac.ProjAttacking != 0 {
			f |= FAttacking
		}
		if o.Flags&dirac.ProjBlocking != 0 {
			f |= FBlocking
		}
		if o.Flags&dirac.ProjMonstrous != 0 {
			f |= FMonstrous
		}
		if o.Flags2&dirac.ProjSuspected != 0 {
			f |= FSuspected
		}
		if o.Zone >= kop.ZonePhased {
			f |= FPhasedOut
		}
		if o.Flags&dirac.ProjFaceDown != 0 {
			f |= FFaceDown
		}
		if o.Token {
			f |= FToken
		}
		c.Flags = f
		c.Damage = int16(o.Damage)
		c.Power, c.Toughness = int16(o.Power), int16(o.Tough)
		for k, n := range o.Counters {
			slot := CtrOther
			switch kop.CounterKind(k) {
			case kop.CtrP1P1:
				slot = CtrP1P1
			case kop.CtrM1M1:
				slot = CtrM1M1
			case kop.CtrLoyalty:
				slot = CtrLoyalty
			case kop.CtrShield:
				slot = CtrShieldC
			}
			c.Counters[slot] += int16(n)
		}
		for x, n := range o.CountersX {
			c.Counters[CtrX+x] += int16(n)
		}
		// gorge's deathtouch mark is a counter of its own (InternalCounterMarker)
		c.Counters[CtrOther] += int16(r.s.FeatDeathtouchMark(dirac.Obj(i)))
		// and so is its regeneration shield ("Shield", the same marker family)
		c.Counters[CtrOther] += int16(r.s.FeatRegenShields(dirac.Obj(i)))
		c.AttachedTo, c.Blocks = Ref(o.Attached), Ref(o.Blocks)
		if o.Zone == kop.ZoneBattlefield || o.Zone >= kop.ZonePhased {
			ch := &kp.Chars[i]
			c.Chars = CharsProj{Types: uint16(ch.Types), Subtypes: ch.Subs, AllTypes: ch.AllSub, Colors: uint8(ch.Colors), Keywords: uint32(ch.Kw &^ kwStaticBits)}
		}
	}
	p.Initiative = r.s.Initiative() // CR 726 (act_initiative.go)
	p.DayNight = r.s.DayNight()     // CR 731 (kw_daynight.go)
	for s := 0; s < 2; s++ {
		p.Room[s] = r.s.DungeonRoom(uint8(s))
		p.Life[s] = int32(kp.Life[s])
		p.Poison[s] = int16(r.s.Poison(uint8(s))) // infect / toxic (kw_infect.go)
		p.Speed[s] = uint8(r.s.Speed(uint8(s)))   // CR 702.179 (kw_speed.go)
		if uint8(s) == kp.Active {
			p.LandsPlayed[s] = kp.Lands[s] // the active player's only (project.go)
		}
		for k := 0; k < 6 && k < len(kp.Pool[s]); k++ {
			p.Pool[s][k] = int16(kp.Pool[s][k])
		}
		p.LibCount[s], p.HandCount[s] = kp.LibN[s], kp.HandN[s]
	}
	p.NStack = kp.NStack
	_, _, building := r.s.BuildingCast()
	for i := 0; i < int(kp.NStack) && i < MaxStack; i++ {
		e := &kp.Stack[i]
		st := &p.Stack[i]
		st.Kind, st.Source, st.Controller = e.Kind, Ref(e.Source), e.Ctrl
		st.Ability = 255
		st.Modes = r.stackModes(e)
		if !building || i < int(kp.NStack)-1 {
			st.X = int16(e.X) // gorge writes Object.X as the cast commits, not as X is announced
		}
		if e.Kind != 0 {
			srcCard := e.Card
			if int(e.Source) < int(kp.NObj) {
				srcCard = kp.Objs[e.Source].Card
			}
			st.Ability = r.stackAbility(srcCard, e.Card, e.Ability)
		}
		for k := range st.Targets {
			st.Targets[k] = RefNone
		}
		for si, t := range e.Targets {
			if t != dirac.NoTarget && int(st.NTargets) < MaxTgts {
				st.Targets[st.NTargets] = t
				if t == dirac.StackTargetGone {
					st.Targets[st.NTargets] = RefNone // a stack ability that left (gorge's vanished object)
				}
				st.NTargets++
			}
			for j := 0; j < int(e.NExtra); j++ {
				if int(e.ExtraSlot[j]) == si && int(st.NTargets) < MaxTgts {
					st.Targets[st.NTargets] = e.Extra[j]
					st.NTargets++
				}
			}
		}
	}
	p.Turn = int32(kp.Turn)
	switch st := kp.Step; {
	case st <= kop.StepBlockers:
		p.Step = uint8(st)
	case st == kop.StepFirstStrikeDamage || st == kop.StepDamage:
		p.Step = 7 // gorge's single combat-damage step
	default:
		p.Step = uint8(st) - 1
	}
	p.Active, p.Priority = kp.Active, kp.Prio
	if kp.Turn == 0 {
		p.Active = 0 // gorge's Game.Active before the first turn (kernel Projection names the starter)
	}
	p.RNGDraws = kp.Draws
	p.Over, p.Winner = kp.Over, kp.Winner
}

// stackModes is e's chosen modes: its mode byte also carries cast provenance
// outside the ability's NModes bits.
func (r *realSide) stackModes(e *dirac.ProjStack) uint8 {
	if int(e.Card) >= len(r.tbl.Cards) || int(e.Ability) >= len(r.tbl.Cards[e.Card].Abilities) {
		return 0
	}
	return e.Mode & uint8(1<<r.tbl.Cards[e.Card].Abilities[e.Ability].NModes-1)
}

// Close releases the kernel's registration of the side's table (a run
// that builds a table per matchup releases each when its game is done).
func (r *realSide) Close() { dirac.ReleaseTable(r.tbl) }

// ChanceTranscript implements ChanceSource for a kernel built with
// Config.RecordChance: every IntN the kernel drew, in order. It is nil when
// recording was off or the kernel's transcript overflowed
// dirac.MaxChanceRuns (a game always draws the toss, so nil is never a real
// empty transcript).
func (r *realSide) ChanceTranscript() []ChanceDraw {
	tr, ok := r.s.ChanceTranscript()
	if !ok {
		return nil
	}
	out := make([]ChanceDraw, len(tr))
	for i, c := range tr {
		out[i] = ChanceDraw{Bound: c.Bound, Value: c.Value}
	}
	return out
}

// CloseSide closes a side that holds kernel resources (a RealKernel side).
func CloseSide(s Side) {
	if c, ok := s.(interface{ Close() }); ok {
		c.Close()
	}
}

var _ Side = (*realSide)(nil)

// SetMatchup builds pool decks a and b for both sides: the gorge matchup
// (Corpus.Matchup) and the kernel table (set.Matchup) the SetSide adapter
// plays.
func SetMatchup(set cardset.Set, c *Corpus, a, b string, mulligans int) (*Matchup, *kop.Table, error) {
	m, err := c.Matchup(a, b, mulligans)
	if err != nil {
		return nil, nil, err
	}
	var es [2][]cardset.Entry
	for s := 0; s < 2; s++ {
		f, err := c.deckFile(m.Names[s])
		if err != nil {
			return nil, nil, err
		}
		for _, e := range f.Cards {
			es[s] = append(es[s], cardset.Entry{Name: e.Name, Count: e.Count})
		}
	}
	tbl, _, _, err := set.Matchup(es[0], es[1])
	if err != nil {
		return nil, nil, err
	}
	return m, tbl, nil
}

// FDNMatchup is SetMatchup over the "fdn" set.
func FDNMatchup(c *Corpus, a, b string, mulligans int) (*Matchup, *kop.Table, error) {
	return SetMatchup(fdnSet(), c, a, b, mulligans)
}

// maxSpeedBase is the first canonical ability code of a max-speed grant
// (CR 702.179e): the n-th activated ability a card's "Max speed" statics
// grant is maxSpeedBase+n. Gorge offers it as a "granted" option naming the
// SVar (GorgeSide), the kernel as the card's Gate{CtrMaxSpeed} ability.
const maxSpeedBase = 0xE0

// maxSpeedCode is the canonical code of the max-speed activated ability abs[idx].
func maxSpeedCode(abs []kop.Ability, idx int) uint8 {
	n := 0
	for i := 0; i < idx; i++ {
		if abs[i].Gate.Ctr == kop.CtrMaxSpeed && abs[i].Kind == kop.AbActivated {
			n++
		}
	}
	return maxSpeedBase + uint8(n)
}

// impendTickAb reports whether ability idx of cd is Impending's synthesized
// end-step tick (kgen lower.go impendingTick: one RemoveCounter of a time
// counter from itself).
func impendTickAb(cd *kop.CardDef, idx uint8) bool {
	if cd.Impending == 0 || int(idx) >= len(cd.Abilities) {
		return false
	}
	a := &cd.Abilities[idx]
	return a.Kind == kop.AbTriggered && len(a.Prog) == 1 && a.Prog[0].Op == kop.OpRemoveCounter && kop.CounterKind(a.Prog[0].C) == kop.CtrTime
}
