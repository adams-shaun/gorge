package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The no-RememberObjects$ Effect-static lockstep:
// Goblin Maskmaker's attack trigger (DB$ Effect, Mode$ ReduceCost on
// face-down spells) and the three Defender creatures whose Effect grants
// Mode$ CanAttackDefender | ValidCard$ Card.EffectSource (Krotiq Nestguard's
// activation, Territorial Witchstalker's begin-combat trigger, Stalked
// Researcher's eerie triggers), against face-down creatures, enchantments,
// power-4 creatures and burn. The kernel's WhileHost / EOT emblems must
// decide every attack declaration and every cost the way gorge's continuous
// effects do.

var keffnoremDecks = [2][]cardset.Entry{
	// R/B face-down deck: Maskmaker beside megamorph / disguise creatures
	{
		{Name: "Goblin Maskmaker", Count: 4}, {Name: "Kolaghan Stormsinger", Count: 4},
		{Name: "Bolrac-Clan Basher", Count: 4}, {Name: "Basilica Stalker", Count: 3},
		{Name: "Priest of Urabrask", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 1}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 14}, {Name: "Swamp", Count: 2},
	},
	// G/U/W Defender carriers, enchantments and power-4 creatures
	{
		{Name: "Krotiq Nestguard", Count: 2}, {Name: "Territorial Witchstalker", Count: 2},
		{Name: "Stalked Researcher", Count: 2}, {Name: "Journey to Nowhere", Count: 4},
		{Name: "Pacifism", Count: 4}, {Name: "Colossal Dreadmaw", Count: 4}, {Name: "Earth Elemental", Count: 4},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 5}, {Name: "Plains", Count: 4},
	},
}

var keffnoremDefenders = map[string]bool{"Krotiq Nestguard": true, "Territorial Witchstalker": true, "Stalked Researcher": true}

type keffnoremStats struct {
	games, divs, stalls int
	attacks             map[string]int // attacks by a Defender carrier (each is live-Effect only)
	faceOffered         int
	maskAtk, castFace   int         // Maskmaker attacks; face-down casts of any kind
	paid                map[int]int // mana spent per face-down cast in a live-Maskmaker turn, from gorge's pool
	discounted          int         // face-down casts after Goblin Maskmaker attacked that turn
	firstDiv            uint64
}

// keffnoremLive reports whether the deciding player's Goblin Maskmaker has
// attacked this turn.
func keffnoremLive(c *PickCtx, maskTurn, maskPl int) bool {
	return c.G != nil && int(c.G.E.G.Turn) == maskTurn && int(c.D.Player) == maskPl
}

// keffnoremMaskReady reports whether the deciding player controls a Goblin
// Maskmaker that can still attack this turn: the policy then keeps its mana
// for the post-combat Priest and face-down cast.
func keffnoremMaskReady(c *PickCtx) bool {
	if c.G == nil {
		return false
	}
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
		ob := c.G.E.G.Obj(id)
		if ob != nil && ob.Face() != nil && ob.Face().Name == "Goblin Maskmaker" && !ob.Tapped && !ob.SummonSick {
			return true
		}
	}
	return false
}

// keffnoremPolicy attacks whenever it can, activates, casts and plays lands
// otherwise, else random. A ready Maskmaker's player keeps its mana for the
// post-combat Priest of Urabrask (floating {R}{R}{R}) and a face-down cast.
func keffnoremPolicy(seed uint64, st *keffnoremStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	maskTurn, maskPl := -1, -1
	pre, preLive := -1, false
	pool := func(c *PickCtx) int {
		n := 0
		for _, v := range c.G.E.G.Players[c.D.Player].Pool {
			n += int(v)
		}
		return n
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if pre >= 0 && c.G != nil {
			if preLive {
				st.paid[pre-pool(c)]++
			}
			pre = -1
		}
		buf = buf[:0]
		hasAtk := false
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace {
				st.faceOffered++
			}
			if a.Kind() == ActAttack {
				hasAtk = true
				buf = append(buf, a)
			}
		}
		hold := !hasAtk && seed%3 != 0 && !keffnoremLive(c, maskTurn, maskPl) && keffnoremMaskReady(c)
		if hold {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActPlayLand || k == ActPass {
					buf = append(buf, a)
				}
			}
		}
		if !hasAtk && !hold {
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActPlayLand, ActCast, ActActivate:
					buf = append(buf, a)
				case ActCastFace:
					// a face-down cast is offered only from floating mana
					// (gorge's planner prices no face cast): hold it for the
					// turn the Maskmaker attacked, plus every third seed
					if keffnoremLive(c, maskTurn, maskPl) || seed%3 == 0 {
						buf = append(buf, a)
					}
				}
			}
			// the Priest's {R}{R}{R} floats the mana a face-down cast is
			// offered from (gorge's planner prices none): cast it once the
			// discount is live
			if keffnoremLive(c, maskTurn, maskPl) || seed%3 == 0 {
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && optionFaceName(c, a) == "Priest of Urabrask" {
						buf = append(buf, a, a, a)
					}
				}
			}
		}
		a := Canon(0)
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G == nil {
			return a
		}
		switch a.Kind() {
		case ActAttack:
			if name := optionFaceName(c, a); keffnoremDefenders[name] {
				st.attacks[name]++
			} else if name == "Goblin Maskmaker" {
				maskTurn, maskPl = int(c.G.E.G.Turn), int(c.D.Player)
				st.maskAtk++
			}
		case ActCastFace:
			st.castFace++
			pre, preLive = pool(c), keffnoremLive(c, maskTurn, maskPl)
			if int(c.G.E.G.Turn) == maskTurn && int(c.D.Player) == maskPl {
				st.discounted++
			}
		}
		return a
	})
}

