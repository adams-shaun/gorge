package policynet

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// Option is one encoded offered option (R1 §1.4's per-option block): its
// categorical one-hot slots, its hashed ids into the shared TableRows table,
// its 24 dense scalars, and (filled by the loader) its label target.
type Option struct {
	// Slots are the categorical one-hot features, Row < OptionSlotWidth,
	// emitted in the fixed block order documented at EncodeOption (so slot
	// order is fixed by construction, never by map or hash iteration).
	Slots []Feature
	// Hashed are the hashed ids into the shared TableRows table, emitted in
	// the fixed order: [0] card identity, [1] card identity × target
	// descriptor, [2] option-kind identity. (R1 §1.4 names the first two;
	// the third is this build's addition — the option-kind one-hot list is
	// R1's measured 23 kinds plus "other", and the hashed kind id keeps an
	// unseen kind distinguishable instead of collapsing into "other".)
	Hashed []Feature
	// Dense is the fixed 24-wide scalar vector documented at EncodeOption.
	Dense []float32
	// Target is the per-option label target: the candidate value when the
	// teacher evaluated an answer containing this option, the
	// teacher-preferred mask, and the unlabelled flag. Zero value =
	// unlabelled, never silently zero-valued.
	Target OptionTarget
}

// OptionTarget is one option's training target from the label record.
type OptionTarget struct {
	// Labelled is true when at least one teacher candidate evaluated an
	// answer containing this option; false = unlabelled.
	Labelled bool
	// Preferred is true when this option is part of the teacher's chosen
	// candidate (candidates[teacher_choice]; when teacher_choice is 0 the
	// bot's own answer was kept, so it is the teacher's choice). Margin is
	// carried on the Example for ranking losses.
	Preferred bool
	// Value is the FIRST candidate (in record order) that evaluated an
	// answer containing this option.
	Value float64
}

// Option slot layout (R1 §1.4's block table, in its order). The widths and
// offsets are a pinned contract (golden test).
const (
	OptionSlotWidth = 128

	optKindOffset   = 0 // 24 slots: optionKinds (23) + other
	numOptKindSlots = 24
	decKindOffset   = 24 // 13: decision.Kinds (12) + other
	numDecKindSlots = 13
	typeOffset      = 37 // 12: cardTypeBits (10) + token + other
	numTypeSlots    = 12
	colourOffset    = 49 // 6: WUBRG + colourless + unused spare
	numColourSlots  = 6
	mvOffset        = 55 // 9: MV 0..7, 8+
	numMvSlots      = 9
	modeOffset      = 64 // 6: modeValues (5) + other
	numModeSlots    = 6
	altCostOffset   = 70 // 1: alt-cost present
	kwOffset        = 71 // 32: keywordBits (31) + other
	numKwSlots      = 32
	zoneOffset      = 103 // 8: mine-vs-theirs × {battlefield, hand, graveyard, exile}
	numZoneSlots    = 8
)

// optionKinds is R1 §1.4's measured option-kind one-hot vocabulary (23,
// measured over all 9 decision kinds, 660 games). The live engine vocabulary
// has since grown to 85 distinct kinds; anything outside this list sets the
// "other" slot AND the hashed "optkind|<kind>" id, so an unseen kind stays
// distinguishable.
var optionKinds = [...]string{
	"ability", "activate", "attacker", "block", "bottom", "cast", "concede",
	"exile", "keep", "mode", "mulligan", "name", "no", "number", "pass",
	"permanent", "play_land", "player", "sacrifice", "trigger", "type", "x",
	"yes",
}

// cardTypeBits is the fixed card-type vocabulary (10 words + a Token bit
// driven by CardView.Token + an "other" bit when no listed word matched).
var cardTypeBits = [...]string{
	"Land", "Creature", "Artifact", "Enchantment", "Instant", "Sorcery",
	"Planeswalker", "Battle", "Legendary", "Basic",
}

// modeValues is Option.Mode's vocabulary (decision.go): "" the card's own
// cost, then the named alternates, plus "other".
var modeValues = [...]string{"", "kicked", "surged", "flashback", "miracle"}

