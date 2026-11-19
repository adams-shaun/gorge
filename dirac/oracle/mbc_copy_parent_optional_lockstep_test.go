package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Sevinne's Reclamation's "if this spell was cast from a graveyard, you may
// copy this spell and may choose a new target for the copy": an Optional$
// CopySpellAbility Defined$ Parent (dirac/kgen/lower_copyspell.go, an
// OpMayAsk under MayGuardCopyParent). gorge poses its copy_optional yes/no
// once the copy is possible; the flashback casts reach it. Cheap creatures
// trade in combat so the graveyard holds targets, in two-seat games and
// four-seat pods.
var copyParentOptionalDeck = []cardset.Entry{
	{Name: "Sevinne's Reclamation", Count: 8}, {Name: "Savannah Lions", Count: 6},
	{Name: "Elite Vanguard", Count: 6}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Plains", Count: 10}, {Name: "Forest", Count: 6},
}

func TestCopyParentOptionalLockstep(t *testing.T) {
	t.Parallel()
	d := copyParentOptionalDeck
	names := []string{"Sevinne's Reclamation"}
	tally := func(g *state.Game) int { return cedhZoneCount(g, state.ZExile, names[0]) } // flashback casts
	two := cedhPodRun(t, "copyparentopt", [][]cardset.Entry{d, d}, names, 40, tally)
	four := cedhPodRun(t, "copyparentopt", [][]cardset.Entry{d, d, d, d}, names, 16, tally)
	t.Logf("flashback casts: %d in two-seat games, %d in pods", two, four)
	if two < 10 || four < 10 {
		t.Fatalf("too little evidence: %d two-seat, %d four-seat (want 10 each)", two, four)
	}
}
