package combat

import "github.com/adams-shaun/gorge/engine/state"

// Companion is CR 508.1c / 509.1b's whole-declaration restriction on
// declaring one creature as an attacker (or a blocker) beside others: "can
// only attack alone" (Master of Cruelties), "can't attack unless a creature
// with greater power also attacks" (Okk), "... at least two other creatures
// attack" (Orcish Conscripts), "... a black or green creature also attacks"
// (Scarred Puma). Like CR 506.5's attack-alone rule it is a property of the
// declared SET, so the per-pair readers never consult it: CompanionViolated
// is the one oracle the engine's validators and the option facts
// (decision.Option.AloneOnly, CompanionNeed, CompanionFrom) share.
type Companion uint8

// The companion vocabulary. CompanionNone is no restriction.
const (
	CompanionNone Companion = iota
	// CompanionOnlyAlone: no other creature is declared with this one.
	CompanionOnlyAlone
	// CompanionGreaterPower: another declared creature has greater power.
	CompanionGreaterPower
	// CompanionTwoOthers: at least two other creatures are declared.
	CompanionTwoOthers
	// CompanionBlackOrGreen: another declared creature is black or green.
	CompanionBlackOrGreen
)

// CompanionOf is id's companion restriction on the given side of the
// declaration, read from its derived keyword list.
func CompanionOf(b Board, id state.ObjID, attack bool) Companion {
	f := DerivedHiddenFlags(b, id)
	if attack {
		return f.AttackCompanion
	}
	return f.BlockCompanion
}

// Need is how many distinct qualifying companions the restriction asks for
// (0 for no restriction and for "only alone", which asks for none).
func (c Companion) Need() int {
	switch c {
	case CompanionGreaterPower, CompanionBlackOrGreen:
		return 1
	case CompanionTwoOthers:
		return 2
	}
	return 0
}

// CompanionCounts reports whether other, declared on the same side, counts
// toward id's restriction c. Power and colours are the derived
// characteristics (a pumped creature's real power).
func CompanionCounts(b Board, c Companion, id, other state.ObjID) bool {
	if other == id {
		return false
	}
	switch c {
	case CompanionGreaterPower:
		return b.Power(other) > b.Power(id)
	case CompanionTwoOthers:
		return true
	case CompanionBlackOrGreen:
		o := b.Game().Obj(other)
		if o == nil {
			return false
		}
		for _, col := range b.ObjectColors(o) {
			if col == 'B' || col == 'G' {
				return true
			}
		}
	}
	return false
}

// CompanionViolated reports whether declaring id (attacker when attack, else
// blocker) together with the declared creatures `others` (which may repeat a
// creature and may contain id) disobeys id's companion restriction.
func CompanionViolated(b Board, id state.ObjID, attack bool, others []state.ObjID) bool {
	c := CompanionOf(b, id, attack)
	if c == CompanionNone {
		return false
	}
	n := 0
	var distinct []state.ObjID
	for _, o := range others {
		if o == id {
			continue
		}
		dup := false
		for _, d := range distinct {
			if d == o {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		distinct = append(distinct, o)
		if c != CompanionOnlyAlone && CompanionCounts(b, c, id, o) {
			n++
		}
	}
	if c == CompanionOnlyAlone {
		return len(distinct) > 0
	}
	return n < c.Need()
}
