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
	// Extra is an EXPERIMENTAL per-option dense augmentation appended after
	// Dense in the hidden-layer input. It is NOT produced by EncodeOption and
	// NOT part of the pinned encoder geometry (EncoderHash and the golden
	// encodings are unchanged): it exists so a feature-family experiment can
	// extend an option's input without moving the shipped encoder. A model
	// trained with a non-zero ExtraW cannot be checkpointed (WriteCheckpoint
	// has no field for the width) and must not be: it is a measurement
	// vehicle, not a deployable scorer.
	Extra []float32
	// EntA and EntB are the option's ENTITY references (FeaturesEntity only,
	// ticket pn14): 1 + the index into State.Cards of the option's own card
	// (Obj: the attacker, the blocker, the spell cast, the target) and of its
	// related card (the blocked attacker, the attacked planeswalker or
	// battle); 0 = none. The zero value keeps every non-entity option inert.
	EntA, EntB int32
	// Target is the per-option label target: the candidate value when the
	// teacher evaluated an answer containing this option, the
	// teacher-preferred mask, and the unlabelled flag. Zero value =
	// unlabelled, never silently zero-valued.
	Target OptionTarget
	// BotPick is true when this option is part of the BOT's own answer for
	// the decision: at TRAINING time the loader marks the label record's bot
	// candidate's Choices (candidate BotIndex — the writer emits the bot's
	// answer first, Example.BotIndex == 0 by construction); at INFERENCE the
	// seat's PolicyNetBot marks the wrapped default bot's own answer for the
	// same decision (seat.markBotPicks), so a model trained with a positive
	// residual prior scores under the contract it trained under. It is the
	// training/inference-side "bot score" the residual head adds at a fixed
	// weight (Model.ResidualW): with a large prior the model starts at the
	// bot baseline, so reproducing the bot costs nothing and capacity goes
	// to the overrides. It is NOT part of the encoded feature geometry — it
	// is not in Slots/Dense — so it never moves EncoderHash; the checkpoint
	// schema version is what carries the residual weight (a v1 checkpoint is
	// refused, not silently loaded; until a checkpoint carries a non-zero
	// weight the prior is a train/eval device and the deployed fallback is
	// the delegation path). Set by the loader and the seat's Decide, never by
	// EncodeOption.
	BotPick bool
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
	odCostTotal  = 12 // mana pips in the option's cost; X counts 0 and a
	// non-mana component (Sac<...>, PayLife<...>, T, ...) contributes 0,
	// never a phantom pip
	odCostDelta = 13 // costTotal − (my pool + my available)
	odTapOut    = 14 // costTotal > 0 && costTotal > available
	odAmount    = 15 // Option.Amount (the X value an "x" option represents)
	odAltCost   = 16 // AltCostIndex != 0
	odRequired  = 17 // Option.Required (a must-attack creature)
	odIndex     = 18 // idx/(n−1), 0 when n ≤ 1
	odManaValue = 19 // the option card's mana value
	odActivated = 20 // the option card's ActivatedThisTurn
	odCounters  = 21 // total counters on the option card
	odAttached  = 22 // AttachedTo != 0
	odGroup     = 23 // Option.Group != ""
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
	// Cost pricing. An "activate" option's cost is the ACTIVATION cost the
	// engine put on the wire in Option.Cost (bare taps omit it), never the
	// source card's printed mana cost -- Llanowar Elves' tap ability costs
	// nothing even though the card costs {G}. Only when Option.Cost is empty
	// (every other option kind, and a bare-tap activate) does the card's own
	// printed cost apply. This mirrors decision.Option.Cost's contract
	// ("the activation cost of a priority-window activate option").
	//
	// Option.Cost is a formatCost rendering, NOT a bare mana cost: it mixes
	// mana symbols with non-mana components (T, Sac<...>, PayLife<...>,
	// Exile<...>, SubCounter<...>, PayEnergy<...>, and so on). Pricing the
	// whole string with manaCostBits counted every such token as one generic
	// pip, so e.g. "T Sac<1/CARDNAME>" priced 2 instead of 0. manaOnlyPips
	// reads the mana tokens out of the marker and ignores every non-mana
	// component, so the cost triple reflects only mana the payment actually
	// charges.
	costTotal := float32(mvOf(cost))
	if o.Kind == "activate" {
		costTotal = float32(manaOnlyPips(o.Cost))
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

// manaBraceForm normalises a brace-form mana cost ("{2}{U}{U}") to the
// space-separated Forge notation this parser reads. It mirrors
// cards.manaBraceForm exactly (cards/face.go), so a view that ever projects
// brace-form costs is priced identically to the engine.
var manaBraceForm = strings.NewReplacer("{", " ", "}", " ")

// manaCostBits parses a Forge-notation mana cost ("1 W", "X G", "G/W",
// "2/W", "UP") into six colour booleans (WUBRG; index 5 = colourless {C})
// and the mana value. It is an exact mirror of the engine's canonical
// conversion, cards/cmcFromManaCost (cards/face.go), because policynet must
// not import rules and there is no exported entry point: the value rule is
// repeated here, character for character, so the two cannot drift silently
// -- and TestManaCostBitsMirrorsEngine pins the parity on the shapes the
// corpus carries. A view's "no cost" (every land) or empty ManaCost is mana
// value 0 with no colours, exactly as cards.Face.ManaValue reports it.
func manaCostBits(cost string) ([6]bool, int) {
	cost = strings.TrimSpace(manaBraceForm.Replace(cost))
	var colours [6]bool
	if cost == "" || strings.EqualFold(cost, "no cost") {
		return colours, 0
	}
	mv := 0
	for _, tok := range costTokens(cost) {
		// Colour bits come from the token's own colour letters: a plain pip,
		// each '/'-separated face of a hybrid ("G/W"), or a twobrid's colour
		// half ("2/W" -> W). X/Y/Z and pure-generic tokens carry none.
		addColour(costTokenColours(tok), &colours)

		if tok == "X" { // {X} is 0 off the stack
			continue
		}
		if len(tok) == 1 && strings.ContainsRune("WUBRGC", rune(tok[0])) {
			mv++
			continue
		}
		if n, err := strconv.Atoi(tok); err == nil && n >= 0 {
			mv += n
			continue
		}
		if v, ok := twobridManaValue(tok); ok {
			// CR 202.4b: a monocolour hybrid's mana value is its generic
			// face. {2/W} is mana value 2 whether it is paid with two mana
			// or one white mana.
			mv += v
			continue
		}
		// Hybrid ("W/U"), Phyrexian ("UP"), and any other symbolic token:
		// one generic pip.
		mv++
	}
	return colours, mv
}

// costTokenColours returns the WUBRG C colour letters a mana-cost token
// names: a plain pip ("W"), each face of a hybrid ("G/W" -> G,W), a
// Phyrexian single colour ("UP" -> U), or a twobrid's colour half
// ("2/W" -> W, "2W" -> W). Generic digits and X/Y/Z yield nothing.
func costTokenColours(tok string) string {
	var b strings.Builder
	add := func(r rune) {
		if strings.ContainsRune("WUBRGC", r) {
			b.WriteRune(r)
		}
	}
	if strings.ContainsRune(tok, '/') {
		for _, p := range strings.Split(tok, "/") {
			if len(p) == 1 {
				add(rune(p[0]))
			}
		}
		return b.String()
	}
	// Phyrexian: a GC/WUBRG-first, P-second spelling ("UP"); or a bare pip.
	for i := 0; i < len(tok); i++ {
		add(rune(tok[i]))
	}
	return b.String()
}

// addColour ORs a token's colour letters into the WUBRG C mask.
func addColour(letters string, colours *[6]bool) {
	for i := 0; i < len(letters); i++ {
		switch letters[i] {
		case 'W':
			colours[0] = true
		case 'U':
			colours[1] = true
		case 'B':
			colours[2] = true
		case 'R':
			colours[3] = true
		case 'G':
			colours[4] = true
		case 'C':
			colours[5] = true
		}
	}
}

// twobridManaValue recognises Forge's concatenated ("2W") and slash
// ("2/W") monocolour-hybrid spellings, returning the generic face — the
// symbol's mana value. This is cards/twobridManaValue copied verbatim
// (cards/face.go) so the two stay in lockstep; that one in turn mirrors
// rules.ParseCost's twobrid parser without importing rules.
func twobridManaValue(sym string) (int, bool) {
	generic, col := "", ""
	if left, right, ok := strings.Cut(sym, "/"); ok {
		generic, col = left, right
	} else {
		i := 0
		for i < len(sym) && sym[i] >= '0' && sym[i] <= '9' {
			i++
		}
		if i == 0 {
			return 0, false
		}
		generic, col = sym[:i], sym[i:]
	}
	if len(col) != 1 || !strings.ContainsRune("WUBRGC", rune(col[0])) {
		return 0, false
	}
	v, err := strconv.Atoi(generic)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// mvOf is manaCostBits' mana-value half alone.
func mvOf(cost string) int {
	_, mv := manaCostBits(cost)
	return mv
}

// costTokens splits a cost string on whitespace but keeps each <...> group
// atomic, so a Forge non-mana token whose trailing "/description" contains
// spaces (e.g. "Sac<1/Creature.powerGE4/creature with power 4 or greater>")
// is one token rather than several. This mirrors rules.splitCostTokens; a
// plain strings.Fields would tear the description apart and mis-read its
// stray digits as generic mana. A cost with no <...> group tokenises exactly
// as Fields would.
func costTokens(s string) []string {
	var out []string
	i := 0
	for i < len(s) {
		if isSpaceByte(s[i]) {
			i++
			continue
		}
		start, depth := i, 0
		for i < len(s) {
			if s[i] == '<' {
				depth++
			} else if s[i] == '>' && depth > 0 {
				depth--
			} else if isSpaceByte(s[i]) && depth == 0 {
				break
			}
			i++
		}
		out = append(out, s[start:i])
	}
	return out
}

// isSpaceByte reports whether b is an ASCII whitespace byte. Forge costs are
// ASCII; rules's iterator uses unicode.IsSpace, which agrees on every byte
// that can appear in a cost string.
func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

// manaOnlyPips prices the MANA value of a formatCost rendering
// (decision.Option.Cost), ignoring every non-mana component. It mirrors the
// positive mana vocabulary of rules.ParseCost token by token: a token is
// mana only when it is X, S, a single WUBRGC letter, a non-negative integer,
// a hybrid, a Phyrexian, a twobrid or a hybrid-Phyrexian symbol. Anything
// else -- T, Forage, and every <head><N/Spec> component (Sac, Discard,
// Draw, SubCounter, AddCounter, Exile/ExileFromHand/ExileFromGrave, Reveal,
// Behold, tapXType, Blight, Return, PayLife, LifeX, DamageYou, PayEnergy) --
// is non-mana and contributes 0. Classifying by POSITIVE mana membership
// (rather than enumerating non-mana heads) is deliberate: a future cost
// component formatCost learns to render is priced 0 here unless it is also
// real mana, so no new component can silently become a phantom pip.
//
// A malformed or unrecognised token is non-mana here. That cannot diverge
// from the engine on real wire data: formatCost only ever emits recognised
// mana symbols and the known non-mana heads (an unparseable raw token is
// folded to generic mana before formatCost ever sees it), so every token in
// a real marker classifies.
func manaOnlyPips(cost string) int {
	cost = strings.TrimSpace(manaBraceForm.Replace(cost))
	if cost == "" || strings.EqualFold(cost, "no cost") {
		return 0
	}
	mv := 0
	for _, tok := range costTokens(cost) {
		switch {
		case tok == "X": // {X} is 0 off the stack
		case tok == "S": // snow mana: one pip
			mv++
		case len(tok) == 1 && strings.ContainsRune("WUBRGC", rune(tok[0])):
			mv++
		case isPlainDigits(tok):
			if n, err := strconv.Atoi(tok); err == nil {
				mv += n
			}
		case isHybridSym(tok), isPhyrexianSym(tok), isHybridPhyrexianSym(tok):
			mv++
		default:
			// Twobrid is the one mana symbol with a non-unit value; every
			// other token here is a non-mana component and contributes 0.
			if v, ok := twobridManaValue(tok); ok {
				mv += v
			}
		}
	}
	return mv
}

// isPlainDigits reports whether s is a non-empty run of ASCII digits with no
// sign or other character.
func isPlainDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isHybridSym mirrors rules.isHybrid: a two-colour hybrid pip, slash
// ("W/U") or concatenated ("WU"), the two colours distinct and WUBRGC.
func isHybridSym(sym string) bool {
	var a, b byte
	if len(sym) == 3 && sym[1] == '/' {
		a, b = sym[0], sym[2]
	} else if len(sym) == 2 && sym[1] != 'P' {
		a, b = sym[0], sym[1]
	} else {
		return false
	}
	return a != b && strings.ContainsRune("WUBRGC", rune(a)) && strings.ContainsRune("WUBRGC", rune(b))
}

// isPhyrexianSym mirrors rules.isPhyrexian: a WUBRG colour then P, slash
// ("W/P") or concatenated ("UP").
func isPhyrexianSym(sym string) bool {
	if len(sym) == 3 && sym[1] == '/' {
		return strings.ContainsRune("WUBRG", rune(sym[0])) && sym[2] == 'P'
	}
	if len(sym) != 2 {
		return false
	}
	return sym[1] == 'P' && strings.ContainsRune("WUBRG", rune(sym[0]))
}

// isHybridPhyrexianSym mirrors rules.isHybridPhyrexian: two distinct WUBRG
// colours and P, in either spelling ("G/W/P", "GWP", the P-first "PRG").
func isHybridPhyrexianSym(sym string) bool {
	if a, b, ok := stringsCutSlash(sym); ok {
		if len(a) != 1 || len(b) != 3 || b[1] != '/' {
			return false
		}
		if a[0] == 'P' {
			return b[0] != b[2] && strings.ContainsRune("WUBRG", rune(b[0])) &&
				strings.ContainsRune("WUBRG", rune(b[2]))
		}
		return b[2] == 'P' && a[0] != b[0] && strings.ContainsRune("WUBRG", rune(a[0])) &&
			strings.ContainsRune("WUBRG", rune(b[0]))
	}
	if len(sym) != 3 {
		return false
	}
	if sym[0] == 'P' {
		return sym[1] != sym[2] && strings.ContainsRune("WUBRG", rune(sym[1])) &&
			strings.ContainsRune("WUBRG", rune(sym[2]))
	}
	return sym[2] == 'P' && sym[0] != sym[1] && strings.ContainsRune("WUBRG", rune(sym[0])) &&
		strings.ContainsRune("WUBRG", rune(sym[1]))
}

// stringsCutSlash splits "a/b" at the first slash, returning ok=false when
// the slash is absent or at either end -- rules.splitHybridSlash's contract.
func stringsCutSlash(sym string) (a, b string, ok bool) {
	i := strings.IndexByte(sym, '/')
	if i <= 0 || i == len(sym)-1 {
		return "", "", false
	}
	return sym[:i], sym[i+1:], true
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
