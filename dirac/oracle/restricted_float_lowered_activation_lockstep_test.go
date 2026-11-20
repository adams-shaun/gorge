package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestRestrictedFloatLoweredActivationReprosLockstep replays the divergent
// game: every land tapped, gorge pays a {G/W} activation from the restricted G
// Eclipsed Realms left floating (CR 106.6) where the lowering found no plan.
func TestRestrictedFloatLoweredActivationReprosLockstep(t *testing.T) {
	stdReproNamed(t, "restricted_float_lowered_activation")
}

// TestRestrictedFloatLoweredActivationMutation: with the lowering blind to
// floating restricted mana (the old behaviour) the same game diverges.
func TestRestrictedFloatLoweredActivationMutation(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "restricted_float_lowered_activation.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugRestrictedFloatIgnored
	dirac.DebugRestrictedFloatIgnored = true
	defer func() { dirac.DebugRestrictedFloatIgnored = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("the mutation (restricted float ignored) replayed the game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}

// restrFloatDeck holds Eclipsed Realms (restricted mana for the chosen type's
// spells and abilities) and Figure of Fable's {G/W} hybrid activations. Parcel
// Myr and Myr Reservoir are outside the std-ecl pool, so the Kithkin
// creatures stand for the restricted activations.
var restrFloatDeck = []string{
	"Eclipsed Realms", "Eclipsed Realms", "Eclipsed Realms", "Eclipsed Realms", "Eclipsed Realms", "Eclipsed Realms", "Eclipsed Realms", "Eclipsed Realms",
	"Figure of Fable", "Figure of Fable", "Figure of Fable", "Figure of Fable", "Figure of Fable", "Figure of Fable",
	"Kinscaer Sentry", "Eclipsed Kithkin", "Thoughtweft Lieutenant", "Safewright Cavalry", "Selfless Safewright",
	"Forest", "Forest", "Forest", "Forest", "Forest", "Forest",
}

// TestRestrictedFloatLoweredActivationFourSeat plays a four-seat pod of
// restricted-mana decks: no divergence, and an activation was paid from
// floating restricted mana at least once (so the test cannot pass empty).
func TestRestrictedFloatLoweredActivationFourSeat(t *testing.T) {
	set := cardsettest.Synth(t, "std-ecl")
	paid, last := 0, 0
	check := func(g *GorgeSide, _ Side) *Divergence {
		evs := g.E.L.Events
		for ; last < len(evs); last++ {
			e := &evs[last]
			if e.Kind == events.ManaAdd && e.Amount < 0 && strings.HasPrefix(e.Text, "mana-restriction:") && strings.Contains(e.Text, "Activated.") {
				paid++
			}
		}
		return nil
	}
	base := fourSeatSetPlay(t, set, [4][]string{restrFloatDeck, restrFloatDeck, restrFloatDeck, restrFloatDeck},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	play := func(seed uint64) Result {
		last = 0
		return base(seed)
	}
	fourSeatAtkClean(t, play, 20)
	t.Logf("%d activations paid from floating restricted mana", paid)
	if paid == 0 {
		t.Error("no activation was paid from floating restricted mana: the mechanism was never reached")
	}
}
