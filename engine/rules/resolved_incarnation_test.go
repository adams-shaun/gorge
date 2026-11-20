package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// hostDrain places queued triggers on the stack, answers any target ask with
// its first option, and resolves the stack empty.
func hostDrain(t *testing.T, e *Engine) {
	t.Helper()
	e.putTriggersOnStack()
	for i := 0; i < 20; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
			if len(d.Options) == 0 {
				t.Fatalf("target ask with no options %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
				t.Fatalf("submit: %v", err)
			}
			continue
		}
		if len(e.G.Stack) == 0 {
			if d := e.Pending(); d != nil && d.Kind != decision.KPriority && len(d.Options) > 0 {
				var ch []int
				for i := 0; i < d.Min && i < len(d.Options); i++ {
					ch = append(ch, d.Options[i].Index)
				}
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
					t.Fatalf("submit %v: %v", d.Kind, err)
				}
				continue
			}
			return
		}
		e.resolveTop()
	}
	t.Fatal("stack did not drain")
}

func hostEngine(t *testing.T, copies int) (*Engine, Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	host, ok := reg.Lookup("Harvestrite Host")
	if !ok {
		t.Fatal("corpus fixture: Harvestrite Host missing")
	}
	deck := make([]*cards.Card, copies)
	for i := range deck {
		deck[i] = host
	}
	cfg := seatZeroStart(Config{Seed: 9, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{append(deck, mountainDeck(t, 40-copies)...), mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	return e, cfg
}

// TestResolvedThisTurnIsPerIncarnation pins CR 400.7 for Count$ResolvedThisTurn
// (Harvestrite Host's "draw if this is the second time this ability has
// resolved this turn"): a blinked Host is a new object whose ability has not
// resolved yet, so its re-entry trigger reads 1 and does not draw. Farm window
// BLB-s111 xe:RAND:4:1:w11 step 17, where Forge and XMage both skip the draw.
func TestResolvedThisTurnIsPerIncarnation(t *testing.T) {
	t.Parallel()
	e, cfg := hostEngine(t, 1)
	host := moveByName(t, e, 0, "Harvestrite Host", state.ZBattlefield)
	hostDrain(t, e)
	hand := len(e.G.Zone(state.ZHand, 0))

	e.emit(eventMove(host, state.ZBattlefield, state.ZExile))
	e.emit(eventMove(host, state.ZExile, state.ZBattlefield))
	hostDrain(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand {
		t.Fatalf("blinked Host's re-entry trigger drew (hand %d -> %d); the returned object is new (CR 400.7)", hand, got)
	}
	// The returned object's own second resolution does count.
	e.emit(eventMove(host, state.ZBattlefield, state.ZExile))
	e.emit(eventMove(host, state.ZExile, state.ZBattlefield))
	hostDrain(t, e)
	replayCheck(t, e, cfg)
}

// TestResolvedThisTurnSameObjectStillCounts is the control: a second Host
// entering triggers the FIRST Host again (same object, second resolution), so
// it draws exactly once.
func TestResolvedThisTurnSameObjectStillCounts(t *testing.T) {
	t.Parallel()
	e, cfg := hostEngine(t, 2)
	moveByName(t, e, 0, "Harvestrite Host", state.ZBattlefield)
	hostDrain(t, e)
	hand := len(e.G.Zone(state.ZHand, 0))
	moveByName(t, e, 0, "Harvestrite Host", state.ZBattlefield)
	hostDrain(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand %d -> %d, want exactly one draw (first Host's second resolution)", hand, got)
	}
	replayCheck(t, e, cfg)
}

func eventMove(id state.ObjID, from, to state.Zone) events.Event {
	return events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to}
}

// resolvedThisTurnCarriers is every corpus card face whose script reads
// Count$ResolvedThisTurn (measured over .cards/cardsfolder: 34 script files; a double-faced card's
// name is the face that carries the SVar).
// All of them tally through events.ResolvedAbilityKey, which is keyed by the
// source's CR 400.7 identity (TestResolvedThisTurnIsPerIncarnation); a new
// carrier must be reviewed against that rule, so the set is pinned both ways.
var resolvedThisTurnCarriers = []string{
	"A-Omnath, Locus of Creation", "A-Sizzling Soloist", "Ashling the Pilgrim", "Ashling, Flame Dancer",
	"Belladonna Took", "Bronze Cudgels", "Captain Michael Burnham", "Elrond, Lord of Rivendell",
	"Gimli, Mournful Avenger", "Harvestrite Host", "Inner-Flame Igniter", "Jake Sisko, War Correspondent",
	"Ms. Bumbleflower", "Nissa, Leyline Tamer", "Nissa, Resurgent Animist", "Omnath, Locus of Creation",
	"Prowl, Pursuit Vehicle", "Rose Room Treasurer", "Rumor Gatherer",
	"Sahir, Visitor in Darkness", "Saint Traft and Rem Karolus", "Scythecat Cub",
	"Sephiroth, Fabled SOLDIER", "Sizzling Soloist", "Soulbright Flamekin",
	"Soulbright Seeker", "South Pole Voyager", "Tannuk, Memorial Ensign", "Teething Wurmlet",
	"Temporal Aperture", "Venom Connoisseur", "Victor, Valgavoth's Seneschal",
	"Vito, Fanatic of Aclazotz", "Zimone, Mystery Unraveler",
}

// TestResolvedThisTurnCarrierCensus pins the Count$ResolvedThisTurn class.
func TestResolvedThisTurnCarrierCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	got := map[string]bool{}
	for _, c := range reg.AllCards() {
		for fi := range c.Faces {
			f := c.Faces[fi]
			for _, body := range f.SVars {
				if strings.Contains(body, "Count$ResolvedThisTurn") {
					got[f.Name] = true
				}
			}
		}
	}
	want := map[string]bool{}
	for _, n := range resolvedThisTurnCarriers {
		want[n] = true
		if !got[n] {
			t.Errorf("%q no longer carries Count$ResolvedThisTurn: delete it from resolvedThisTurnCarriers", n)
		}
	}
	for n := range got {
		if !want[n] {
			t.Errorf("%q is a new Count$ResolvedThisTurn carrier: review it against the per-incarnation key and add it", n)
		}
	}
}
