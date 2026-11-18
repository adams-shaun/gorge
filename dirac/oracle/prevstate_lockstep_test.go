package oracle

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The scoped-prevention-with-a-rider lockstep (dirac/repl_prevx.go): Chant of
// Vitu-Ghazi prevents all damage creatures would deal this turn and its
// controller gains life equal to the damage prevented (gorge
// damageReplacementPreventsWithRider runs the GainLife body for each damage
// event). The expected answer at every decision is gorge's: the life totals
// are in the projection, so a missed or extra gain diverges.
//
// Gorge offers a Convoke cast only when the credit of the untapped creatures
// leaves nothing for the lands to pay (its planner prices no convoke cast, so
// a Chant paid by lands is no play of the canonical list): the first deck is
// mostly Savannah Lions, eight or more of which must be untapped at once.
func kchantDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Chant of Vitu-Ghazi", Count: 8}, {Name: "Savannah Lions", Count: 26},
			{Name: "Plains", Count: 6},
		},
		kprevscopeDecks[1],
	}
}

// kchantPolicy is castFirstPolicy that casts a Chant whenever gorge offers one
// and counts, from gorge's event log, the life gains: neither deck has another source of life gain (no lifelink, no
// gain effect), so each positive life change of the games is a Chant's
// rider run.
func kchantPolicy(gains *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		seen := 0
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.G.E != nil && c.G.E.L != nil {
				evs := c.G.E.L.Events
				for ; seen < len(evs); seen++ {
					if evs[seen].Kind == events.LifeChange && evs[seen].Amount > 0 {
						gains.Add(1)
					}
				}
			}
			if c.G != nil && c.D.Kind == DPriority {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActCast {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Chant of Vitu-Ghazi" {
							return a
						}
					}
				}
			}
			return inner.Pick(c)
		})
	}
}

// kaweDecks is the Awe Strike lockstep (dirac/repl_prevnext.go's
// source-keyed shield, kop.PrevNextFrom): the next damage the remembered
// creature deals is prevented and its controller gains life equal to the
// damage prevented (gorge damageReplacementPreventsWithRider runs the
// GainLife body, the Effect exiles itself). The expected answer at every
// decision is gorge's: the life totals are in the projection, so a missed,
// extra or early gain diverges. The first deck is mostly Savannah Lions, so
// the cast Awe Strikes have small creatures to target; neither deck has
// another source of life gain, so each positive life change of the games is
// an Awe Strike's rider run.
func kaweDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Awe Strike", Count: 8}, {Name: "Savannah Lions", Count: 12},
			{Name: "Plains", Count: 20},
		},
		kprevscopeDecks[1],
	}
}

// kawePolicy is kchantPolicy for Awe Strike: it casts an Awe Strike whenever
// gorge offers one and counts the life gains from gorge's event log.
func kawePolicy(gains *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		seen := 0
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.G.E != nil && c.G.E.L != nil {
				evs := c.G.E.L.Events
				for ; seen < len(evs); seen++ {
					if evs[seen].Kind == events.LifeChange && evs[seen].Amount > 0 {
						gains.Add(1)
					}
				}
			}
			if c.G != nil && c.D.Kind == DPriority {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActCast {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Awe Strike" {
							return a
						}
					}
				}
			}
			return inner.Pick(c)
		})
	}
}

// TestAweStrikeLockstep proves the source-keyed shield's rider runs: the
// games agree with gorge and Awe Strike gains life in them.
func TestAweStrikeLockstep(t *testing.T) {
	t.Parallel()
	var gains atomic.Int64
	lockstepRunPolicy(t, "prevstate", kaweDecks(), 900, true, nil, nil, kawePolicy(&gains))
	if gains.Load() == 0 {
		t.Fatal("Awe Strike never gained life in the games; the lockstep proves nothing")
	}
	t.Logf("Awe Strike life gains: %d", gains.Load())
}

// TestAweStrikeMutation proves the pool gains the prevented life: with the
// rider skipped (the prevention still applying), some seed diverges.
func TestAweStrikeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevRider = true
	defer func() { dirac.DebugNoPrevRider = false }()
	var gains atomic.Int64
	if lockstepRunPolicy(t, "prevstate", kaweDecks(), 900, false, nil, nil, kawePolicy(&gains)) == 0 {
		t.Fatal("no seed diverged with the rider skipped: the pool never has damage prevented by Awe Strike")
	}
}

