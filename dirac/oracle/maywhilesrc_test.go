package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Effect MayPlay Duration$ AsLongAsControl
// ("you may play that card for as long as you control <this>"). Lightning,
// Security Sergeant (combat damage to a player) and Gwen Stacy (enters)
// exile the top card of their controller's library and grant its play while
// the source stays on the battlefield; gorge's continuousLive ends the
// grant when the source leaves. Shock and Lightning Bolt kill the source
// while a card is still exiled; Unsummon bounces it so it can be recast on a
// later turn, which revives the grant (gorge keeps the effect through
// cleanup); the rest are vanilla bodies and basics to play from exile.
var kmaywhilesrcDeck = []cardset.Entry{
	{Name: "Lightning, Security Sergeant", Count: 4}, {Name: "Gwen Stacy", Count: 4},
	{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3},
	{Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 2},
	{Name: "Unsummon", Count: 4},
	{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 3}, {Name: "Island", Count: 3},
}

// kmaywhilesrcCounts is read off gorge's finished event log.
type kmaywhilesrcCounts struct {
	exilePlays int // plays (cast or land) of a card out of exile
	ended      int // a source left the battlefield while a card its grant exiled was still exiled
	grants     int // cards a carrier exiled from the library top
	revived    int // a carrier re-entered on a later turn than its grant ended, a card still exiled
}

// kmaywhilesrcFinish walks the log: a carrier's exile of a library card is a
// grant; a carrier leaving the battlefield while such a card is still in
// exile ends the grant.
func kmaywhilesrcFinish(c *kmaywhilesrcCounts) func(*GorgeSide) {
	return func(g *GorgeSide) {
		onBF := 0
		endedTurn := false // a turn passed since the last grant ended
		exiled := map[state.ObjID]bool{}
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind == events.TurnChange {
				endedTurn = onBF == 0 && len(exiled) > 0
			}
			if e.Kind != events.MoveZone {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			name := ""
			if o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			carrier := name == "Lightning, Security Sergeant" || strings.HasPrefix(name, "Gwen Stacy")
			switch {
			case carrier && e.To == state.ZBattlefield:
				if onBF == 0 && endedTurn && len(exiled) > 0 {
					c.revived++
				}
				onBF++
			case carrier && e.From == state.ZBattlefield:
				onBF--
				if onBF == 0 && len(exiled) > 0 {
					c.ended++
				}
			case e.From == state.ZLibrary && e.To == state.ZExile && onBF > 0:
				exiled[e.Obj] = true
				c.grants++
			case e.From == state.ZExile:
				if exiled[e.Obj] && (e.To == state.ZStack || e.To == state.ZBattlefield) {
					c.exilePlays++
				}
				delete(exiled, e.Obj)
			}
		}
	}
}

// kmaywhilesrcGorgeSeed is skipped: a gorge finding
// (report). Ghost-Spider dies with its "exile the top card, you may play it
// this turn" ability on the stack; gorge resolves an activated ability with
// the SVars of its source's CURRENT face (rules/resolution_ability_ctx.go
// abilityResolutionSVars falls to src.Face()), the graveyard card is the
// Gwen Stacy front face, its table has no "Play" SVar, so the Effect
// registers no grant and the exiled card is unplayable. The kernel keeps the
// activated face's program and grants it. Not a kernel rules gap; do not
// work around it.
const kmaywhilesrcGorgeSeed = 35

func kmaywhilesrcRun(t *testing.T, seeds uint64, report bool, c *kmaywhilesrcCounts) (diverged bool, stalls int) {
	t.Helper()
	return kmaywhilesrcRunDeck(t, kmaywhilesrcDeck, kmaywhilesrcGorgeSeed, seeds, report, kmaywhilesrcFinish(c))
}

// kmaywhilesrcRunDeck is the lockstep loop over deck (both seats), skipping
// skipSeed (0: none), with finish reading gorge's log.
func kmaywhilesrcRunDeck(t *testing.T, deck []cardset.Entry, skipSeed, seeds uint64, report bool, finish func(*GorgeSide)) (diverged bool, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("maywhilesrc")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range deck {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], cd)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		if seed == skipSeed {
			continue
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kmaywhilesrc-a", "kmaywhilesrc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Finish: finish})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return true, stalls
		}
	}
	return false, stalls
}

func TestMaywhilesrcLockstep(t *testing.T) {
	var c kmaywhilesrcCounts
	div, stalls := kmaywhilesrcRun(t, 192, true, &c)
	t.Logf("192 seeds (seed %d skipped: gorge finding), %d divergences, %d stalls; grants %d, plays from exile %d, grants ended by the source leaving %d, revived on a later turn %d", kmaywhilesrcGorgeSeed, b2i(div), stalls, c.grants, c.exilePlays, c.ended, c.revived)
	if div || stalls > 0 {
		return
	}
	if c.exilePlays < 5 {
		t.Errorf("plays from exile: %d, want at least 5", c.exilePlays)
	}
	if c.ended < 1 {
		t.Errorf("grants ended by the source leaving: %d, want at least 1", c.ended)
	}
	if c.revived < 1 {
		t.Errorf("grants revived by the source returning on a later turn: %d, want at least 1", c.revived)
	}
}