// keywordBits is the fixed 31-keyword block (R1 §1.4: 31 registered kw:
// primitives + "other"); membership is matched case-insensitively against
// the card's projected keyword heads. The exact 31 are this build's
// combat/cast set; any other keyword a card carries sets "other".
var keywordBits = [...]string{
	"Flying", "Reach", "Haste", "Vigilance", "Deathtouch", "Trample",
	"Lifelink", "First Strike", "Double Strike", "Flash", "Indestructible",
	"Devoid", "Defender", "Menace", "Fear", "Shadow", "Ward", "Hexproof",
	"Protection", "Undying", "Persist", "Evolve", "Exalted", "Dethrone",
	"Prowess", "Storm", "Affinity", "Kicker", "Flashback", "Convoke",
	"Cycling",
}

// Zone classes for the zone/ownership bits: (mine?0:4) + zoneIndex.
const (
	zoneBattlefield = 0
	zoneHand        = 1
	zoneGraveyard   = 2
	zoneExile       = 3
)

// Option dense-scalar offsets (24 wide, pinned).
const (
	OptionDenseWidth = 24

	odPower      = 0
	odToughness  = 1
	odRemaining  = 2 // toughness − damage
	odDamage     = 3
	odTapped     = 4
	odSummonSick = 5
	odAttacking  = 6
	odIsPlayer   = 7 // the option names a player (a player-ref Obj or Kind "player")
	odAtkPower   = 8 // block option: the attacker's card, else 0
	odAtkTough   = 9
	odAtkDamage  = 10
	odAtkDelta   = 11 // my toughness − attacker power (does the block survive?)
	odCostTotal  = 12 // mana pips in the option's cost (X counts 0)
	odCostDelta  = 13 // costTotal − (my pool + my available)
	odTapOut     = 14 // costTotal > 0 && costTotal > available
	odAmount     = 15 // Option.Amount (the X value an "x" option represents)
	odAltCost    = 16 // AltCostIndex != 0
	odRequired   = 17 // Option.Required (a must-attack creature)
	odIndex      = 18 // idx/(n−1), 0 when n ≤ 1
	odManaValue  = 19 // the option card's mana value
	odActivated  = 20 // the option card's ActivatedThisTurn
	odCounters   = 21 // total counters on the option card
	odAttached   = 22 // AttachedTo != 0
	odGroup      = 23 // Option.Group != ""
)

// cardRef is one view-resolved card: its CardView, which zone class it sits
// in (zoneIndex, or -1 for command/commanders/stack), and whether the zone
// list is the viewer's own.
type cardRef struct {
	cv   view.CardView
	zone int
	mine bool
}

// cardIndex resolves ObjIDs to cards by scanning the view's zone lists in a
// fixed order (my hand, my battlefield, then each other battlefield in seat
// order, graveyards, exiles, command, commanders, stack) — first match wins,
// and the map is only ever read by key afterwards, so no iteration order
// reaches an output.
func cardIndex(v view.View, seat state.PlayerID) map[state.ObjID]cardRef {
	idx := make(map[state.ObjID]cardRef)
	add := func(cards []view.CardView, zone int, mine bool) {
		for i := range cards {
			cv := cards[i]
			if _, ok := idx[cv.ID]; !ok {
				idx[cv.ID] = cardRef{cv: cv, zone: zone, mine: mine}
			}
		}
	}
	for i := range v.Players {
		pv := &v.Players[i]
		mine := pv.ID == seat
		if mine {
			add(pv.Hand, zoneHand, true)
			add(pv.Battlefield, zoneBattlefield, true)
			add(pv.Graveyard, zoneGraveyard, true)
			add(pv.Exile, zoneExile, true)
		}
	}
	for i := range v.Players {
		pv := &v.Players[i]
		if pv.ID == seat {
			continue
		}
		add(pv.Battlefield, zoneBattlefield, false)
		add(pv.Graveyard, zoneGraveyard, false)
		add(pv.Exile, zoneExile, false)
	}
	for i := range v.Players {
		pv := &v.Players[i]
		if pv.ID == seat {
			add(pv.Command, -1, true)
			add(pv.Commanders, -1, true)
		}
	}
	for i := range v.Stack {
		if sv := &v.Stack[i]; sv.Card != nil {
			if _, ok := idx[sv.Card.ID]; !ok {
				idx[sv.Card.ID] = cardRef{cv: *sv.Card, zone: -1, mine: sv.Card.Controller == seat}
			}
		}
	}
	return idx
}