func TestChantLockstep(t *testing.T) {
	t.Parallel()
	var gains atomic.Int64
	lockstepRunPolicy(t, "prevstate", kchantDecks(), 900, true, nil, nil, kchantPolicy(&gains))
	if gains.Load() == 0 {
		t.Fatal("Chant of Vitu-Ghazi never gained life in the games; the lockstep proves nothing")
	}
	t.Logf("Chant life gains: %d", gains.Load())
}

// TestChantMutation proves the pool gains the prevented life: with the rider
// skipped (the prevention still applying), some seed diverges.
func TestChantMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevRider = true
	defer func() { dirac.DebugNoPrevRider = false }()
	var gains atomic.Int64
	if lockstepRunPolicy(t, "prevstate", kchantDecks(), 900, false, nil, nil, kchantPolicy(&gains)) == 0 {
		t.Fatal("no seed diverged with the rider skipped: the pool never has damage prevented by Chant")
	}
}

// kriaDecks is the Ria Ivor lockstep (dirac/repl_prevnext.go's token-rider
// shield, kop.PrevNextTokens): the next combat damage a remembered creature
// deals to a player this combat is prevented and its controller creates that
// many Mites, the Effect exiles itself, and the record ends with the combat.
// Relentless Assault gives extra combats, so the first record is freed at the
// first combat's end before the second one is made.
func kriaDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Ria Ivor, Bane of Bladehold", Count: 4}, {Name: "Relentless Assault", Count: 4},
			{Name: "Savannah Lions", Count: 10}, {Name: "Hill Giant", Count: 2},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		},
		kprevscopeDecks[1],
	}
}

// kriaPolicy prefers casting, and declaring every creature that can attack,
// over the random pick (the karaumiPolicy shape), and counts, from gorge's
// event log, the Phyrexian Mite tokens: deck 1 holds no other token maker, so
// each Mite is Ria's rider run.
//
// It also declines a Relentless Assault cast in a turn that has already
// granted an extra phase: a second cast-grant resolving beside a pending one
// is an interleaving the kernel's addPhaseGrant does not model (a pre-existing
// lowering gap of the AddPhase pool, reached only by casting two in one
// phase), and the random pick would reach it.
//
// With bias it prefers the active player's Ria Ivor as every target (the
// kbtssProbe shape): a 3/4 survives being blocked, so her rider record is the
// one an unspent first combat leaves into the extra combat Relentless Assault
// grants, and the end-of-combat mutation has a carrier to collide with. The
// same bias holds Ria out of the first combat of a turn (an attacker stays
// tapped for the turn's later combats) and declines every block from the
// second combat on: the doubly-remembered carrier then deals its damage to a
// player, where gorge's one live Effect and the kernel's two records must
// disagree.
func kriaPolicy(mites *atomic.Int64, bias bool) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		seen := 0
		grants := 0 // extra-phase grants since the turn began
		return PolicyFunc(func(c *PickCtx) Canon {
			if bias && c.D.Kind == DBlockers && c.G != nil && c.G.E.G.CombatsThisTurn > 1 {
				for _, a := range c.D.Options {
					if a.Kind() == ActDone {
						return a
					}
				}
			}
			if bias && c.D.Kind == DTarget && c.G != nil {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActTarget {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Ria Ivor, Bane of Bladehold" &&
							ob.Controller == c.G.E.G.Active { // Ria herself: 3/4, blocked and alive, attacking again in the extra combat
							return a
						}
					}
				}
			}
			if c.G != nil && c.G.E != nil && c.G.E.L != nil {
				evs := c.G.E.L.Events
				for ; seen < len(evs); seen++ {
					e := &evs[seen]
					switch e.Kind {
					case events.TurnChange:
						grants = 0
					case events.ExtraPhase:
						if e.Amount > 0 {
							grants++
						}
					case events.TokenCreate:
						if mites != nil && strings.Contains(e.Text, "phyrexian_mite") {
							mites.Add(1)
						}
					}
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack:
					if bias && a.Kind() == ActAttack && c.G != nil && c.G.E.G.CombatsThisTurn <= 1 &&
						refCardName(c.G, a.Src()) == "Ria Ivor, Bane of Bladehold" {
						continue // the carrier sits the first combat out, untapped for the later ones
					}
					if a.Kind() == ActCast && grants > 0 && c.G != nil {
						if o, ok := c.G.GorgeOption(a); ok {
							if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Relentless Assault" {
								continue
							}
						}
					}
					buf = append(buf, a)
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

// kriaRun plays seeds 1..seeds of the riavor matchup under kriaPolicy, with
// the harness's seed%4 autopay coverage, and counts the clean games, the
// divergences and the kernel stalls. Report mode stops at the first
// divergence with its detail; otherwise it plays on.
func kriaRun(t *testing.T, seeds uint64, report bool, mites *atomic.Int64, bias bool) (clean, divs, stalls int) {
	t.Helper()
	reg, set, gd, tbl := kriaMatchup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"riavor-a", "riavor-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		polf := kriaPolicy(mites, bias)
		res := Lockstep(m, seed, k, polf(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Fatalf("seed %d diverges at decision %d: %s", seed, d.At, d.What)
			}
			continue
		}
		clean++
	}
	return clean, divs, stalls
}

// kriaMatchup builds the kernel table of kriaDecks on the riavor fixture pool,
// or skips the test with the interaction guard's reason: Ria Ivor carries both
// halves of the infect stall herself (her ExtPreventNext is a damage
// replacement the conversion would have to order against, and her Mite token
// is toxic), so every matchup with her is refused until the frozen damage
// pass's conversion ordering is modelled (the order ask's walk deals its
// events as plain damage, kw_infect.go prepInfect). The lockstep waits on
// that rule work; nothing about the shield itself is unproven at this skip.
func kriaMatchup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	decks := kriaDecks()
	reg, set, gd := lockstepPool(t, "riavor", decks)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		var inter *cardset.InteractionRefusal
		if errors.As(err, &inter) {
			t.Skipf("the interaction guard refuses Ria Ivor's matchup (%s): %s", inter.Reason, inter.Why)
		}
		t.Fatal(err)
	}
	return reg, set, gd, tbl
}

