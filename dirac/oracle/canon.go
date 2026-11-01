package oracle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Canon is a canonical option tuple (doc.go's grammar), packed
// kind<<24 | source<<16 | target<<8 | mode. Source and target are Refs.
type Canon uint32

// CanonKind is a Canon's top byte.
type CanonKind uint8

// The canonical action kinds (doc.go).
const (
	ActPass          CanonKind = iota // priority: pass
	ActPlayLand                       // src = land, mode = the face played (1: a modal DFC back land)
	ActCast                           // src = card, mode = cast mode | alt<<4
	ActActivate                       // src = permanent, mode = ability index (Face.Abilities)
	ActAttack                         // src = attacker, tgt = defender (player ref, or battle/planeswalker)
	ActBlock                          // src = blocker, tgt = attacker
	ActDone                           // finish a multi-pick decision
	ActTarget                         // tgt = the target
	ActMode                           // mode = option index (Choices$ order; unless-pay asks too)
	ActKeep                           // mulligan: keep
	ActMulligan                       // mulligan: take a mulligan
	ActBottom                         // mulligan bottoming: src = card
	ActOrder                          // trigger order: src = trigger source, mode = occurrence
	ActYes                            // trigger_optional / replacement apply: src = subject
	ActNo                             //
	ActArrange                        // arrange pile-A pick: src = card
	ActReplacement                    // replacement order: src = source, mode = occurrence
	ActSpecial                        // priority special action: src = obj, mode = special kind
	ActStartPlayer                    // starting player: tgt = player
	ActCommanderZone                  // commander zone: mode = option index
	ActCastFace                       // src = card, mode = face<<4 | face route code (faces.go: modal_spell, adventure_alt, adventure_recast, split_alt, fuse, aftermath)
	ActCastCasualty                   // src = card, mode = alt<<4: the casualty cast (CR 702.153a; gorge's cast Mode "casualty")
	ActCastImpended                   // src = card, mode = alt<<4: the impending cast (CR 702.176a; gorge's cast Mode "impended"; castModes is full)
	ActCastAirbend                    // src = card: an airbent card cast from exile for {2} (CR 701.65a; gorge's cast Mode "airbend_cast"; castModes and the face routes are full)
	ActChoose        CanonKind = 32   // + choose kind: a KChoose option
)

// Pack builds a Canon.
func Pack(k CanonKind, src, tgt Ref, mode uint8) Canon {
	return Canon(uint32(k)<<24 | uint32(src)<<16 | uint32(tgt)<<8 | uint32(mode))
}

// Kind, Src, Tgt and Mode unpack a Canon.
func (a Canon) Kind() CanonKind { return CanonKind(a >> 24) }
func (a Canon) Src() Ref        { return Ref(a >> 16) }
func (a Canon) Tgt() Ref        { return Ref(a >> 8) }
func (a Canon) Mode() uint8     { return uint8(a) }

// DKind is a canonical decision's kind: gorge's decision.Kind.
type DKind uint8

const (
	DPriority DKind = iota
	DAttackers
	DBlockers
	DTarget
	DModes
	DChoose
	DTriggerOrder
	DMulligan
	DNone // terminal
	DTriggerOptional
	DArrange
	DReplacement
	DStartingPlayer
	DCommanderZone
)

// CDecision is one canonical decision: its options, sorted ascending.
type CDecision struct {
	Kind    DKind
	Player  uint8
	Options []Canon
}

// Side is one side of a lockstep run in the canonical vocabulary: gorge
// (GorgeSide) or a kernel adapted by KernelSide. Either side can lead
// (LockstepLed): the leader poses the decision the policy answers, the
// follower mirrors it.
type Side interface {
	Step() CDecision
	Apply(c Canon)
	Terminal() (over bool, winner int8)
	// Decided is the game result once a player has lost, before Terminal
	// reports the game over too (gorge can still pose a decision after the
	// loss; a kernel ends the game at once).
	Decided() (over bool, winner int8)
	Project(p *Projection)
	// Dirty reports whether the side was answered since the last Project.
	Dirty() bool
	// Turn is the current turn number (the leader's turn caps the game).
	Turn() int
	// Err is the side's first internal failure, nil when it has none.
	Err() error
}

// SideFactory builds a fresh side for a matchup and seed.
type SideFactory func(m *Matchup, seed uint64) (Side, error)

func sortCanon(cs []Canon) { sort.Slice(cs, func(i, j int) bool { return cs[i] < cs[j] }) }

// CastMode codes, the low nibble of an ActCast mode (the high nibble is the
// option's AltCostIndex). Option.Mode / PlannedCast.Origin spellings.
var castModes = []string{"", "kicked", "flashback", "mayplay", "surged", "miracle", "bestowed", "plot", "warped", "evoked", "dash", "blitz", "foretell", "disturb", "escape", "offspring"}

// castModeOther is the code of a cast mode the table does not list. The
// table fills the nibble, so it shares "offspring"'s code: castModeCode
// reports false for an unlisted mode and its callers drop it.
const castModeOther = 15

func castModeCode(mode string) (uint8, bool) {
	for i, m := range castModes {
		if m == mode {
			return uint8(i), true
		}
	}
	return castModeOther, false
}

// CastModeName is the Option.Mode a cast mode code stands for.
func CastModeName(code uint8) string {
	if int(code) < len(castModes) {
		return castModes[code]
	}
	return "other"
}

// chooseKinds is the KChoose option-kind vocabulary (decision.KChoose's doc
// plus the kinds the FDN pool poses); ActChoose+i is its action kind.
var chooseKinds = []string{
	"x", "exile", "sacrifice", "discard", "search", "dig", "name", "type", "number",
	"yes", "no", "keep", "dungeon", "room", "mana", "color", "altaddcost", "division",
	"hidden_pick", "bottom", "primary", "destination", "opening_yes", "opening_no",
	"trigger_cost_pay", "trigger_cost_decline", "done", "hand_move", "imprint", "player",
	"card", "counter", "tap", "untap", "reveal", "mode", "pile", "creature_type", "land_type",
	"cost", "order", "return", "graveyard", "library", "hand", "top", "convoke", "delve",
	"twopiles", "pile-a", "pile-b", "pay", "enchant", "tapcost", "subcounter", "divide",
	"gift_decline", "gift_promise", "forage_exile", "forage_food", "exilecost", "returncost", "clone",
	"counter_pick", "proliferate", "station", "waterbend_generic", "casualty",
	"defeat_cast_yes", "defeat_cast_no", "exile_cost", "unless", "ordinary",
	"ward_evidence",                    // gorge kw_ward.go's collect-evidence pick
	"ward_discard",                     // gorge kw_ward.go's discard pick
	"hideaway",                         // gorge effHideaway's hideaway_pick over the top cards
	"endure_counters", "endure_spirit", // gorge act_endure.go's two modes
	"trigger_cost_blight", "blightcost", "blight", // CR 701.68
	"trigger_cost_counter", // RemoveAnyCounter trigger cost kind ask
	"bargain",              // CR 702.166: gorge optionalSacrificeAsk's Option.Kind "bargain"
	"evidence",             // gorge evidenceAsk / evidenceStage's collect-evidence pick (CR 701.59a)
	"teamwork",             // CR 702.194a: gorge's teamwork tap pick (kw_teamwork.go)
	"ward_sac",             // gorge kw_ward.go's sacrifice pick
	"zone",                 // gorge originAlternativeZones' pick of the zones a search looks in (the option index is canonical)
	"manifest_dread",       // gorge's manifest dread pick of the card to manifest
	"permanent",            // gorge's clone_choice pick
	"beholdcost",           // gorge's beholdCostAsk pick
	"unlock", "lock",       // gorge effUnlockDoor's election
}

// chooseOther is the ActChoose offset of an option kind chooseKinds lacks.
const chooseOther = 127

func chooseCode(kind string) (uint8, bool) {
	for i, k := range chooseKinds {
		if k == kind {
			return uint8(i), true
		}
	}
	return chooseOther, false
}

// specialKinds are the priority special actions (ActSpecial's mode).
var specialKinds = []string{"granted", "unlock", "turn_face_up", "specialize", "station", "plot", "foretell", "suspend", "crew", "saddle"}

// turnUpModeCode is the turn_face_up ActSpecial mode's procedure sub-mode
// (bits 4..7, beside specialCode's kind index): 0 the face-down cast's own
// keyword, 1 manifest, 2 cloak, 3 morph, 4 megamorph, 5 disguise
// (CR 701.40b-d, 701.58b-d). Both sides encode the same value.
func turnUpModeCode(mode string) uint8 {
	switch mode {
	case "manifest":
		return 1
	case "cloak":
		return 2
	case "morph":
		return 3
	case "megamorph":
		return 4
	case "disguise":
		return 5
	}
	return 0
}

// unlockModeCode is the unlock ActSpecial mode's door sub-mode (bits 4..7):
// gorge's Option.Mode is the door index ("0" / "1"), coded door+1 so the two
// doors of a both-locked Room stay apart (kernel AkUnlock, mode = the door).
func unlockModeCode(mode string) uint8 {
	switch mode {
	case "0":
		return 1
	case "1":
		return 2
	}
	return 0
}

func specialCode(kind string) (uint8, bool) {
	for i, k := range specialKinds {
		if k == kind {
			return uint8(i), true
		}
	}
	return 0, false
}

const manaSyms = "WUBRGC"

// labelValue is a label-valued choice's 16-bit canonical value (optionAction).
func labelValue(label string) uint16 {
	if i, ok := creatureTypeIndex[label]; ok {
		return uint16(i)
	}
	return 0x8000 | uint16(nameHash(label)&0x7FFF)
}

var creatureTypeIndex = func() map[string]int {
	m := map[string]int{}
	for i, w := range effects.CreatureTypeWordList() {
		m[w] = i
	}
	return m
}()

// optionAction is the canonical action of option o of gorge decision d
// (doc.go's grammar), for every kind but priority (priorityVocabulary). ok
// false: the option has no canonical form (Stats.Unknown names it).
func (g *GorgeSide) optionAction(d *decision.Decision, o *decision.Option) (Canon, bool) {
	ref := func(id state.ObjID) Ref { return g.id.ref(g.E.G, id) }
	pl := func(p state.PlayerID) Ref { return RefPlayer + Ref(p) }
	switch d.Kind {
	case decision.KTarget:
		if o.Obj == 0 {
			return Pack(ActTarget, RefNone, pl(o.Player), 0), true
		}
		return Pack(ActTarget, RefNone, ref(o.Obj), 0), true
	case decision.KAttackers:
		if o.Battle != 0 {
			return Pack(ActAttack, ref(o.Obj), ref(o.Battle), 0), true
		}
		return Pack(ActAttack, ref(o.Obj), pl(o.Player), 0), true
	case decision.KBlockers:
		return Pack(ActBlock, ref(o.Obj), ref(o.Attacker), 0), true
	case decision.KModes:
		if o.Kind == "discard" {
			// An effect discard (effects.discardAsk): a card pick.
			code, _ := chooseCode("discard")
			return Pack(ActChoose+CanonKind(code), ref(o.Obj), RefNone, 0), true
		}
		return Pack(ActMode, RefNone, RefNone, uint8(o.Index)), o.Index < 256
	case decision.KMulligan:
		switch o.Kind {
		case "keep":
			return Pack(ActKeep, RefNone, RefNone, 0), true
		case "mulligan":
			return Pack(ActMulligan, RefNone, RefNone, 0), true
		case "bottom":
			return Pack(ActBottom, ref(o.Obj), RefNone, 0), true
		}
	case decision.KTriggerOrder:
		return Pack(ActOrder, ref(o.Obj), RefNone, 0), true
	case decision.KTriggerOptional:
		switch o.Kind {
		case "yes":
			return Pack(ActYes, ref(o.Obj), RefNone, 0), true
		case "no":
			return Pack(ActNo, ref(o.Obj), RefNone, 0), true
		}
	case decision.KArrange:
		return Pack(ActArrange, ref(o.Obj), RefNone, 0), true
	case decision.KReplacement:
		switch o.Kind {
		case "replacement":
			// the occurrence among the same source's options: a recipient's
			// granted prevention and its shield counter share one source
			occ := 0
			for i := range d.Options {
				op := &d.Options[i]
				if op == o {
					break
				}
				if op.Kind == "replacement" && op.Obj == o.Obj {
					occ++
				}
			}
			return Pack(ActReplacement, ref(o.Obj), RefNone, uint8(occ)), true
		case "apply":
			return Pack(ActYes, ref(o.Obj), RefNone, 0), true
		case "decline":
			return Pack(ActNo, ref(o.Obj), RefNone, 0), true
		case "mana":
			if c := strings.Index(manaSyms, o.ManaSymbol); len(o.ManaSymbol) == 1 && c >= 0 {
				code, _ := chooseCode("mana")
				return Pack(ActChoose+CanonKind(code), RefNone, RefNone, uint8(c)), true
			}
		}
	case decision.KStartingPlayer:
		return Pack(ActStartPlayer, RefNone, pl(o.Player), 0), true
	case decision.KCommanderZone:
		return Pack(ActCommanderZone, ref(o.Obj), RefNone, uint8(o.Index)), true
	case decision.KChoose:
		if strings.HasPrefix(o.Kind, "pay_") {
			// A pip announcement (rules.pipAnnounceAsk): pay_W..pay_C,
			// pay_generic, pay_life -> mode 0..5, 6, 7.
			code, _ := chooseCode("pay")
			mode := strings.Index(manaSyms, strings.TrimPrefix(o.Kind, "pay_"))
			switch o.Kind {
			case "pay_generic":
				mode = 6
			case "pay_life":
				mode = 7
			}
			if mode < 0 || len(o.Kind) != len("pay_")+1 && mode < 6 {
				return 0, false
			}
			return Pack(ActChoose+CanonKind(code), RefNone, RefNone, uint8(mode)), true
		}
		if o.Kind == "trigger_cost_x" {
			// gorge's Announce$ NumTimes repetition count (cumulative.go
			// triggeredCostNumTimesAsk): the kernel's ChX pick,
			// mode = the count
			code, _ := chooseCode("x")
			return Pack(ActChoose+CanonKind(code), RefNone, RefNone, uint8(o.Amount)), o.Amount >= 0 && o.Amount < 256
		}
		if o.Kind == "trigger_cost_tap" {
			// a printed trigger's own literal tapXType election
			// (gorge cumulative.go:1386); the kernel's ChGenPayTap pick.
			code, _ := chooseCode("tapcost")
			return Pack(ActChoose+CanonKind(code), ref(o.Obj), RefNone, 0), true
		}
		if strings.HasPrefix(o.Kind, "convoke_") {
			// gorge's convoke announcement (rules/pay.ConvokeAsk):
			// convoke_W..convoke_G, convoke_generic -> mode 0..4, 5.
			code, _ := chooseCode("convoke")
			mode := 5
			if o.Kind != "convoke_generic" {
				if len(o.Kind) != len("convoke_")+1 {
					return 0, false
				}
				mode = strings.Index(manaSyms, o.Kind[len("convoke_"):])
				if mode < 0 {
					return 0, false
				}
			}
			return Pack(ActChoose+CanonKind(code), ref(o.Obj), RefNone, uint8(mode)), true
		}
		if o.Kind == "harmonize" {
			// gorge's harmonize announcement (CR 702.180a; pay.ConvokeAsk
			// Kind "harmonize"): the kernel's mode-7 pick, one creature
			// (kw_harmonize.go).
			code, _ := chooseCode("convoke")
			return Pack(ActChoose+CanonKind(code), ref(o.Obj), RefNone, 7), true
		}
		if o.Kind == "improvise_generic" {
			// gorge's improvise announcement (CR 702.126a): mode 6, the
			// kernel's generic-only pick (kw_convoke.go).
			code, _ := chooseCode("convoke")
			return Pack(ActChoose+CanonKind(code), ref(o.Obj), RefNone, 6), true
		}
		if d.ResumeKind == "counter_kind" {
			// gorge removeCounterAny's per-counter kind pick
			// (Glissa Sunslayer): the kind, by its kernel counter kind (the
			// option order is the carrier's slice order, a presentation
			// detail); Done is the declaration's ActDone
			if o.Kind == "remove_counter_done" {
				return Pack(ActDone, RefNone, RefNone, 0), true
			}
			c, ok := kernelCounterKind(o.Label)
			if o.Kind != "counter" || !ok {
				return 0, false
			}
			code, _ := chooseCode("counter")
			return Pack(ActChoose+CanonKind(code), RefNone, RefNone, uint8(c)), true
		}
		code, known := chooseCode(o.Kind)
		if !known {
			// An option kind the table lacks: still canonical as
			// choose:other over its object (or its index when it names
			// none); the caller counts it under Stats.Unknown.
			g.Stats.note(&g.Stats.Unknown, "choose/"+o.Kind+" (as choose:other)")
			if o.Obj != 0 {
				return Pack(ActChoose+chooseOther, ref(o.Obj), RefNone, 0), true
			}
			return Pack(ActChoose+chooseOther, RefNone, RefNone, uint8(o.Index)), o.Index < 256
		}
		k := ActChoose + CanonKind(code)
		src, tgt := ref(o.Obj), RefNone
		if o.Obj == 0 && (o.Kind == "player" || o.Kind == "primary" || o.Kind == "destination" || o.Kind == "divide" || o.Kind == "proliferate") {
			tgt = pl(o.Player)
		}
		var mode int
		switch o.Kind {
		case "type", "creature_type", "land_type", "name":
			// A label-valued choice: the option order is state-dependent
			// (rules.creatureTypeOptions puts the chooser's own types first),
			// so the value is the label's index in gorge's creature-type
			// vocabulary (effects.CreatureTypeWordList), else 0x8000 | its
			// 15-bit FNV-1a, in (target, mode).
			h := labelValue(o.Label)
			return Pack(k, src, Ref(h>>8), uint8(h)), true
		case "x", "number":
			mode = o.Amount
		case "mana", "color":
			if manaChoiceResolving(d) {
				src = RefNone // the source names the ask, not the pick
			}
			if c := strings.Index(manaSyms, o.ManaSymbol); len(o.ManaSymbol) == 1 && c >= 0 {
				mode = c
			} else {
				mode = o.Index
			}
		case "subcounter":
			// a RemoveAnyCounter ask names the kind on each
			// option (the wildcard form); a fixed-kind ask names only the
			// permanent. The kernel carries the kind in the option mode.
			if o.Counter != "" {
				mode = counterKindMode(o.Counter)
			}
		case "trigger_cost_counter", "trigger_cost_among":
			mode = counterKindMode(o.Counter)
		case "division", "altaddcost", "dungeon", "room", "pile", "mode", "counter", "pile-a", "pile-b", "zone":
			// The option index is canonical here: a division is the
			// lexicographic composition order over the attacker's blockers
			// (rules.divisionOptions), an additional-cost pick is the
			// card's cost order, a name/type pick is the sorted universe.
			mode = o.Index
		}
		if mode < 0 || mode > 255 {
			return 0, false
		}
		return Pack(k, src, tgt, uint8(mode)), true
	}
	return 0, false
}

// counterKindMode maps gorge's counter name to the kernel CounterKind index
// (P1P1=0, M1M1=1, ...; kop.CtrXNames for the unfeaturized kinds).
func counterKindMode(name string) int {
	switch name {
	case "P1P1":
		return int(kop.CtrP1P1)
	case "M1M1":
		return int(kop.CtrM1M1)
	case "LOYALTY", "Loyalty":
		return int(kop.CtrLoyalty)
	case "STUN", "Stun":
		return int(kop.CtrStun)
	case "SHIELD", "Shield":
		return int(kop.CtrShield)
	}
	for i, n := range kop.CtrXNames {
		if n == name {
			return int(kop.CtrCharge) + i
		}
	}
	return 0
}

// disambiguate gives repeated actions within one decision distinct modes:
// the k-th repeat (engine order) of an action whose mode byte is 0 gets mode
// k (occ: the canonical form then depends on gorge's option order, not on
// the game state alone -- doc.go, "Not 1:1"). An action still duplicated
// after that is dropped from the list (dup).
func disambiguate(acts []Canon, valid []bool) (occ, dup bool) {
	orig := append([]Canon(nil), acts...)
	for i := range acts {
		if !valid[i] {
			continue
		}
		n := 0
		for j := 0; j < i; j++ {
			if valid[j] && orig[j] == orig[i] {
				n++
			}
		}
		if n > 0 && orig[i].Mode() == 0 && n < 256 {
			acts[i] = orig[i]&^0xFF | Canon(n)
			occ = true
		}
	}
	for i := range acts {
		for j := 0; j < i; j++ {
			if valid[i] && valid[j] && acts[i] == acts[j] {
				valid[i] = false
				dup = true
			}
		}
	}
	return occ, dup
}

// String renders an action for reports.
func (a Canon) String() string {
	k := a.Kind()
	r := func(x Ref) string {
		switch {
		case x == RefNone:
			return "-"
		case x >= RefPlayer:
			return fmt.Sprintf("P%d", x-RefPlayer)
		case x >= RefStack:
			return fmt.Sprintf("S%d", x-RefStack)
		}
		return fmt.Sprintf("#%d", x)
	}
	name := ""
	switch {
	case k >= ActChoose:
		i := int(k - ActChoose)
		if i < len(chooseKinds) {
			name = "choose:" + chooseKinds[i]
		} else {
			name = "choose:other"
		}
	case int(k) < len(actionNames):
		name = actionNames[k]
	default:
		name = fmt.Sprintf("kind%d", k)
	}
	return fmt.Sprintf("%s(%s,%s,%d)", name, r(a.Src()), r(a.Tgt()), a.Mode())
}

var actionNames = []string{"pass", "land", "cast", "activate", "attack", "block", "done", "target", "mode",
	"keep", "mulligan", "bottom", "order", "yes", "no", "arrange", "replacement", "special", "startplayer", "commanderzone", "castface", "castcasualty", "castimpended"}

var decisionKinds = map[decision.Kind]DKind{
	decision.KPriority: DPriority, decision.KTarget: DTarget, decision.KAttackers: DAttackers,
	decision.KBlockers: DBlockers, decision.KMulligan: DMulligan, decision.KModes: DModes,
	decision.KTriggerOrder: DTriggerOrder, decision.KTriggerOptional: DTriggerOptional,
	decision.KCommanderZone: DCommanderZone, decision.KChoose: DChoose,
	decision.KReplacement: DReplacement, decision.KArrange: DArrange,
	decision.KStartingPlayer: DStartingPlayer,
}

var decisionNames = []string{"priority", "attackers", "blockers", "target", "modes", "choose", "trigger_order",
	"mulligan", "none", "trigger_optional", "arrange", "replacement", "starting_player", "commander_zone"}

func (k DKind) String() string {
	if int(k) < len(decisionNames) {
		return decisionNames[k]
	}
	return fmt.Sprintf("kind%d", k)
}

// landFaceCode is a play_land option's canonical mode: the face played, 1
// for a modal DFC's back land ("modal_land"b), 0 otherwise.
func landFaceCode(mode string) uint8 {
	if mode == "modal_land" {
		return 1
	}
	return 0
}

// kernelCounterKind is the kernel counter kind of gorge's counter name.
func kernelCounterKind(name string) (kop.CounterKind, bool) {
	for c, n := range sbtCounterNames {
		if n != "" && n == name && kop.CounterKind(c) != kop.CtrOther {
			return kop.CounterKind(c), true
		}
	}
	for x, n := range kop.CtrXNames {
		if n == name {
			return kop.CtrCharge + kop.CounterKind(x), true
		}
	}
	return 0, false
}