// EncodeOption encodes one offered option (R1 §1.4). d is the decision kind
// being answered (the record's view carries no decision), idx the option's
// index in the offered list and n the list length.
//
// Slot blocks, in emission order: option-kind one-hot (23 R1 kinds + other),
// decision-kind one-hot (decision.Kinds + other), card-type bits, colour
// bits (WUBRG + colourless from the card's mana cost), mana-value bucket
// (0..7, 8+), Option.Mode one-hot (5 + other), alt-cost-present bit, keyword
// bits (31 + other), zone/ownership bits (mine-vs-theirs ×
// battlefield/hand/graveyard/exile).
//
// Dense scalars are documented at their offset constants above; a scalar
// whose subject the option does not name (no card, not a block option) is 0.
func EncodeOption(v view.View, seat state.PlayerID, d decision.Kind, o decision.Option, idx, n int) Option {
	idxCards := cardIndex(v, seat)

	var slots []Feature
	set := func(row int) { slots = append(slots, Feature{Row: uint16(row), Value: 1}) }
	if i := indexOf(optionKinds[:], o.Kind); i >= 0 {
		set(optKindOffset + i)
	} else {
		set(optKindOffset + len(optionKinds))
	}
	if i := decKindIndex(d); i >= 0 {
		set(decKindOffset + i)
	} else {
		set(decKindOffset + len(decision.Kinds))
	}

	// Card resolution: a player-ref Obj (a player-target option) resolves
	// to the player, not a card.
	isPlayer := false
	var card *cardRef
	if o.Obj != 0 {
		if p, ok := o.Obj.PlayerRef(); ok {
			isPlayer = true
			_ = p
		} else if ref, ok := idxCards[o.Obj]; ok {
			card = &ref
		}
	}
	if o.Kind == "player" {
		isPlayer = true
	}

	// Card-type bits.
	types := ""
	token := false
	if card != nil {
		types = card.cv.Types
		token = card.cv.Token != ""
	}
	any := false
	for i, w := range cardTypeBits {
		if hasTypeWord(types, w) {
			set(typeOffset + i)
			any = true
		}
	}
	if token {
		set(typeOffset + 10)
		any = true
	}
	if !any {
		set(typeOffset + 11) // other
	}

	// Colour bits off the mana cost's symbols; mana-value bucket.
	cost := ""
	if card != nil {
		cost = card.cv.ManaCost
	}
	colours, mv := manaCostBits(cost)
	for c := 0; c < 5; c++ {
		if colours[c] {
			set(colourOffset + c)
		}
	}
	if colours[5] { // colourless {C}
		set(colourOffset + 5)
	}
	if mv > 8 {
		mv = 8
	}
	set(mvOffset + mv)

	// Mode one-hot.
	if i := indexOf(modeValues[:], o.Mode); i >= 0 {
		set(modeOffset + i)
	} else {
		set(modeOffset + len(modeValues))
	}
	if o.AltCostIndex != 0 {
		set(altCostOffset)
	}

	// Keyword bits.
	kwOther := false
	if card != nil {
		for _, k := range card.cv.Keywords {
			hit := false
			for i, w := range keywordBits {
				if strings.EqualFold(k, w) {
					set(kwOffset + i)
					hit = true
					break
				}
			}
			if !hit {
				kwOther = true
			}
		}
	}
	if kwOther {
		set(kwOffset + 31)
	}

	// Zone/ownership relation.
	if card != nil && card.zone >= 0 {
		set(zoneOffset + boolInt(card.mine)*4 + card.zone)
	}

	// Dense scalars.
	dense := make([]float32, OptionDenseWidth)
	if card != nil {
		dense[odPower] = float32(card.cv.Power)
		dense[odToughness] = float32(card.cv.Toughness)
		dense[odRemaining] = float32(card.cv.Toughness - card.cv.Damage)
		dense[odDamage] = float32(card.cv.Damage)
		dense[odTapped] = float32(boolInt(card.cv.Tapped))
		dense[odSummonSick] = float32(boolInt(card.cv.SummonSick))
		dense[odAttacking] = float32(boolInt(card.cv.Attacking))
		dense[odManaValue] = float32(mvOf(cost))
		dense[odActivated] = float32(card.cv.ActivatedThisTurn)
		for _, v := range card.cv.Counters {
			dense[odCounters] += float32(v)
		}
		dense[odAttached] = float32(boolInt(card.cv.AttachedTo != 0))
	}
	dense[odIsPlayer] = float32(boolInt(isPlayer))
	if o.Attacker != 0 {
		if aref, ok := idxCards[o.Attacker]; ok {
			dense[odAtkPower] = float32(aref.cv.Power)
			dense[odAtkTough] = float32(aref.cv.Toughness)
			dense[odAtkDamage] = float32(aref.cv.Damage)
			if card != nil {
				dense[odAtkDelta] = float32(card.cv.Toughness - aref.cv.Power)
			}
		}
	}
	costTotal := float32(mvOf(cost))
	if o.Kind == "activate" && cost == "" {
		costTotal = float32(mvOf(o.Cost))
	}
	if costTotal > 0 {
		avail := availableTotal(v, seat)
		dense[odCostTotal] = costTotal
		dense[odCostDelta] = costTotal - avail
		if costTotal > avail {
			dense[odTapOut] = 1
		}
	}
	dense[odAmount] = float32(o.Amount)
	dense[odAltCost] = float32(boolInt(o.AltCostIndex != 0))
	dense[odRequired] = float32(boolInt(o.Required))
	if n > 1 {
		dense[odIndex] = float32(idx) / float32(n-1)
	}
	dense[odGroup] = float32(boolInt(o.Group != ""))

	// Hashed ids, in the fixed order documented at Option.Hashed.
	var hashed []Feature
	cardID := ""
	if isPlayer {
		cardID = "card|player"
	} else if card != nil {
		cardID = "card|" + card.cv.Name
	}
	if cardID != "" {
		desc := "none"
		if o.Attacker != 0 {
			if aref, ok := idxCards[o.Attacker]; ok {
				desc = "atk|" + aref.cv.Name
			} else {
				desc = "atk|unknown"
			}
		}
		hashed = append(hashed,
			Feature{Row: hashID(cardID), Value: 1},
			Feature{Row: hashID(cardID + "\x1f" + desc), Value: 1},
		)
	}
	hashed = append(hashed, Feature{Row: hashID("optkind|" + o.Kind), Value: 1})

	return Option{Slots: slots, Hashed: hashed, Dense: dense}
}

