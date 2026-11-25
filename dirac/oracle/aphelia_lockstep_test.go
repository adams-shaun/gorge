package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Aphelia, Viper Whisperer's lockstep (dirac/kgen/lower_zone.go emblem gate,
// lower_damagedoneonce.go ddoEvent arm): the {4}{B} Effect's end-of-turn
// DamageDoneOnce line lowers as the per-referent DamageDone emblem line, and
// both engines latch one instance per damaged player per damage batch (CR
// 603.2c; gorge trigger_delayed.go checkEventDelayedTriggers, dirac
// fireEmblemsDmg's refLatch). The deck is Aphelia plus Snake creatures, so
// Aphelia's own attack makes more Snakes and two Gorgons/Snakes can hit the
// same player in one combat damage step after the ability is activated.
//
// The counters measure the scenario the lowering must hold: the policy's own picks
// of Aphelia's activation, the activation resolving (a DelayedPush, and
// dirac's fireEmblemsDmg actually queueing), combat damage a Gorgon/Snake
// deals a PLAYER read from events.DamageProvenance (a Damage event carries
// the recipient, not the source), and each batch resolving into a LoseLife of
// half the player's current life (TriggeredTarget$LifeTotal/HalfUp) — so a
// game where the emblem never resolves, or one engine fires per Damage event
// while the other latches the batch, fails here before the life comparison
// even matters.
func kapheliaDeck() []string {
	var deck []string
	for i := 0; i < 6; i++ {
		deck = append(deck, "Aphelia, Viper Whisperer")
	}
	for i := 0; i < 10; i++ {
		deck = append(deck, "Skeletal Snake")
	}
	for i := 0; i < 4; i++ {
		deck = append(deck, "Python")
	}
	for len(deck) < 40 {
		deck = append(deck, "Swamp")
	}
	return deck
}

// kapheliaStats is what the policy counts from the policy's own picks and
// gorge's event log.
type kapheliaStats struct {
	acts     int      // Aphelia {4}{B} activations the policy picked
	delayed  int      // DelayedPush events (the Effect's registered trigger firing)
	dmgHits  int      // combat damage events from a Gorgon/Snake to a player
	sameStep int      // steps where a second such source hit one player
	maxStep  int      // most such sources on one player in one step
	popped   int      // halvings that resolved (the LoseLife after a counted hit)
	lifeBad  []string // resolved halvings that were not half the current life
	life     map[state.PlayerID]int32
	pend     map[state.PlayerID]int
	perTgt   map[state.PlayerID]int
}

// kapheliaPolicy tracks gorge's event log and counts its own picks. The
// counter's life table is event-derived (foldDamage's player arm and
// foldLifeChange), so a halving is checked against the life the engine
// actually held when it resolved.
func kapheliaPolicy(st *kapheliaStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		scanned := 0
		seeded := false
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				if !seeded {
					// one game's tables: the life totals start from the state
					// the first decision sees (the log's earlier events are
					// already folded into it), and the scan starts past them.
					seeded = true
					scanned = len(c.G.E.L.Events)
					st.life = map[state.PlayerID]int32{}
					st.pend = map[state.PlayerID]int{}
					st.perTgt = map[state.PlayerID]int{}
					for p := range c.G.E.G.Players {
						st.life[state.PlayerID(p)] = c.G.E.G.Players[p].Life
					}
				}
				evs := c.G.E.L.Events
				for ; scanned < len(evs); scanned++ {
					e := &evs[scanned]
					switch e.Kind {
					case events.StepChange:
						clear(st.perTgt) // a new step: the per-player hit counts restart
					case events.DelayedPush:
						st.delayed++
					case events.Damage:
						if e.Obj != 0 || int(e.Player) >= len(c.G.E.G.Players) {
							continue
						}
						// foldDamage's player arm: the life total moves by the
						// recorded loss (a LifeReduced replacement's value when
						// one rides the event), never for an infect hit.
						if e.Amount > 0 && strings.Contains(e.Counter, "infect") {
							continue
						}
						loss := e.Amount
						if v, ok := events.DamageLifeLoss(*e); ok {
							loss = v
						}
						st.life[e.Player] -= loss
					case events.LifeChange:
						if int(e.Player) >= len(c.G.E.G.Players) {
							continue
						}
						before := st.life[e.Player]
						st.life[e.Player] += e.Amount
						if e.Amount >= 0 || st.pend[e.Player] == 0 {
							continue
						}
						st.pend[e.Player]--
						st.popped++
						// the halving: the player lost exactly half their life
						// total at the resolution, rounded up (X:
						// TriggeredTarget$LifeTotal/HalfUp) — a batch firing or
						// a lost emblem cannot produce this loss.
						if before > 0 && -e.Amount != int32((before+1)/2) {
							st.lifeBad = append(st.lifeBad, fmt.Sprintf("seat %d lost %d with %d life", e.Player, -e.Amount, before))
						}
					case events.DamageProvenance:
						// the damage SOURCE rides only the provenance fact (a
						// player hit's Damage event has Obj 0); combat is the
						// Text's first field, as foldDamageProvenance cuts it.
						if !strings.HasPrefix(e.Text, events.DamageProvenanceCombat) || len(e.IDs) == 0 {
							continue
						}
						p, ok := e.IDs[0].PlayerRef()
						if !ok || int(p) >= len(c.G.E.G.Players) {
							continue // an object recipient, not a player hit
						}
						src := c.G.E.G.Obj(e.Obj)
						if src == nil || src.Face() == nil {
							continue
						}
						isSnake := false
						for _, s := range src.Face().Types {
							if s == "Snake" || s == "Gorgon" {
								isSnake = true
							}
						}
						if !isSnake || e.Amount <= 0 {
							continue
						}
						st.dmgHits++
						st.perTgt[p]++
						if st.perTgt[p] > st.maxStep {
							st.maxStep = st.perTgt[p]
						}
						if st.perTgt[p] == 2 {
							st.sameStep++
						}
						st.pend[p]++ // the hit's halving is now expected
					}
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			var ans Canon
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				ans = rnd.Pick(c)
			} else {
				d := *c.D
				d.Options = buf
				ans = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			if c.G != nil && ans.Kind() == ActActivate {
				// count what the policy CHOSE, not what was offered
				for _, o := range c.D.Options {
					if o != ans {
						continue
					}
					if o, ok := c.G.GorgeOption(o); ok && o.Kind == "ability" {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil &&
							ob.Zone == state.ZBattlefield && ob.Face().Name == "Aphelia, Viper Whisperer" {
							st.acts++
						}
						break
					}
				}
			}
			return ans
		})
	}
}

