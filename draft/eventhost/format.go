package eventhost

import "github.com/adams-shaun/gorge/draft/event"

// The event formats a create request may name. formatDraft is stored as ""
// so a draft event's meta marshals exactly as before the field.
const (
	formatDraft      = "draft"
	formatSealed     = "sealed"
	formatPrerelease = "prerelease"
)

// sealed reports a sealed-supply format (sealed or prerelease).
func (m meta) sealed() bool { return m.Format == formatSealed || m.Format == formatPrerelease }

// promos is the promo cards per seat: event.PrereleasePromos for a
// prerelease, else 0.
func (m meta) promos() int {
	if m.Format == formatPrerelease {
		return event.PrereleasePromos
	}
	return 0
}

// packsPerSeat is the booster count per seat: event.DefaultSealedPacks in a
// sealed format, else draftRounds.
func (m meta) packsPerSeat() int {
	if m.sealed() {
		return event.DefaultSealedPacks
	}
	return draftRounds
}

// supplyRounds is packsPerSeat()+promos(): the PackOpened rounds per seat.
func (m meta) supplyRounds() int { return m.packsPerSeat() + m.promos() }
