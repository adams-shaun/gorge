package rules

// altcast_modes.go is the ONE descriptor of the alternative-cost keyword
// family: the cast modes whose cost is a printed keyword's parameter paid in
// place of the mana cost (CR 118.9). Every rules site that used to name one
// of these modes by its own string -- the cast-mode table, modeFlags, the
// potential-plan base cost, the hand and command-zone offer loops, the
// cost-static scope predicates and the per-mode pending-cast checks -- reads
// this table instead, so a new keyword of the family is one row here.
// TestAltCastModeLiteralsLiveInTheDescriptor (altcast_modes_test.go) freezes
// the string literals naming a row's mode anywhere else in rules/.
//
// The entry riders (evoke's sacrifice, dash's return, blitz's draw and
// sacrifice, warp's exile, impending's counters) stay in altcast.go's
// altCostEnter: they key on the pay-time CastFlags bit each row names, never
// on the mode string.

import (
	"math/bits"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// altCastID indexes altCastModes.
type altCastID uint8

const (
	altEvoke altCastID = iota
	altDash
	altOverload
	altWarp
	altImpending
	altMadness
	altBestow
	altBlitz
	altProwl
	altFreerunning
	// altAwaken (CR 702.113a): the cost half of K:Awaken:<N>:<cost>; the
	// rider is cards.kwAwaken's Condition$ Awakened chain.
	altAwaken
	// altSpectacle (CR 702.137a): the cost of K:Spectacle:<cost>, offered while
	// an opponent has lost life this turn (altGateSpectacle).
	altSpectacle
	altCastCount
)

// altCastCostKind says how a row's cost is read.
type altCastCostKind uint8

const (
	// altCostKeyword: keywordAltCost(face, head) -- the printed parameter
	// (Impending's two-field N:cost form included).
	altCostKeyword altCastCostKind = iota
	// altCostBestow: bestowCost, which cuts the colon metadata suffix and
	// withholds an unpriceable token.
	altCostBestow
	// altCostBlitzInstance: the ID-aware e.blitzCosts list (printed Blitz
	// plus layer-6 grants), matched by the selected offer mode.
	altCostBlitzInstance
)

// altCastMode is one row of the family.
type altCastMode struct {
	// id is the row's index, so a reader with only a *altCastMode (altCastFor,
	// the offer-loop rows) can address its sidecar entry in walkFaceFacts.
	id altCastID
	// mode is the decision.Option.Mode word the offer carries and beginCast
	// reads (the past participle the wire has always used).
	mode string
	// head is the printed K: keyword whose parameter is the cost.
	head string
	// flag is the pay-time CastFlags bit modeFlags records (0: none -- a
	// row with no provenance bit of its own; none today).
	flag uint64
	cost altCastCostKind
	// ph gates the hand walk's offer loop; handLoop/cmdLoop name the rows
	// that loop offers, in table order.
	ph       printedHeads
	handLoop bool
	cmdLoop  bool
	// untargeted rows are offered without the plain spell's target check
	// (overload's "each" replaces "target", CR 702.96b).
	untargeted bool
	// potential rows are priced by potentialModeBaseCost.
	potential bool
	// gate is the row's turn condition (altGateNone: always open).
	gate altCastGate
}

// altCastGate names the turn condition a row's offer waits on.
type altCastGate uint8

const (
	altGateNone altCastGate = iota
	// altGateProwl: the caster dealt combat damage to a player this turn
	// with a source sharing a creature type with the spell (CR 702.76a).
	altGateProwl
	// altGateFreerunning: the caster dealt combat damage to a player this
	// turn with an Assassin or their commander (CR 702.173a).
	altGateFreerunning
	// altGateAwaken: the caster controls a land the awakened cast can target
	// (awaken's "target land you control" is announced on the awakened cast
	// only, CR 601.2c, so a landless caster cannot choose the mode).
	altGateAwaken
	// altGateSpectacle: an opponent of the caster lost life this turn
	// (CR 702.137a).
	altGateSpectacle
)

// altCastModes is the family, indexed by altCastID. Row order is the order
// the hand and command-zone loops offer the modes, so it is part of the
// replayed option list: do not reorder rows.
var altCastModes = [altCastCount]altCastMode{
	altEvoke:     {id: altEvoke, mode: "evoked", head: "Evoke", flag: state.FlagEvoked, ph: phEvoke, handLoop: true, cmdLoop: true, potential: true},
	altDash:      {id: altDash, mode: "dashed", head: "Dash", flag: state.FlagDashed, ph: phDash, handLoop: true, cmdLoop: true, potential: true},
	altOverload:  {id: altOverload, mode: "overloaded", head: "Overload", flag: state.FlagOverloaded, ph: phOverload, handLoop: true, cmdLoop: true, untargeted: true, potential: true},
	altWarp:      {id: altWarp, mode: "warped", head: "Warp", flag: state.FlagWarped, ph: phWarp, handLoop: true, potential: true},
	altImpending: {id: altImpending, mode: "impended", head: "Impending", flag: state.FlagImpending, ph: phImpending, handLoop: true, cmdLoop: true, potential: true},
	// Madness stamps FlagAltCostPaid like prowl and awaken: on a face
	// carrying K:Madness that bit reads "this spell's madness cost was paid"
	// (altCostKeywordPaid), the provenance the Count$Madness fork heads and
	// the `madness` filter predicate read. The corpus's three Madness faces
	// (Avacyn's Judgment, From Under the Floorboards, Welcome to the Fold)
	// carry no other flagless alternative-cost row, so the bit is
	// unambiguous on one; a copy -- never cast (CR 707.10) -- keeps the bit
	// but resolves with no paid X, and FlagAltCostPaid is deliberately NOT a
	// CastProvenanceFlag (CR 707.10 copies the alternative costs paid).
	altMadness: {id: altMadness, mode: "madness", head: "Madness", flag: state.FlagAltCostPaid, potential: true},
	altBestow:  {id: altBestow, mode: "bestowed", head: "Bestow", flag: state.FlagBestowed, cost: altCostBestow, potential: true},
	altBlitz:   {id: altBlitz, mode: "blitzed", head: "Blitz", flag: state.FlagBlitzed, cost: altCostBlitzInstance},
	// Prowl and freerunning have no CastFlags bit of their own (the word is
	// full): each stamps FlagAltCostPaid, the bit for "an alternative cost
	// with no bit of its own was paid" (CR 118.9a allows one per cast). On a
	// face carrying the keyword that bit therefore reads "this keyword's cost
	// was paid" -- the prowled predicate and the Count$Freerunning head --
	// because the S:Mode$ AlternativeCost stamp is withheld from every face
	// that does not read Count$AltCost (faceWantsAltCost), and no Prowl or
	// Freerunning face does (TestProwlFreerunningCardsAreSupported pins it).
	altProwl:       {id: altProwl, mode: "prowled", head: "Prowl", flag: state.FlagAltCostPaid, ph: phProwl, handLoop: true, cmdLoop: true, potential: true, gate: altGateProwl},
	altFreerunning: {id: altFreerunning, mode: "freerun", head: "Freerunning", flag: state.FlagAltCostPaid, ph: phFreerunning, handLoop: true, cmdLoop: true, potential: true, gate: altGateFreerunning},
	// Awaken stamps FlagAltCostPaid like prowl: the Condition$ Awakened rider
	// reads it through altCostKeywordPaid(o, "Awaken"). No Awaken face reads
	// Count$AltCost (TestAwakenCardsAreSupported pins it), so the bit is
	// unambiguous on one.
	altAwaken: {id: altAwaken, mode: "awakened", head: "Awaken", flag: state.FlagAltCostPaid, ph: phAwaken, handLoop: true, cmdLoop: true, potential: true, gate: altGateAwaken},
	// Spectacle stamps FlagAltCostPaid like prowl and awaken: on a face
	// carrying K:Spectacle that bit reads "this spell's spectacle cost was
	// paid" (altCostKeywordPaid), the provenance the `spectacle` filter
	// predicate reads (Rafter Demon's Card.Self+spectacle, Rix Maadi
	// Reveler's Card.spectacle). No Spectacle face carries another flagless
	// alternative-cost row, so the bit is unambiguous on one.
	altSpectacle: {id: altSpectacle, mode: "spectacle", head: "Spectacle", flag: state.FlagAltCostPaid, ph: phSpectacle, handLoop: true, potential: true, gate: altGateSpectacle},
}

// spellTypeLedger is the layer-4 table a prowl gate reads the spell's
// types through (Engine's EffectiveTypes).
type spellTypeLedger interface {
	EffectiveTypes() []effects.ObjectTypes
}

// altGateHost is what a row's turn gate reads: the layer-4 type table and the
// target census.
type altGateHost interface {
	spellTypeLedger
	awakenTargetReader
}

// gateOpen reports whether row m's turn condition holds for caster p
// casting spell id: prowl and freerunning read combatHitThisTurn.
func (m *altCastMode) gateOpen(g *state.Game, h altGateHost, p state.PlayerID, id state.ObjID) bool {
	switch m.gate {
	case altGateProwl:
		o := g.Obj(id)
		if o == nil {
			return false
		}
		return combatHitThisTurn(g, p, prowlHit(damageSnapshotTypes(id, o, h.EffectiveTypes())))
	case altGateFreerunning:
		return combatHitThisTurn(g, p, freerunningHit(g, p))
	case altGateAwaken:
		return awakenLandAvailable(h, g, p, id)
	case altGateSpectacle:
		return opponentLostLifeThisTurn(g, p)
	}
	return true
}

// altMode is row id's mode word.
func altMode(id altCastID) string { return altCastModes[id].mode }

// altCastFor is the row whose mode is mode, or nil. A Blitz grant instance
// ("blitzed_grant_N") is canonicalised to "blitzed" by beginCast before any
// reader here sees it.
func altCastFor(mode string) *altCastMode {
	for i := range altCastModes {
		if altCastModes[i].mode == mode {
			return &altCastModes[i]
		}
	}
	return nil
}

// altCastIs reports whether mode is row id's mode.
func altCastIs(mode string, id altCastID) bool { return mode == altCastModes[id].mode }

// altFaceCost is one row's compiled printed cost on a face: walkFaceFacts'
// sidecar entry. ok is faceCostRaw's ok at compile time; only present rows
// (ok true) are stored in walkFaceFacts.altCosts, indexed via altCostMask.
type altFaceCost struct {
	cost Cost
	ok   bool
}

// faceCostRaw is the row's printed cost on f, parsed; ok is false when the
// face does not carry the keyword (or carries an unpriceable bestow). A
// Blitz row's grants are ID-aware: beginCast and the walks read e.blitzCosts
// for it. This is the per-face compile's source (computeWalkFaceFacts) and
// faceCost's fallback for a face with no current sidecar.
func (m *altCastMode) faceCostRaw(f *cards.Face) (Cost, bool) {
	if m.cost == altCostBestow {
		return bestowCost(f)
	}
	return keywordAltCost(f, m.head)
}

// faceCost is faceCostRaw read from the sidecar ff (walkFaceFacts) when its
// keyword half is current for f, and parsed fresh otherwise. The parameter
// is a pure function of the face, so the offer walk, the command-zone offer,
// the potential-plan pricing and the charge all share ONE compiled parse per
// face instead of reparsing on every offer. The sidecar is sparse: m.id's
// entry sits at the population count of altCostMask's lower bits, and an
// absent bit means the face does not carry the keyword (no reparse).
func (m *altCastMode) faceCost(f *cards.Face, ff *walkFaceFacts) (Cost, bool) {
	if ff == nil || !ff.keywordsCurrent(f) {
		return m.faceCostRaw(f)
	}
	bit := uint16(1) << m.id
	if ff.altCostMask&bit == 0 {
		return Cost{}, false
	}
	c := ff.altCosts[bits.OnesCount16(ff.altCostMask&(bit-1))]
	return c.cost, c.ok
}

// altCastModeEntries are castModeCodes' rows for the family, all sharing the
// alternative-cost keyword arm.
func altCastModeEntries() []state.StrEntry[castModeCode] {
	out := make([]state.StrEntry[castModeCode], 0, len(altCastModes))
	for i := range altCastModes {
		out = append(out, state.StrEntry[castModeCode]{Key: altCastModes[i].mode, Val: castModeAltCostKeyword})
	}
	return out
}
