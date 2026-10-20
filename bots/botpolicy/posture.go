package botpolicy

import (
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// DeckShape is the A4 deck-shape classification derived from the deciding
// seat's own manifest. The zero value is ShapeMidrange so an unarmed Board
// stays exactly inert.
type DeckShape uint8

const (
	// ShapeMidrange is the zero value and the classification for any deck
	// whose curve is absent, inconsistent or between the two thresholds.
	ShapeMidrange DeckShape = iota
	// ShapeAggro is a nonland curve averaging at most 2.25.
	ShapeAggro
	// ShapeControl is a nonland curve averaging at least 3.01.
	ShapeControl
)

// String names the shape for a design note or a log line.
func (s DeckShape) String() string {
	switch s {
	case ShapeAggro:
		return "aggro"
	case ShapeControl:
		return "control"
	default:
		return "midrange"
	}
}

// PostureRules arms the A4 deck-shape posture. The zero value is off, so the
// default bot and every existing policy are byte-identical.
type PostureRules struct {
	// Attack arms R2 (release the AR4 blocker) and R3 (sim life weights).
	Attack bool
	// Reserve arms R4 (scale the C7 reserve preference by shape).
	Reserve bool
}

// Enabled reports whether any A4 rule is armed.
func (r PostureRules) Enabled() bool { return r.Attack || r.Reserve }

const (
	// shapeAggroMaxAvg100 is an average nonland card of at most 2.25
	// (100*S/N); 225 is the integer-floor boundary.
	shapeAggroMaxAvg100 int64 = 225
	// shapeControlMinAvg100 is Forge's ">3" on the exact mean: an average
	// of at least 3.01 (301 integer-floored) is control.
	shapeControlMinAvg100 int64 = 301
	// postureLifeBasePct is the unweighted life value.
	postureLifeBasePct int64 = 100
	// postureFavouredLifePct makes the favoured side's life worth half
	// again in the attack simulation.
	postureFavouredLifePct int64 = 150
	// postureAggroReserveMul makes an aggro deck tap out (reserve worth 0).
	postureAggroReserveMul int32 = 0
	// postureControlReserveMul makes a control deck keep instant mana up.
	postureControlReserveMul int32 = 2
)

// ShapeOf classifies m from its printed curve and main-deck land split.
// Copies are weighted, the deck NAME is never read, and any absent or
// inconsistent curve (or a nonland count that is not positive) is Midrange
// so a missing curve never reads as Aggro.
func ShapeOf(m *deck.Manifest) DeckShape {
	if m == nil || len(m.Curve) == 0 {
		return ShapeMidrange
	}
	var curveCount int
	var weighted int64
	for _, row := range m.Curve {
		curveCount += row.Count
		weighted += int64(row.CMC) * int64(row.Count)
	}
	var mainCount, landCount int
	for _, row := range m.Main {
		mainCount += row.Count
		if row.Land {
			landCount += row.Count
		}
	}
	if curveCount != mainCount {
		return ShapeMidrange
	}
	n := mainCount - landCount
	if n <= 0 {
		return ShapeMidrange
	}
	avg100 := 100 * weighted / int64(n)
	switch {
	case avg100 <= shapeAggroMaxAvg100:
		return ShapeAggro
	case avg100 >= shapeControlMinAvg100:
		return ShapeControl
	default:
		return ShapeMidrange
	}
}

// attackShape is the shape R2/R3 read: Midrange unless Attack is armed.
func (b Board) attackShape() DeckShape {
	if !b.Posture.Attack {
		return ShapeMidrange
	}
	return ShapeOf(b.OwnDeck)
}

// aggroReleasesBlocker reports whether an armed aggro posture may skip AR4's
// hold-back: the unblocked crack-back is summed over every opposing creature
// (double strike counted twice, defenders excluded) and only a life total
// strictly above it releases the blocker. A missing life fact never skips.
func (b Board) aggroReleasesBlocker(me state.PlayerID) bool {
	if b.attackShape() != ShapeAggro {
		return false
	}
	life, ok := b.Life.Lookup(me)
	if !ok {
		return false
	}
	var crack int32
	for _, c := range b.Creatures.All() {
		if c.Controller == me || c.Power <= 0 || c.hasKeyword("Defender") {
			continue
		}
		power := c.Power
		if c.hasKeyword("Double Strike") {
			power *= 2
		}
		crack += power
	}
	return life > crack
}

// postureLifePcts returns the R3 life weights: aggro favours the opponent's
// life, control favours its own, and Midrange (or an unarmed Board) leaves
// both at the base so the score is unchanged.
func (b Board) postureLifePcts() (own, opp int64) {
	switch b.attackShape() {
	case ShapeAggro:
		return postureLifeBasePct, postureFavouredLifePct
	case ShapeControl:
		return postureFavouredLifePct, postureLifeBasePct
	default:
		return postureLifeBasePct, postureLifeBasePct
	}
}

// reserveScale returns the C7 reserve scale for R4: aggro taps out, control
// doubles the preference, and Midrange (or an unarmed Board) is unchanged.
func (b Board) reserveScale(w CastWeights) int32 {
	if !b.Posture.Reserve {
		return w.ReserveScale
	}
	switch ShapeOf(b.OwnDeck) {
	case ShapeAggro:
		return w.ReserveScale * postureAggroReserveMul
	case ShapeControl:
		return w.ReserveScale * postureControlReserveMul
	default:
		return w.ReserveScale
	}
}

// scorePosture is score with the two life values weighted by the posture
// percentages. A dead side keeps score's win/loss sentinel.
func (w *simWorld) scorePosture(unit, ownPct, oppPct int64) int64 {
	s := w.score(unit)
	if w.life[w.me] <= 0 || w.life[w.opp] <= 0 {
		return s
	}
	lm, lo := lifeSimValue(w.life[w.me], unit), lifeSimValue(w.life[w.opp], unit)
	return s + (lm*ownPct/100 - lm) - (lo*oppPct/100 - lo)
}
