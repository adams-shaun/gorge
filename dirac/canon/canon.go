// Package canon is the vocabulary both engines' decisions are compared in, and
// the translation of the kernel's actions into it. An option is one packed tuple
// of kind, source, target and mode, independent of either engine's object ids.
package canon

import (
	"fmt"
)

// Ref is a canonical reference, the source/target byte of a Canon and the
// reference fields of a Projection: 0..MaxInst-1 a card instance (doc.go,
// "Card identity"), RefStack+i the i-th stack object from the bottom (an
// ability, which is no card), RefPlayer+p seat p, RefNone nothing.
type Ref = uint8

const (
	RefStack  Ref = 0xE0
	RefPlayer Ref = 0xF0
	RefNone   Ref = 0xFF
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

func (a Canon) Src() Ref { return Ref(a >> 16) }

func (a Canon) Tgt() Ref { return Ref(a >> 8) }

func (a Canon) Mode() uint8 { return uint8(a) }

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

// CastMode codes, the low nibble of an ActCast mode (the high nibble is the
// option's AltCostIndex). Option.Mode / PlannedCast.Origin spellings.
var castModes = []string{"", "kicked", "flashback", "mayplay", "surged", "miracle", "bestowed", "plot", "warped", "evoked", "dash", "blitz", "foretell", "disturb", "escape", "offspring"}

// CastModeOther is the code of a cast mode the table does not list. The
// table fills the nibble, so it shares "offspring"'s code: CastModeCode
// reports false for an unlisted mode and its callers drop it.
const CastModeOther = 15

func CastModeCode(mode string) (uint8, bool) {
	for i, m := range castModes {
		if m == mode {
			return uint8(i), true
		}
	}
	return CastModeOther, false
}

// CastModeName is the Option.Mode a cast mode code stands for.
func CastModeName(code uint8) string {
	if int(code) < len(castModes) {
		return castModes[code]
	}
	return "other"
}

// ChooseKinds is the KChoose option-kind vocabulary (decision.KChoose's doc
// plus the kinds the FDN pool poses); ActChoose+i is its action kind.
var ChooseKinds = []string{
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
	"revealcost",       // gorge's AdvanceUnless reveal pick
	"ward_mana",        // gorge ward.go's {N} option of a discard-or-pay ward
	"puttolibcost",     // gorge putToLibAsk's PutCardToLibFrom<Zone> pick
	"keep_tapped",      // gorge finishUntapStep's untap-step election (CR 502.3), beside "untap"
	"conspire",         // CR 702.78a: gorge conspireAsk's tap pick (kw_conspire.go)
	"setstate_pick",    // gorge setStateChoose's pick (SetState Choices$; Hustle // Bustle)
	"face_down_choice", // gorge effCloak's cloak-from-hand pick (chooseFromZoneForFaceDown, CR 701.58a)
	"ward_waterbend",   // gorge kw_ward.go's Ward—Waterbend pick (The Unagi of Kyoshi Island)
	"counter_dist",     // gorge's distribute-counters pick among the chosen permanents (Vastwood Hydra)
	"vote_card",        // gorge effCardVote's ballot pick (CR 701.38a)
}

// ChooseOther is the ActChoose offset of an option kind ChooseKinds lacks.
const ChooseOther = 127

func ChooseCode(kind string) (uint8, bool) {
	for i, k := range ChooseKinds {
		if k == kind {
			return uint8(i), true
		}
	}
	return ChooseOther, false
}

// SpecialKinds are the priority special actions (ActSpecial's mode).
var SpecialKinds = []string{"granted", "unlock", "turn_face_up", "specialize", "station", "plot", "foretell", "suspend", "crew", "saddle"}

// TurnUpModeCode is the turn_face_up ActSpecial mode's procedure sub-mode
// (bits 4..7, beside SpecialCode's kind index): 0 the face-down cast's own
// keyword, 1 manifest, 2 cloak, 3 morph, 4 megamorph, 5 disguise
// (CR 701.40b-d, 701.58b-d). Both sides encode the same value.
func TurnUpModeCode(mode string) uint8 {
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

// UnlockModeCode is the unlock ActSpecial mode's door sub-mode (bits 4..7):
// gorge's Option.Mode is the door index ("0" / "1"), coded door+1 so the two
// doors of a both-locked Room stay apart (kernel AkUnlock, mode = the door).
func UnlockModeCode(mode string) uint8 {
	switch mode {
	case "0":
		return 1
	case "1":
		return 2
	}
	return 0
}

func SpecialCode(kind string) (uint8, bool) {
	for i, k := range SpecialKinds {
		if k == kind {
			return uint8(i), true
		}
	}
	return 0, false
}

const ManaSyms = "WUBRGC"

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
		if i < len(ChooseKinds) {
			name = "choose:" + ChooseKinds[i]
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

var decisionNames = []string{"priority", "attackers", "blockers", "target", "modes", "choose", "trigger_order",
	"mulligan", "none", "trigger_optional", "arrange", "replacement", "starting_player", "commander_zone"}

func (k DKind) String() string {
	if int(k) < len(decisionNames) {
		return decisionNames[k]
	}
	return fmt.Sprintf("kind%d", k)
}

// LandFaceCode is a play_land option's canonical mode: the face played, 1
// for a modal DFC's back land ("modal_land"b), 0 otherwise.
func LandFaceCode(mode string) uint8 {
	if mode == "modal_land" {
		return 1
	}
	return 0
}
