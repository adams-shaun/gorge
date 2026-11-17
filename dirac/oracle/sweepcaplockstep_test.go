package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The captured-source sweep in lockstep, over a synthetic probe card built in
// process through kgen.TableFromCards (no corpus card carries the shape:
// DamageSource$ Remembered on a DamageAll names only Arcbond, whose trigger
// half is the sibling hook ticket). The probe is the capturing-emblem sweep
// shape (dirac/kgen/lower_sweepfrom.go) with a deathtouch, lifelink body, so
// the one-batch rules show in a real game: every hit carries the capture's
// deathtouch and the whole batch gains its lifelink once. Both sides play the
// same script; the table is in-memory and untracked.
//
// The ETB Effect targets a creature its controller controls, and the capture
// seat's deck holds no other creatures, so every capture is the probe itself
// and the sweep's source is always the keyworded body. The capture's
// CantBlockBy static makes it unblockable, so a clean game's capture is alive
// at every natural sweep. The gone-source game
// (TestSweepFromCapturedGoneLockstep) floods the capture seat with probes
// instead: two captures attack the same combat, and each sweep's deathtouch
// hits kill the other capture in the same damage step its own trigger fired
// in — the second trigger then resolves with its capture gone (dirac's NoSlot
// arm deals from the resolving effect; gorge's newDamageRider reads the gone
// source's keywords through LKI, CR 608.2h).

// sweepcapProbe is the probe script: one script for both games, the
// capturing-emblem sweep shape with a deathtouch, lifelink body.
const sweepcapProbe = `Name:Sweep Cap Probe
ManaCost:3
Types:Creature Frog Scout
PT:2/3
K:Deathtouch
K:Lifelink
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCap | TriggerDescription$ probe
SVar:TrigCap:DB$ Effect | ValidTgts$ Creature.YouCtrl | RememberObjects$ Targeted | ExileOnMoved$ Battlefield | StaticAbilities$ Unblockable | Triggers$ TrigDmg
SVar:Unblockable:Mode$ CantBlockBy | Description$ probe | ValidAttacker$ Card.IsRemembered
SVar:TrigDmg:Mode$ DamageDealtOnce | ValidSource$ Creature.IsRemembered | CombatDamage$ True | Execute$ DmgOther | TriggerDescription$ probe
SVar:DmgOther:DB$ DamageAll | DamageSource$ Remembered | NumDmg$ X | ValidPlayers$ Player | ValidCards$ Creature.!IsRemembered
SVar:X:TriggerCount$Amount
`

// sweepcapDecks are the two seats' decks: the capture seat holds the probe and
// basics only (every capture is the probe), the other seat vanilla creatures
// for the sweep to hit (a 6/6 Dreadmaw shows a deathtouch mark: 2 damage is
// not lethal without it). The gone game floods the capture seat with probes so
// two captures share a combat.
func sweepcapDecks(gone bool) [2][]cardset.Entry {
	if gone {
		return [2][]cardset.Entry{
			{{Name: "Sweep Cap Probe", Count: 16}, {Name: "Island", Count: 12}, {Name: "Plains", Count: 12}},
			{{Name: "Grizzly Bears", Count: 10}, {Name: "Hill Giant", Count: 6}, {Name: "Colossal Dreadmaw", Count: 6}, {Name: "Forest", Count: 18}},
		}
	}
	return [2][]cardset.Entry{
		{{Name: "Sweep Cap Probe", Count: 4}, {Name: "Island", Count: 18}, {Name: "Plains", Count: 18}},
		{{Name: "Grizzly Bears", Count: 10}, {Name: "Hill Giant", Count: 6}, {Name: "Colossal Dreadmaw", Count: 6}, {Name: "Forest", Count: 18}},
	}
}

