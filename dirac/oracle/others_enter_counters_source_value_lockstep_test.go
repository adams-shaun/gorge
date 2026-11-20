package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Arwen, Weaver of Hope: "Each other creature you control enters with an
// additional number of +1/+1 counters on it equal to Arwen's toughness"
// (CR 614.12). The amount reads Arwen, the replacement's source, not the
// entering creature: Stoneshock Giant (toughness 4) entered with 4 counters
// in the kernel and 1 in gorge. Recorded as tail-48 pair 3 (surface autopay).
func TestOthersEnterCountersSourceValueReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "others_enter_counters_source_value")
}

// TestOthersEnterCountersSourceValueMutation makes the kernel read the
// entering creature again: the recorded game diverges.
func TestOthersEnterCountersSourceValueMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "others_enter_counters_source_value.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugOthersEnterEntrantValue = true
	defer func() { dirac.DebugOthersEnterEntrantValue = false }()
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div == nil {
		t.Fatal("the mutation (entrant's value) replayed the recorded game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}

// arwenEntryTally counts, from gorge's own log, creatures entering under
// Arwen's controller while Arwen is on the battlefield and her printed
// toughness differs from the entrant's: the entries a wrong source (the
// entrant's own toughness) would count differently.
type arwenEntryTally struct {
	last, reached, any int
}

func (p *arwenEntryTally) walk(ref *GorgeSide) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; p.last < len(evs); p.last++ {
		e := &evs[p.last]
		if e.Kind != events.MoveZone || e.To != state.ZBattlefield {
			continue
		}
		o := g.Obj(e.Obj)
		if o == nil || o.Face() == nil || !o.Face().IsCreature() || o.Face().Name == "Arwen, Weaver of Hope" {
			continue
		}
		p.any++
		for _, id := range g.Zone(state.ZBattlefield, o.Controller) {
			a := g.Obj(id)
			if a != nil && a.Face() != nil && a.Face().Name == "Arwen, Weaver of Hope" && a.Controller == o.Controller &&
				a.Face().Toughness() != o.Face().Toughness() {
				p.reached++
				break
			}
		}
	}
}

// TestOthersEnterCountersSourceValueFourSeat plays four-seat pods of Arwen
// beside creatures of toughness 1 to 6: every creature that enters under
// Arwen's controller takes counters equal to Arwen's toughness. No seed may
// diverge, and the mechanism must be reached at least once.
func TestOthersEnterCountersSourceValueFourSeat(t *testing.T) {
	corpusDirOrSkip(t)
	set := cardsettest.Synth(t, "tail-48")
	deck := deckOf(6, "Arwen, Weaver of Hope", deckOf(7, "Mountain", deckOf(5, "Forest")...)...)
	for _, c := range []string{"Stoneshock Giant", "Ravenous Lindwurm", "Hellkite Charger", "Shockmaw Dragon",
		"Hall Monitor", "Dwarven Miner", "Lathnu Sailback", "Yuki-Onna"} {
		deck = append(deck, deckOf(3, c)...)
	}
	deck = append(deck, "Falkenrath Exterminator")
	var tally *arwenEntryTally
	base := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck}, nil,
		func(g *GorgeSide, _ Side) *Divergence {
			tally.walk(g)
			return nil
		},
		func(g *GorgeSide) { tally.walk(g) },
		func(seed uint64) Policy { return RandomPolicy(seed) })
	var reached, entries int
	play := func(seed uint64) Result {
		tally = &arwenEntryTally{}
		res := base(seed)
		reached += tally.reached
		entries += tally.any
		return res
	}
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	t.Logf("arwen: %d entries under Arwen's controller with a different toughness, %d creature entries in %d seeds", reached, entries, seeds)
	if reached == 0 {
		t.Fatalf("the mechanism was never reached in %d seeds: no creature entered under Arwen's controller with a toughness other than hers", seeds)
	}
}
