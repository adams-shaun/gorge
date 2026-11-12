//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestFourSeatMonarchHandoffLockstep plays four-seat pods of the monarch
// fixture pool dirac against gorge: monarch makers (Throne of the High City,
// Palace Sentinels, Thorn of the Black Rose, Court of Grace, Crimson Fleet
// Commodore) and the BecomeMonarch triggers of Custodi Lich and Knights of the
// Black Rose, beside cheap attackers and burn, so a monarch dies while two or
// more players remain. Then the active player becomes the monarch, or the next
// player in turn order when the active player is the one leaving (CR 725.4,
// gorge monarchHandoffOnDeparture), and draws at its end step. No divergence
// and no stall, some game hands the crown off with the game going on, and with
// the handoff dropped (dirac.DebugNoMonarchHandoff) some seed diverges.
func TestFourSeatMonarchHandoffLockstep(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("monarch")
	if err != nil {
		t.Fatal(err)
	}
	var wb, rw []string
	for _, e := range []cardset.Entry{
		{Name: "Knights of the Black Rose", Count: 2}, {Name: "Custodi Lich", Count: 2}, {Name: "Palace Sentinels", Count: 3},
		{Name: "Thorn of the Black Rose", Count: 2}, {Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Walking Corpse", Count: 3}, {Name: "Throne of the High City", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9},
	} {
		wb = append(wb, deckOf(e.Count, e.Name)...)
	}
	for _, e := range []cardset.Entry{
		{Name: "Palace Sentinels", Count: 3}, {Name: "Court of Grace", Count: 2}, {Name: "Crimson Fleet Commodore", Count: 2},
		{Name: "Goblin Piker", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Throne of the High City", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 7},
	} {
		rw = append(rw, deckOf(e.Count, e.Name)...)
	}
	decks := [4][]string{wb, rw, wb, rw}
	var lists [][]cardset.Entry
	for i, d := range decks {
		if len(d) != 40 {
			t.Fatalf("deck %d: %d cards", i, len(d))
		}
		es := make([]cardset.Entry, len(d))
		for j, n := range d {
			es[j] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	play := func(seed uint64) (Result, int) {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 8000, 80
		handoffs := 0
		rc.Finish = func(g *GorgeSide) { handoffs = monarchHandoffs(g) }
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		return Lockstep(m, seed, k, castFirstPolicy(seed), rc), handoffs
	}
	const seeds = 16
	total := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		res, n := play(seed)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		total += n
		t.Logf("seed %d: %d decisions, over %v %s; %d handoff(s)", seed, res.Decisions, res.Over, res.Stall, n)
	}
	if total == 0 {
		t.Fatal("no game handed the crown off: no monarch left a game that went on")
	}
	dirac.DebugNoMonarchHandoff = true
	defer func() { dirac.DebugNoMonarchHandoff = false }()
	for seed := uint64(1); seed <= seeds; seed++ {
		if res, _ := play(seed); res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("dropping the CR 725.4 handoff diverged on no seed")
}

// monarchHandoffs counts, in a finished gorge log, the monarch's departures
// that passed the crown on while two or more players remained: a PlayerLost of
// the monarch followed by a MonarchChange before any other designation-moving
// event (gorge's losePlayer emits the InitiativeChange, if any, first).
func monarchHandoffs(g *GorgeSide) int {
	mon, alive, n := -1, len(g.E.G.Players), 0
	departed := -1
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.PlayerLost:
			alive--
			departed = -1
			if int(e.Player) == mon && alive >= 2 {
				departed = mon
			}
		case events.MonarchChange:
			if departed >= 0 && int(e.Player) != departed {
				n++
			}
			departed = -1
			mon = int(e.Player)
		case events.InitiativeChange:
		default:
			departed = -1
		}
	}
	return n
}
