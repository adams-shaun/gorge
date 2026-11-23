package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Oathsworn Knight (must attack, enters with four +1/+1 counters, prevents
// damage dealt to it and removes a counter instead: kgen
// `repl PreventDamageCounters`) attacks into token blockers, several at once,
// so combat damage reaches it from more than one blocker and the attacker's
// division ask is posed (tail-73 pair 14 parted on the Knight's counters after
// such a choose:division). The opponent makes the tokens (Dragon Fodder,
// Hordeling Outburst, Goblin Instigator) and the policy blocks the Knight with
// them, a second blocker joining the first.
var prevRemoveCtrBlockerDecks = [2][]cardset.Entry{
	{
		{Name: "Oathsworn Knight", Count: 10}, {Name: "Undergrowth Champion", Count: 3}, {Name: "Ugin's Conjurant", Count: 3},
		{Name: "Forest", Count: 12}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Dragon Fodder", Count: 6}, {Name: "Hordeling Outburst", Count: 6}, {Name: "Goblin Instigator", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 18},
	},
}

type prevRemoveCtrBlockerStats struct {
	games, divs, stalls int
	tokenBlocks         int // a token declared as a blocker of the Knight
	doubled             int // a second token declared on the Knight a token already blocks
	divAsks             int // a division ask while a token blocks the Knight
	drops               int // the Knight lost a +1/+1 counter (observed in gorge between two decisions)
	seedsReaching       int // games with a token block and a division ask
}

// prevRemoveCtrBlockerPolicy plays lands and casts and activates first,
// attacks with everything, and blocks the Knight with a token, a second
// blocker joining the attacker the previous pick blocked.
func prevRemoveCtrBlockerPolicy(seed uint64, st *prevRemoveCtrBlockerStats) Policy {
	rnd := RandomPolicy(seed)
	division, _ := chooseCode("division")
	lastAtk := state.ObjID(0)
	counters := map[state.ObjID]int32{}
	tokBlock, divAsk := false, false
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.Seq == 0 {
			lastAtk, tokBlock, divAsk = 0, false, false
			clear(counters)
		}
		if c.G != nil {
			g := c.G.E.G
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					ob := g.Obj(id)
					if ob == nil || ob.Face() == nil || ob.Face().Name != "Oathsworn Knight" {
						continue
					}
					n := ob.Counter("P1P1")
					if prev, ok := counters[id]; ok && n < prev {
						st.drops++
					}
					counters[id] = n
				}
			}
		}
		if c.D.Kind == DAttackers {
			lastAtk = 0
			for _, o := range c.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		if c.D.Kind == DBlockers && c.G != nil {
			join, tok := -1, -1
			for i, o := range c.D.Options {
				if o.Kind() != ActBlock {
					continue
				}
				go_, ok := c.G.GorgeOption(o)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(go_.Attacker)
				bl := c.G.E.G.Obj(go_.Obj)
				if ob == nil || ob.Face() == nil || ob.Face().Name != "Oathsworn Knight" || bl == nil || !bl.IsToken {
					continue
				}
				if tok < 0 {
					tok = i
				}
				if lastAtk != 0 && go_.Attacker == lastAtk && join < 0 {
					join = i
				}
			}
			pick := join
			if pick < 0 {
				pick = tok
			}
			if pick >= 0 {
				o := c.D.Options[pick]
				go_, _ := c.G.GorgeOption(o)
				st.tokenBlocks++
				tokBlock = true
				if pick == join {
					st.doubled++
				}
				lastAtk = go_.Attacker
				return o
			}
		}
		if c.D.Kind == DChoose && len(c.D.Options) > 0 && c.D.Options[0].Kind() == ActChoose+CanonKind(division) && tokBlock {
			st.divAsks++
			if !divAsk {
				divAsk = true
				st.seedsReaching++
			}
		}
		for _, o := range c.D.Options {
			if k := o.Kind(); k == ActPlayLand || k == ActCast || k == ActActivate {
				return o
			}
		}
		return rnd.Pick(c)
	})
}

// runPrevRemoveCtrBlockers plays seeds of the blockers matchup; mutate drops
// the +1/+1 counter gate of the kernel's damage prevention.
func runPrevRemoveCtrBlockers(t *testing.T, seeds int, mutate bool) *prevRemoveCtrBlockerStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("prevremovectr")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range prevRemoveCtrBlockerDecks {
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
	tbl, _, _, err := set.Matchup(prevRemoveCtrBlockerDecks[0], prevRemoveCtrBlockerDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		cs := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cs {
			abs := append([]kop.Ability(nil), cs[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbReplacement && abs[j].Repl.Kind == kop.ReplPreventDamageCounters {
					abs[j].Repl.C &^= kop.ReplPrevSelfIfCounter
				}
			}
			cs[i].Abilities = abs
		}
		tbl.Cards = cs
	}
	st := &prevRemoveCtrBlockerStats{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"prc-a", "prc-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, prevRemoveCtrBlockerPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 24})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			st.divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return st
}

// TestDamageDoneReplShapeBlockersLockstep: Oathsworn Knight blocked by tokens,
// several at once, loses the same +1/+1 counters in both engines. Its
// precondition: some seed has a token block on the Knight, a division ask after
// it, and a counter drop.
func TestDamageDoneReplShapeBlockersLockstep(t *testing.T) {
	t.Parallel()
	st := runPrevRemoveCtrBlockers(t, 120, false)
	t.Logf("%+v", *st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.tokenBlocks == 0 || st.doubled == 0 || st.seedsReaching == 0 || st.drops == 0 {
		t.Fatalf("the situation was not reached: %+v", *st)
	}
}

// TestDamageDoneReplShapeBlockersLockstepMutation: with the counter gate
// dropped in the kernel, a game with the Knight blocked by tokens diverges.
func TestDamageDoneReplShapeBlockersLockstepMutation(t *testing.T) {
	t.Parallel()
	st := runPrevRemoveCtrBlockers(t, 120, true)
	t.Logf("%+v", *st)
	if st.divs == 0 {
		t.Fatalf("no seed diverged with the +1/+1 counter gate dropped: %+v", *st)
	}
}
