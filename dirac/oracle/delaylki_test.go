package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The end-step copy of a sacrificed artifact reads last known information, so it
// is still made after the artifact is exiled from the graveyard (CR 608.2h).
func TestDelayedCopyOfAnObjectThatMovedOn(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Esoteric Duplicator", Count: 8}, {Name: "Tormod's Crypt", Count: 10},
		{Name: "Mind Stone", Count: 6}, {Name: "Island", Count: 16},
	}
	lockstepRun(t, "delaylki", [2][]cardset.Entry{deck, deck}, 100, true, nil)
}