// sweepcapBuild parses the probe script beside the decks' corpus fillers,
// lowers all of them into one in-process table (kgen.TableFromCards) and
// returns the set and the two seats' resolved decks.
func sweepcapBuild(t *testing.T, reg *cards.Registry, probeSrc string, decks [2][]cardset.Entry) (cardset.Set, [2][]*cards.Card) {
	t.Helper()
	probe, diags := cards.ParseBytes("sweepcap_probe.txt", []byte(probeSrc))
	if probe == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	probeName := probe.Faces[0].Name
	var pool []*cards.Card
	seen := map[string]bool{}
	var gd [2][]*cards.Card
	for s := range decks {
		for _, e := range decks[s] {
			if !seen[e.Name] {
				seen[e.Name] = true
				if e.Name == probeName {
					pool = append(pool, probe)
				} else {
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					pool = append(pool, c)
				}
			}
			for i := 0; i < e.Count; i++ {
				if e.Name == probeName {
					gd[s] = append(gd[s], probe)
					continue
				}
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, "sweepcap-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	return cardset.Raw(cardset.NewDataSet(pd)), gd
}

// sweepcapCounts is what the policy reads from gorge's log.
type sweepcapCounts struct {
	scanned    int
	hits       int // sweep damage facts whose source is the probe (non-combat)
	playerHits int // of those, recipients that are seats (the ValidPlayers$ Player arm)
	gains      int // life gains (the batch's one lifelink gain, and the blocker's)
}

// sweepcapPolicy casts, activates and plays lands whenever it can; in combat
// it attacks with everything and blocks with everything; else random. It
// counts the sweep's reach from gorge's log: DamageProvenance facts whose
// source is the probe (the attack's own combat damage excluded), the seat
// recipients among them, and the life gains.
func sweepcapPolicy(st *sweepcapCounts, srcName string) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		scanned := 0
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				evs := c.G.E.L.Events
				for ; scanned < len(evs); scanned++ {
					e := &evs[scanned]
					switch e.Kind {
					case events.DamageProvenance:
						if e.Obj == 0 || len(e.IDs) == 0 || strings.HasPrefix(e.Text, events.DamageProvenanceCombat) {
							continue
						}
						src := c.G.E.G.Obj(e.Obj)
						if src == nil || src.Face() == nil || src.Face().Name != srcName {
							continue
						}
						st.hits++
						if _, ok := e.IDs[0].PlayerRef(); ok {
							st.playerHits++
						}
					case events.LifeChange:
						if e.Amount > 0 {
							st.gains += int(e.Amount)
						}
					}
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch c.D.Kind {
				case DAttackers:
					if a.Kind() == ActAttack || a.Kind() == ActDone || a.Kind() == ActPass {
						buf = append(buf, a)
					}
				case DBlockers:
					if a.Kind() == ActBlock || a.Kind() == ActDone || a.Kind() == ActPass {
						buf = append(buf, a)
					}
				default:
					switch a.Kind() {
					case ActCast, ActCastFace, ActActivate, ActPlayLand:
						buf = append(buf, a)
					}
				}
			}
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

// sweepcapTable picks the fixture's matchup table out of the set.
func sweepcapTable(t *testing.T, set cardset.Set, decks [2][]cardset.Entry) *kop.Table {
	t.Helper()
	entries := make([][]cardset.Entry, 2)
	for s := range decks {
		entries[s] = decks[s]
	}
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

// sweepcapRun plays the fixture's lockstep over seeds 1..seeds; report fails
// on each divergence (else the first stops the run). mutate, when set, runs
// before the games (a kernel mutation: its reset is the caller's). Returns
// the number of divergent seeds.
func sweepcapRun(t *testing.T, reg *cards.Registry, set cardset.Set, gd [2][]*cards.Card, tbl *kop.Table,
	decks [2][]cardset.Entry, seeds uint64, report bool, st *sweepcapCounts, srcName string, mutate func()) int {
	t.Helper()
	if mutate != nil {
		mutate()
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"sweepcap-a", "sweepcap-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, sweepcapPolicy(st, srcName)(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	return div
}

// TestSweepFromCapturedLockstep: the capturing probe's lockstep is clean over
// the seeds, and the pool reached the sweep — the kernel's test-only counter
// and gorge's log both assert the reach (the emblem trigger fired, the players
// took the sweep's hits, the lifelink gained), so a vacuous pass cannot hide.
func TestSweepFromCapturedLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set, gd := sweepcapBuild(t, reg, sweepcapProbe, sweepcapDecks(false))
	decks := sweepcapDecks(false)
	tbl := sweepcapTable(t, set, decks)
	st := &sweepcapCounts{}
	var dstats dirac.DebugSweepCounters
	dirac.DebugSweepStats = &dstats
	defer func() { dirac.DebugSweepStats = nil }()
	div := sweepcapRun(t, reg, set, gd, tbl, decks, 40, true, st, "Sweep Cap Probe", nil)
	if dstats.Runs == 0 || dstats.PlHits == 0 {
		t.Fatalf("the pool never reached the sweep (runs %d, player hits %d): the lockstep proves nothing", dstats.Runs, dstats.PlHits)
	}
	if dstats.Gone != 0 {
		t.Errorf("%d sweeps resolved with the capture already gone in the live-capture games", dstats.Gone)
	}
	if st.hits == 0 || st.playerHits == 0 {
		t.Fatalf("gorge's log has no sweep damage from the probe (hits %d, player hits %d): the emblem trigger never fired", st.hits, st.playerHits)
	}
	if st.gains == 0 {
		t.Fatal("no life was gained in gorge; the capture's lifelink never applied")
	}
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	t.Logf("capture sweeps %d, player hits %d, log hits %d, player log hits %d, gains %d over 40 seeds",
		dstats.Runs, dstats.PlHits, st.hits, st.playerHits, st.gains)
}

// TestSweepFromCapturedLockstepMutation: with the sweep's player arm dropped
// some seed diverges — the proof that the lockstep observes the player arm (a
// player's life then differs). The kernel's counter asserts the sweeps ran
// with the mutation on (the clean test asserts their player hits); gorge's
// log is not the instrument here: the diverged game's tail is not comparable.
// Test-only globals: never parallel.
func TestSweepFromCapturedLockstepMutation(t *testing.T) {
	reg := synthReg(t)
	set, gd := sweepcapBuild(t, reg, sweepcapProbe, sweepcapDecks(false))
	decks := sweepcapDecks(false)
	tbl := sweepcapTable(t, set, decks)
	st := &sweepcapCounts{}
	var dstats dirac.DebugSweepCounters
	dirac.DebugSweepStats = &dstats
	defer func() { dirac.DebugSweepStats = nil }()
	div := sweepcapRun(t, reg, set, gd, tbl, decks, 40, false, st, "Sweep Cap Probe", func() { dirac.DebugSweepNoPlHits = true })
	dirac.DebugSweepNoPlHits = false
	if dstats.Runs == 0 {
		t.Fatal("no capture sweep ran with the mutation on; the mutation proves nothing")
	}
	if div == 0 {
		t.Fatal("no seed diverged with the sweep's player arm dropped: the pool does not reach it")
	}
}

// TestSweepFromCapturedGoneLockstep: a second seat whose capture died in the
// same combat damage step its trigger fired in exercises the gone-source
// fallback (dirac's NoSlot arm deals from the resolving effect; gorge reads
// the gone source's keywords through LKI).
func TestSweepFromCapturedGoneLockstep(t *testing.T) {
	reg := synthReg(t)
	set, gd := sweepcapBuild(t, reg, sweepcapProbe, sweepcapDecks(true))
	decks := sweepcapDecks(true)
	tbl := sweepcapTable(t, set, decks)
	st := &sweepcapCounts{}
	var dstats dirac.DebugSweepCounters
	dirac.DebugSweepStats = &dstats
	defer func() { dirac.DebugSweepStats = nil }()
	div := sweepcapRun(t, reg, set, gd, tbl, decks, 20, true, st, "Sweep Cap Probe", nil)
	if dstats.Gone == 0 {
		t.Fatal("no sweep resolved with the capture gone: the fallback is never reached")
	}
	if st.hits == 0 {
		t.Fatal("gorge's log has no sweep damage from the gone probe")
	}
	t.Logf("capture sweeps %d, gone %d, log hits %d over 20 seeds; %d seeds diverged", dstats.Runs, dstats.Gone, st.hits, div)
}