// decKindIndex maps a decision.Kind to its one-hot index (decision.Kinds
// order — the static universe, so encoder and engine cannot drift).
func decKindIndex(d decision.Kind) int {
	for i, k := range decision.Kinds {
		if k == d {
			return i
		}
	}
	return -1
}

// hasTypeWord reports whether the space-separated Types string carries the
// word (case-insensitive).
func hasTypeWord(types, word string) bool {
	for _, t := range strings.Fields(types) {
		if strings.EqualFold(t, word) {
			return true
		}
	}
	return false
}

// manaCostBits parses a Forge-notation mana cost ("1 W", "X G", "R G",
// "G/W") into five colour booleans (WUBRG; index 5 = colourless {C}) and a
// mana value: a digit token adds its value, an X/Y/Z token adds 0, any other
// symbolic token adds one per '/'-separated pip. Colourless {C} is not a
// colour bit for the five WUBRG slots; it gets its own.
func manaCostBits(cost string) ([6]bool, int) {
	var colours [6]bool
	mv := 0
	for _, tok := range strings.Fields(cost) {
		if n, err := strconv.Atoi(tok); err == nil {
			mv += n
			continue
		}
		switch tok {
		case "X", "Y", "Z":
			// unknown or variable — contributes 0
		default:
			parts := strings.Split(tok, "/")
			for _, p := range parts {
				switch p {
				case "W":
					colours[0] = true
				case "U":
					colours[1] = true
				case "B":
					colours[2] = true
				case "R":
					colours[3] = true
				case "G":
					colours[4] = true
				case "C":
					colours[5] = true
				}
				mv++
			}
		}
	}
	return colours, mv
}

// mvOf is manaCostBits' mana-value half alone.
func mvOf(cost string) int {
	_, mv := manaCostBits(cost)
	return mv
}

// availableTotal sums the viewer seat's floating pool plus its available
// (tappable) mana, in the fixed WUBRG C key order.
func availableTotal(v view.View, seat state.PlayerID) float32 {
	for i := range v.Players {
		pv := &v.Players[i]
		if pv.ID != seat {
			continue
		}
		var total int64
		for _, key := range poolKeys {
			total += int64(pv.Pool[key]) + int64(pv.Available[key])
		}
		return float32(total)
	}
	return 0
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