// keffnoremDropCanAttack removes every CanAttackDefender static from the
// built emblems (the mutation: the defender can't attack); keffnoremZeroReduce
// zeroes every emblem ReduceCost amount. Both copy the emblem table only.
func keffnoremMutate(t *kop.Table, drop bool) (*kop.Table, int) {
	m := *t
	m.Emblems = append([]kop.Emblem(nil), t.Emblems...)
	n := 0
	for i := range m.Emblems {
		abs := make([]kop.Ability, 0, len(m.Emblems[i].Abilities))
		for _, a := range m.Emblems[i].Abilities {
			if a.Kind == kop.AbStatic {
				switch {
				case drop && a.Static.Kind == kop.StatCanAttackDefender:
					n++
					continue
				case !drop && a.Static.Kind == kop.StatReduceCost:
					a.Static.A, a.Static.StFlags = 0, a.Static.StFlags&^kop.StDynamic
					n++
				}
			}
			abs = append(abs, a)
		}
		m.Emblems[i].Abilities = abs
	}
	return &m, n
}

// runEffnorem plays seeds 1..seeds; mutation: 0 none, 1 drop the
// CanAttackDefender statics, 2 zero the ReduceCost amounts.
func runEffnorem(t *testing.T, seeds, mutation int) *keffnoremStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effnorem")
	if err != nil {
		t.Fatal(err)
	}
	st := &keffnoremStats{attacks: map[string]int{}, paid: map[int]int{}}
	var decks [2][]*cards.Card
	for s, d := range keffnoremDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(keffnoremDecks[0], keffnoremDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutation != 0 {
		var n int
		if tbl, n = keffnoremMutate(tbl, mutation == 1); n == 0 {
			t.Fatal("the mutation found no emblem static to change")
		}
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"keffnorem-a", "keffnorem-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, keffnoremPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			if mutation == 0 {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			st.divs++
			if st.firstDiv == 0 {
				st.firstDiv = seed
			}
			if mutation == 0 {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return st
}

func TestEffnoremLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runEffnorem(t, 256, 0)
	t.Logf("%d games, %d divergences, %d stalls, Defender-carrier attacks %v, Maskmaker attacks %d, face-down options %d, casts %d, of them in a live-Maskmaker turn %d, mana paid by face-down casts %v",
		st.games, st.divs, st.stalls, st.attacks, st.maskAtk, st.faceOffered, st.castFace, st.discounted, st.paid)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	for n := range keffnoremDefenders {
		if st.attacks[n] < 3 {
			t.Errorf("only %d attacks by %s", st.attacks[n], n)
		}
	}
	if st.discounted < 3 {
		t.Errorf("only %d face-down casts in a live-Maskmaker turn", st.discounted)
	}
}

// TestEffnoremMutation: the CanAttackDefender statics, dropped from every
// built emblem, must make the games diverge within 64 seeds (first seed 1).
//
// The ReduceCost half is a pin, not a divergence: gorge prices a face-down
// cast while the card is still in hand (rules/statics_costapply.go matchesSpec
// of ValidCard$ Card.faceDown against the hand object), so Goblin Maskmaker's
// discount never applies and a face-down cast pays the full {3} on both
// sides. Zeroing the emblem amount therefore cannot diverge; the test pins
// that gorge still charges {3} in every live-Maskmaker cast, so a gorge that
// starts applying the discount turns the lockstep red instead of silently
// disagreeing with the kernel.
func TestEffnoremMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runEffnorem(t, 64, 1)
	if st.divs == 0 {
		t.Errorf("CanAttackDefender statics dropped: no divergence in %d games", st.games)
	} else {
		t.Logf("CanAttackDefender statics dropped: diverged in %d of %d games (first seed %d)", st.divs, st.games, st.firstDiv)
	}
	st = runEffnorem(t, 64, 2)
	t.Logf("ReduceCost amounts zeroed: %d divergences in %d games; mana paid by live-Maskmaker face-down casts %v", st.divs, st.games, st.paid)
	if st.divs != 0 {
		t.Errorf("ReduceCost amounts zeroed diverged in %d games: gorge now discounts a face-down cast (see the test comment)", st.divs)
	}
	if len(st.paid) != 1 || st.paid[3] < 3 {
		t.Errorf("live-Maskmaker face-down casts paid %v, want only 3 (at least 3 casts)", st.paid)
	}
}