// TestRiaIvorLockstep proves the token-rider shield's mint runs: the games
// agree with gorge and Ria's rider creates Mites in them.
func TestRiaIvorLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var mites atomic.Int64
	clean, divs, stalls := kriaRun(t, 400, true, &mites, false)
	t.Logf("riavor: %d seeds, %d clean, %d divergent, %d kernel stalls", 400, clean, divs, stalls)
	if mites.Load() == 0 {
		t.Fatal("Ria Ivor never created a Mite in the games; the lockstep proves nothing")
	}
	t.Logf("Ria Mite tokens: %d", mites.Load())
}

// TestRiaIvorMutation proves the pool mints the Mites: with the rider skipped
// (the prevention still applying), some seed diverges.
func TestRiaIvorMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevRider = true
	defer func() { dirac.DebugNoPrevRider = false }()
	var mites atomic.Int64
	clean, divs, stalls := kriaRun(t, 400, false, &mites, false)
	t.Logf("riavor mutation: %d seeds, %d clean, %d divergent, %d kernel stalls", 400, clean, divs, stalls)
	if divs == 0 {
		t.Fatal("no seed diverged with the rider skipped: the pool never has damage prevented by Ria Ivor")
	}
}

// TestRiaIvorEndCombatMutation proves the pool ends the records with the
// combat: with prevEndCombat dropped, a first combat's unspent record sits
// beside the next combat's on one creature, and some game diverges or ends at
// the rider capacity stall (gorge asks the CR 616.1 order over the two).
func TestRiaIvorEndCombatMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevEndCombat = true
	defer func() { dirac.DebugNoPrevEndCombat = false }()
	reg, set, gd, tbl := kriaMatchup(t)
	var mites atomic.Int64
	divs, stalls := 0, 0
	for seed := uint64(1); seed <= 400 && divs == 0; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"riavor-a", "riavor-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		polf := kriaPolicy(&mites, true)
		pol := polf(seed)
		if seed%8 == 0 {
			pol = polf(seed ^ 0x9e3779b97f4a7c15) // a different line: see kriaPolicy's grant gate
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			divs++
			t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
		}
	}
	if divs == 0 && stalls == 0 {
		t.Fatal("no seed diverged or stalled with the end-of-combat drop kept: the pool never carries a first combat's record into the next")
	}
	t.Logf("Ria end-of-combat mutation: %d divergences, %d capacity stalls over the seeds played", divs, stalls)
}
