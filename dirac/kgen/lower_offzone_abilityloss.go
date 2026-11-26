package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// graveLossReadersDone gates Yixlid Jailer's lowering. The static lowers to
// kop.StatGraveNoAbilities whatever its value, but while it is false the card
// carries a named refusal, so no game plays it: the kernel's graveyard
// readers (dsub-jailer, split 1: the keyword and offer readers, split 2: the
// trigger scans and the leave-graveyard look-back, split 3: the graveyard
// statics and mana abilities) are not all wired. Split 3 flips it on.
var graveLossReadersDone = false

// graveLossPending is the refusal a Jailer-shaped card carries until the
// readers are done.
const graveLossPending = "graveyard ability loss: readers pending (dsub-jailer split 2/3)"

// offZoneAbilityLoss lowers a Continuous static whose AffectedZone$ is
// Graveyard and which removes every ability of the cards it affects (Yixlid
// Jailer: "Cards in graveyards lose all abilities", CR 613.1f). gorge
// applies the removal to a graveyard card only when the static's
// AffectedZone$ names the graveyard or every zone (rules abilityloss.go
// removalApplies), and reads it live from the battlefield: the kernel keeps
// no mark on the card (kop.StatGraveNoAbilities, dirac/static_graveabilityloss.go).
//
// Only the corpus shape lowers: Affected$ Card, nothing beside the removal
// but Mode$ and Description$. Any other key, and any other Affected$,
// refuses by name.
func (lc *lowerCtx) offZoneAbilityLoss(sp *params, st *lStatic, aff, az string) (handled bool, err error) {
	if aff != "Card" {
		return true, refuse("AffectedZone %q RemoveAllAbilities with Affected %q", az, aff)
	}
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedZone", "RemoveAllAbilities", "Description":
		default:
			return true, refuse("AffectedZone %q RemoveAllAbilities beside %s$", az, k)
		}
	}
	if st.Cond.Kind != cNone {
		return true, refuse("AffectedZone %q RemoveAllAbilities with a condition", az)
	}
	f, err := lc.filter(aff)
	if err != nil {
		return true, err
	}
	st.F = f
	st.Kind = "GraveNoAbilities"
	st.Zone = int(zGraveyard)
	sp.consume("AffectedZone", "RemoveAllAbilities")
	if !graveLossReadersDone {
		lc.refuseNode(lc.guardNode, "%s", graveLossPending)
	}
	return true, nil
}

// graveNoAbilities builds a GraveNoAbilities static: Card in the graveyard
// (dirac/static_graveabilityloss.go).
func (cb *cardBuilder) graveNoAbilities(st *kop.Static, s lStatic) (err error) {
	st.Kind = kop.StatGraveNoAbilities
	st.Zone = kop.ZoneGraveyard
	st.Filter, err = cb.filter(s.F, kop.ZoneGraveyard)
	return err
}
