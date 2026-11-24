package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Miss Highwater in a four-seat pod: a player who took the contract (the
// damaged player discarded their hand, drew seven and got a contract counter)
// and later loses the game while the game goes on is the case the card was
// marked two-seat only for: in two seats the loss ends the game, so the
// Effect's LosesGame trigger (copies of the loser's artifacts and creatures)
// never resolves. Gorge arms the Effect as a delayed LosesGame registration
// (effects.effEffect splits the comma list "LoseGameTrigger,ExileEffect"),
// whose fire puts the copy on the stack at the loss and mints the copies when
// it resolves, from the loser's board read at the loss (TriggerLostBoard).
// The kernel fires the same copy (dirac ctr_losscopy.go, the put's record
// marker) and agrees with it in a pod, lockstep: the game goes on past the
// loss and each contracted loss makes exactly the copies of the loser's
// artifacts and creatures.
//
// The pod: seat 0 plays Miss Highwater; seats 1-3 play short decks of
// creatures and a Manalith, so a seat that takes the contract (it draws seven)
// runs out of library soon after, and the game goes on past the loss.

var missHighwaterPodDecks = [4][]cardset.Entry{
	{{Name: "Miss Highwater", Count: 10}, {Name: "Dark Ritual", Count: 4}, {Name: "Swamp", Count: 16}},
	{{Name: "Savannah Lions", Count: 4}, {Name: "Manalith", Count: 2}, {Name: "Plains", Count: 8}},
	{{Name: "Grizzly Bears", Count: 4}, {Name: "Manalith", Count: 2}, {Name: "Forest", Count: 8}},
	{{Name: "Raging Goblin", Count: 4}, {Name: "Gray Ogre", Count: 3}, {Name: "Mountain", Count: 7}},
}

// missHighwaterPodTally counts what the games showed, read off gorge's logs.
type missHighwaterPodTally struct {
	games, contracts, contractLosses, copiedLosses, goneOn, copies, expected int
}

// missHighwaterPodTallyGame adds one finished game's events to y. Each
// contracted loss whose game goes on must copy exactly the artifacts and
// creatures the loser controlled at the loss. The cease sweep's battlefield
// moves name the loser's permanents -- a moved object keeps its owner, and in
// this pod nothing changes control, so owner is the controller the card
// copies for (a permanent the loser controlled but another seat owns is not
// in these decks). The copy's token mints follow its resolution, after every
// cease sweep of its loss batch ran; the losses of one batch (consecutive
// PlayerLost events, no mints between) resolve together and are checked as
// one sum. A batch whose Effect controller left before its copy resolved --
// gorge drops the queued fire then, and the kernel matches it -- and a batch
// still open at the game's end (the fires are dropped at G.Over, or the log
// was cut with the copy on the stack) are checked for nothing. It reports the
// first mismatched batch, if any.
func missHighwaterPodTallyGame(g *GorgeSide, y *missHighwaterPodTally) string {
	evs := g.E.L.Events
	var contract [MaxSeats]int
	var lost [MaxSeats]bool
	lostCount := 0
	lastGain := -1             // the seat of the most recent Contract counter gain
	armed := [MaxSeats][]int{} // per remembered seat, each armed LosesGame registration's Effect controller
	mints := []int{}           // each token mint's controller, in order
	mintsSinceLoss := false
	type pendLoss struct {
		loser, ctrl, board, tok0 int
	}
	var pend []*pendLoss
	check := func() string {
		for ctrl := 0; ctrl < MaxSeats; ctrl++ {
			if lost[ctrl] {
				continue // the controller left: gorge drops the queued fire
			}
			board, tok0, entries := 0, -1, 0
			for _, pe := range pend {
				if pe.ctrl != ctrl {
					continue
				}
				entries++
				board += pe.board
				if tok0 < 0 || pe.tok0 < tok0 {
					tok0 = pe.tok0
				}
			}
			if entries == 0 {
				continue
			}
			n := 0
			for _, m := range mints[tok0:] {
				if m == ctrl {
					n++
				}
			}
			y.copies += n
			y.expected += board
			if n != board {
				return fmt.Sprintf("a contracted loss batch copied %d tokens for controller %d, want the %d artifacts and creatures its losers controlled", n, ctrl, board)
			}
			y.copiedLosses += entries
		}
		pend = nil
		return ""
	}
	for i := range evs {
		switch e := &evs[i]; e.Kind {
		case events.PlayerCounterChange:
			if e.Counter == "Contract" && int(e.Player) < MaxSeats {
				contract[e.Player] += int(e.Amount)
				y.contracts++
				if e.Amount > 0 {
					lastGain = int(e.Player)
				}
			}
		case events.DelayedRegister:
			if !strings.HasPrefix(e.Text, "LosesGame:") || lastGain < 0 || int(e.Player) >= MaxSeats {
				continue
			}
			// the contract Effect just armed its LosesGame trigger on the seat
			// it put the contract counter on
			armed[lastGain] = append(armed[lastGain], int(e.Player))
		case events.CopyToken, events.TokenCreate:
			if int(e.Player) < MaxSeats {
				mints = append(mints, int(e.Player))
				mintsSinceLoss = true
			}
		case events.MoveZone:
			if e.From != state.ZBattlefield || !strings.Contains(e.Text, "player left the game") {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o == nil || int(o.Owner) >= MaxSeats || o.Face() == nil {
				continue
			}
			isCA := false
			for _, ty := range o.Face().Types {
				if ty == "Creature" || ty == "Artifact" {
					isCA = true
					break
				}
			}
			if !isCA {
				continue
			}
			for _, pe := range pend {
				if pe.loser == int(o.Owner) {
					pe.board++
				}
			}
		case events.PlayerLost:
			if mintsSinceLoss {
				if bad := check(); bad != "" {
					return bad
				}
				mintsSinceLoss = false
			}
			p := int(e.Player)
			if p >= MaxSeats || lost[p] {
				continue
			}
			lost[p] = true
			lostCount++
			if contract[p] == 0 {
				continue
			}
			y.contractLosses++
			if lostCount >= MaxSeats-1 {
				continue // the loss ends the game: gorge drops the queued fire
			}
			y.goneOn++ // two seats or more still stand after it: the game goes on
			if n := len(armed[p]); n > 0 {
				pend = append(pend, &pendLoss{loser: p, ctrl: armed[p][n-1], tok0: len(mints)})
			}
		}
	}
	if mintsSinceLoss {
		return check()
	}
	return ""
}

// TestFourSeatMissHighwaterPodsLockstep plays the pod over seeds in lockstep:
// no game diverges or stalls, and across them a seat that took the contract
// leaves the game while it goes on and the copy of its board is made.
func TestFourSeatMissHighwaterPodsLockstep(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 4)
	gd := make([][]*cards.Card, 4)
	md := make([][]cardset.Entry, 4)
	var all []string
	for s, d := range missHighwaterPodDecks {
		names[s] = "highwater-pod"
		md[s] = d
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			all = append(all, e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, [2][]string{all, nil}))
	tbl, _, err := set.MatchupN(md)
	if err != nil {
		t.Fatal(err)
	}
	var y missHighwaterPodTally
	for seed := uint64(1); seed <= 40; seed++ {
		cfg := Config{}
		m, err := NewMatchupN(reg, names, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100})
		CloseSide(k)
		y.games++
		if bad := missHighwaterPodTallyGame(g, &y); bad != "" {
			t.Errorf("seed %d: %s", seed, bad)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" {
			t.Errorf("seed %d: stall %q", seed, res.Stall)
			break
		}
	}
	t.Logf("%+v", y)
	// the class: seats that took the contract left a game that went on, and
	// each loss made the copy of its loser's artifacts and creatures
	if y.contractLosses < 3 || y.goneOn < 3 || y.copiedLosses < 3 {
		t.Fatalf("the pods never showed the class: %+v", y)
	}
	if y.copies != y.expected {
		t.Fatalf("the pods made %d tokens after contracted losses, want the %d artifacts and creatures the losers controlled: %+v", y.copies, y.expected, y)
	}
}
