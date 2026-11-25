package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// ChooseSource and the next-damage prevention it feeds (gorge effChooseSource
// then effEffect's DamageDone shield; dirac/act_choosesource.go,
// dirac/repl_prevchosen.go): Circle of Protection, Rune of Protection,
// Invulnerability, Righteous Aura, Pentagram of the Ages, Greater Realm of
// Preservation and Haazda Shield Mate. The kernel poses the
// controller's source pick over every permanent and stack spell the pool
// admits, in gorge's order, and the shield prevents the first damage from that
// source to the controller, so the life totals of the projection are gorge's
// at every decision.
//
// A four-seat pod: two shield decks (seats 0 and 2 by the seed's rotation) and
// two burn decks, so a pick has creatures, burn spells and other seats' damage
// to choose among.
func kchosenDecks() [4][]string {
	var shield, shield2, burn, burn2 []string
	for _, e := range []cardset.Entry{
		{Name: "Circle of Protection: Red", Count: 3}, {Name: "Circle of Protection: Artifacts", Count: 2},
		{Name: "Rune of Protection: Lands", Count: 2}, {Name: "Rune of Protection: White", Count: 2},
		{Name: "Greater Realm of Preservation", Count: 2}, {Name: "Invulnerability", Count: 3},
		{Name: "Righteous Aura", Count: 2}, {Name: "Pentagram of the Ages", Count: 1},
		{Name: "Haazda Shield Mate", Count: 2},
		{Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 18},
	} {
		shield = append(shield, deckOf(e.Count, e.Name)...)
	}
	for _, e := range []cardset.Entry{
		{Name: "Circle of Protection: Red", Count: 4}, {Name: "Rune of Protection: Lands", Count: 2},
		{Name: "Invulnerability", Count: 4}, {Name: "Righteous Aura", Count: 3},
		{Name: "Pentagram of the Ages", Count: 1}, {Name: "Haazda Shield Mate", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Mind Stone", Count: 2}, {Name: "Plains", Count: 18},
	} {
		shield2 = append(shield2, deckOf(e.Count, e.Name)...)
	}
	for _, e := range []cardset.Entry{
		{Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Shock", Count: 5},
		{Name: "Lightning Bolt", Count: 5}, {Name: "Short Sword", Count: 3}, {Name: "Mind Stone", Count: 2},
		{Name: "Mountain", Count: 17},
	} {
		burn = append(burn, deckOf(e.Count, e.Name)...)
	}
	for _, e := range []cardset.Entry{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Mind Stone", Count: 3}, {Name: "Short Sword", Count: 3},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	} {
		burn2 = append(burn2, deckOf(e.Count, e.Name)...)
	}
	return [4][]string{shield, burn, shield2, burn2}
}

// kchosenShields are the pod's shield cards.
var kchosenShields = map[string]bool{"Circle of Protection: Red": true, "Circle of Protection: Artifacts": true,
	"Rune of Protection: Lands": true, "Rune of Protection: White": true, "Greater Realm of Preservation": true,
	"Invulnerability": true, "Righteous Aura": true, "Pentagram of the Ages": true,
	"Haazda Shield Mate": true}

// kchosenPolicy is castFirstPolicy that holds the shields until the blockers
// are declared and then makes one, answering the source pick with a creature
// attacking the chooser (else a burn spell or creature of the pod), so the
// shield meets the damage it prevents. Any other decision is castFirstPolicy's.
func kchosenPolicy(seed uint64) Policy { return kchosenPolicyOf(seed, kchosenShields) }

// kchosenPolicyOf is kchosenPolicy over the shield cards named in shields.
func kchosenPolicyOf(seed uint64, shields map[string]bool) Policy {
	inner := castFirstPolicy(seed)
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		g := c.G.E.G
		name := func(a Canon) (string, *state.Object) {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
					return ob.Face().Name, ob
				}
			}
			return "", nil
		}
		switch c.D.Kind {
		case DChoose:
			var atk, other []Canon
			for _, a := range c.D.Options {
				n, ob := name(a)
				switch {
				case ob != nil && ob.IsAttacking && ob.Attacking == state.PlayerID(c.D.Player):
					atk = append(atk, a)
				case n == "Lightning Bolt" || n == "Shock" || n == "Hill Giant" || n == "Raging Goblin":
					other = append(other, a)
				}
			}
			if len(atk) > 0 {
				return atk[int(seed)%len(atk)]
			}
			if len(other) > 0 {
				return other[int(seed)%len(other)]
			}
		case DPriority:
			blockers := g.Step == state.StepDeclareBlockers
			buf = buf[:0]
			for _, a := range c.D.Options {
				k := a.Kind()
				if k != ActCast && k != ActActivate {
					continue
				}
				if n, _ := name(a); shields[n] == blockers {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && blockers {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			if !blockers {
				// the shields wait for the blockers: everything else is castFirst's
				buf = buf[:0]
				for _, a := range c.D.Options {
					if n, _ := name(a); !shields[n] || (a.Kind() != ActCast && a.Kind() != ActActivate) {
						buf = append(buf, a)
					}
				}
				d := *c.D
				d.Options = buf
				return inner.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return inner.Pick(c)
	})
}

// kchosenRun plays seeds lo..hi of the pod (the seed rotates the seats, bit 2 picks
// the autopay surface) and returns the first divergence ("" if none), the games
// that ended in a declared kernel stall and the number of games played.
func kchosenRun(t *testing.T, lo, hi uint64) (div string, stalls map[string]int) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("chosensource")
	if err != nil {
		t.Fatal(err)
	}
	base := kchosenDecks()
	stalls = map[string]int{}
	for seed := lo; seed <= hi; seed++ {
		var decks [4][]string
		for i := range decks {
			decks[i] = base[(i+int(seed))%4]
		}
		tbl, _, err := set.MatchupN(podFixtureLists(t, decks[:]))
		if err != nil {
			t.Fatal(err)
		}
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		if seed/4%2 == 1 {
			r.Surface = "autopay"
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 4000, 40
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		pol := kchosenPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s %v / %v", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return "", stalls
}

// TestChooseSourceShieldLockstep4Seat: the four-seat games agree with gorge at
// every decision, with the source pick posed, over permanents and spells, and
// answered.
func TestChooseSourceShieldLockstep4Seat(t *testing.T) {
	if isolate(t) {
		return
	}
	asked0, spell0 := dirac.ChooseSourceStats.Asked.Load(), dirac.ChooseSourceStats.Spell.Load()
	div, stalls := kchosenRun(t, 1, 300)
	asked, spell := dirac.ChooseSourceStats.Asked.Load()-asked0, dirac.ChooseSourceStats.Spell.Load()-spell0
	t.Logf("source picks posed %d, spell options offered %d, answered %d; stalls %v", asked, spell, dirac.ChooseSourceStats.Answered.Load(), stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if asked < 40 {
		t.Errorf("source picks posed %d, want at least 40", asked)
	}
	if spell == 0 {
		t.Error("no pick ever offered a spell on the stack")
	}
}

// TestChooseSourceShieldMutation: with the shield installed but preventing
// nothing the four-seat games diverge, so the pool does prevent damage.
func TestChooseSourceShieldMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevChosen = true
	defer func() { dirac.DebugNoPrevChosen = false }()
	div, _ := kchosenRun(t, 1, 120)
	if div == "" {
		t.Fatalf("no divergence in 120 games with the chosen-source shield disabled (asked %d answered %d spells %d abilities %d)", dirac.ChooseSourceStats.Asked.Load(), dirac.ChooseSourceStats.Answered.Load(), dirac.ChooseSourceStats.Spell.Load(), dirac.ChooseSourceStats.Ability.Load())
	}
	t.Logf("mutation diverged: %s", div)
}

// TestChooseSourceShieldReproSeed pins the first four-seat game of the pod
// (seed 1): seat 1 makes a Circle-style shield on an attacking creature and
// the next three damage that creature deals to it is prevented, a life total
// of 20 where the disabled shield has 17. The game replays clean and, with the
// shield disabled, diverges at that life total.
func TestChooseSourceShieldReproSeed(t *testing.T) {
	if isolate(t) {
		return
	}
	if div, _ := kchosenRun(t, 1, 1); div != "" {
		t.Fatalf("seed 1 diverged: %s", div)
	}
	dirac.DebugNoPrevChosen = true
	defer func() { dirac.DebugNoPrevChosen = false }()
	div, _ := kchosenRun(t, 1, 1)
	if !strings.HasPrefix(div, "projection Life") {
		t.Fatalf("seed 1 with the shield disabled: %q, want a life-total projection divergence", div)
	}
}