// TestMaywhilesrcMutation: a record that ignores its source must make the
// lockstep diverge (the kernel offers a card gorge no longer lets be played).
func TestMaywhilesrcMutation(t *testing.T) {
	dirac.DebugMayWhileSrcForever = true
	defer func() { dirac.DebugMayWhileSrcForever = false }()
	var c kmaywhilesrcCounts
	if diverged, _ := kmaywhilesrcRun(t, 128, false, &c); !diverged {
		t.Fatal("DebugMayWhileSrcForever: no divergence in 128 seeds")
	}
}

// the grant also needs the source's controller.
// Both seats play the carriers (Lightning, Gwen Stacy, Victor Mancha) and Act
// of Treason, so each side steals the other's carrier: the grant ends for
// good when control changes, and stays ended when Act of Treason's end-of-turn
// return gives the carrier back with the card still in exile.
var kmaywhilesrcStealDeck = []cardset.Entry{
	{Name: "Lightning, Security Sergeant", Count: 4}, {Name: "Gwen Stacy", Count: 4},
	{Name: "Victor Mancha, Runaway", Count: 2}, {Name: "Act of Treason", Count: 4},
	{Name: "Shock", Count: 2}, {Name: "Unsummon", Count: 2},
	{Name: "Hill Giant", Count: 3}, {Name: "Savannah Lions", Count: 2},
	{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 3}, {Name: "Island", Count: 4},
}

// kmaywhilesrcStealCounts is read off gorge's finished event log. A carrier is
// not told apart per grant: a card in exile at the control change counts when
// any carrier is on the battlefield, so the counts are lower-bound evidence;
// the mutation test is the proof the kernel reads control.
type kmaywhilesrcStealCounts struct {
	exilePlays  int // plays (cast or land) of a card out of exile
	grants      int // cards a carrier exiled from the library top
	stolen      int // a carrier changed controller while such a card was still exiled
	afterReturn int // a stolen carrier came back to its owner, a card still exiled
}

func kmaywhilesrcStealFinish(c *kmaywhilesrcStealCounts) func(*GorgeSide) {
	return func(g *GorgeSide) {
		onBF := 0
		exiled := map[state.ObjID]bool{}
		ctrl := map[state.ObjID]state.PlayerID{} // a carrier's controller
		taken := map[state.ObjID]bool{}          // stolen with a card exiled
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			o := g.E.G.Obj(e.Obj)
			name := ""
			if o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			carrier := name == "Lightning, Security Sergeant" || strings.HasPrefix(name, "Gwen Stacy") || strings.HasPrefix(name, "Victor Mancha")
			switch {
			case e.Kind == events.ControlChange && carrier:
				prev, ok := ctrl[e.Obj]
				if !ok {
					prev = o.Owner
				}
				ctrl[e.Obj] = e.Player
				if e.Player != prev && e.Player != o.Owner && len(exiled) > 0 {
					c.stolen++
					taken[e.Obj] = true
				} else if e.Player == o.Owner && taken[e.Obj] {
					taken[e.Obj] = false
					if len(exiled) > 0 {
						c.afterReturn++
					}
				}
			case e.Kind != events.MoveZone:
			case carrier && e.To == state.ZBattlefield:
				onBF++
				delete(ctrl, e.Obj)
				delete(taken, e.Obj)
			case carrier && e.From == state.ZBattlefield:
				onBF--
			case e.From == state.ZLibrary && e.To == state.ZExile && onBF > 0:
				exiled[e.Obj] = true
				c.grants++
			case e.From == state.ZExile:
				if exiled[e.Obj] && (e.To == state.ZStack || e.To == state.ZBattlefield) {
					c.exilePlays++
				}
				delete(exiled, e.Obj)
			}
		}
	}
}

func TestMaywhilesrcStealLockstep(t *testing.T) {
	var c kmaywhilesrcStealCounts
	div, stalls := kmaywhilesrcRunDeck(t, kmaywhilesrcStealDeck, 0, 192, true, kmaywhilesrcStealFinish(&c))
	t.Logf("192 seeds, %d divergences, %d stalls; grants %d, plays from exile %d, carriers stolen with a card exiled %d, returned with a card still exiled %d", b2i(div), stalls, c.grants, c.exilePlays, c.stolen, c.afterReturn)
	if div || stalls > 0 {
		return
	}
	if c.stolen < 5 {
		t.Errorf("stolen: %d, want at least 5", c.stolen)
	}
	if c.afterReturn < 3 {
		t.Errorf("returned after a steal: %d, want at least 3", c.afterReturn)
	}
	if c.exilePlays < 5 {
		t.Errorf("plays from exile: %d, want at least 5", c.exilePlays)
	}
}

// TestMaywhilesrcStealMutation: a record that ignores the source's
// controller must make the lockstep diverge.
func TestMaywhilesrcStealMutation(t *testing.T) {
	dirac.DebugMayWhileSrcIgnoreCtrl = true
	defer func() { dirac.DebugMayWhileSrcIgnoreCtrl = false }()
	var c kmaywhilesrcStealCounts
	if diverged, _ := kmaywhilesrcRunDeck(t, kmaywhilesrcStealDeck, 0, 128, false, kmaywhilesrcStealFinish(&c)); !diverged {
		t.Fatal("DebugMayWhileSrcIgnoreCtrl: no divergence in 128 seeds")
	}
}