// apheliaCorpusOrSkip skips when this tree has no card corpus, including a tree
// with no git repository to resolve one from (a fresh export): corpusDirOrSkip
// fails there, and a corpus lockstep must skip, never fail.
func apheliaCorpusOrSkip(t *testing.T) {
	t.Helper()
	dir, ok := testutil.SharedCorpusDir(".")
	if !ok {
		t.Skip("no git repository to resolve the .cards corpus from")
	}
	if _, err := os.Stat(cards.CorpusDir(dir)); err != nil {
		t.Skip("no .cards corpus present")
	}
}

func TestApheliaLockstep(t *testing.T) {
	apheliaCorpusOrSkip(t)
	reg := synthReg(t)
	deck := kapheliaDeck()
	decks := [2][]string{deck, deck}
	set := printedDelveSet(t, decks)
	entries := make([]cardset.Entry, len(deck))
	for i, n := range deck {
		entries[i] = cardset.Entry{Name: n, Count: 1}
	}
	tbl, _, _, err := set.Matchup(entries, entries)
	if err != nil {
		t.Fatal(err)
	}
	var cardDecks [2][]*cards.Card
	for s := range decks {
		for _, n := range decks[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], c)
		}
	}
	st := new(kapheliaStats)
	pol := kapheliaPolicy(st)
	var dstats dirac.EffEOTTrigStats
	dirac.DebugEffEOTTrigStats = &dstats
	defer func() { dirac.DebugEffEOTTrigStats = nil }()
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"aphelia-a", "aphelia-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			break
		}
	}
	if st.acts == 0 {
		t.Fatal("the policy never picked Aphelia's {4}{B} activation; the lockstep proves nothing")
	}
	if st.dmgHits == 0 || st.sameStep == 0 || st.maxStep < 2 {
		t.Fatalf("no combat step had two Gorgon/Snake sources hit one player (hits %d, two-source steps %d); the lockstep proves nothing",
			st.dmgHits, st.sameStep)
	}
	if st.delayed == 0 {
		t.Fatal("the Effect's registered trigger never fired in gorge; the lockstep proves nothing")
	}
	if dstats.DmgFires == 0 {
		t.Fatal("dirac never queued an emblem DamageDone trigger (fireEmblemsDmg); the lowered line never fired")
	}
	if st.popped == 0 {
		t.Fatal("no Aphelia halving resolved in gorge; the emblem never affected a life total")
	}
	if st.popped < st.sameStep {
		t.Errorf("only %d halvings resolved in gorge for %d two-source steps; each such batch should resolve one", st.popped, st.sameStep)
	}
	if len(st.lifeBad) != 0 {
		t.Errorf("halvings that were not half the current life: %v", st.lifeBad)
	}
	t.Logf("Aphelia: %d activations picked, %d delayed-trigger pushes, %d Gorgon/Snake damage events to players, %d steps with two sources on one player (max %d), %d halvings resolved",
		st.acts, st.delayed, st.dmgHits, st.sameStep, st.maxStep, st.popped)
}
