package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The TriggeredAttacker single-binding lockstep: Bone Sabres ("Whenever
// equipped creature attacks, put four +1/+1 counters on it") is the pool's
// only Equipment. Its ValidCard$ Creature.EquippedBy filter names exactly one
// attacker by construction, so an attack with the equipped creature plus any
// other creature is gorge's exactly-one-match case. gorge binds the
// singular TriggeredAttacker spellings to TriggerCard -- the one attacker that
// matched -- and keeps the Remembered read only as the role-absent fallback;
// before that commit Defined$ TriggeredAttackerLKICopy read the whole batched
// DeclareAttackers Remembered list, so every attacker was pumped. The kernel
// fires the Attacks trigger per matching attacker with that attacker as the
// trigger object, which is the new binding. This test therefore diverges at
// b991c88aa and passes at a2df098cc.
//
// The broad filter (Kragma Warcaller's Minotaur.YouCtrl) is in the pool since
// gorge (5f827510c): TestAttacksPerAttackerLockstep below.
var triggerAttackerDecks = [][2][]cardset.Entry{{
	{
		{Name: "Bone Sabres", Count: 1}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Centaur Courser", Count: 4}, {Name: "Forest", Count: 8},
		{Name: "Mountain", Count: 7},
	},
	{
		{Name: "Bone Sabres", Count: 1}, {Name: "Centaur Courser", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Goblin Piker", Count: 4}, {Name: "Forest", Count: 9},
		{Name: "Mountain", Count: 6},
	},
}}

// triggerAttackerPolicy casts Bone Sabres whenever it can and equips it when
// it is unattached (so the carrier reaches the battlefield and one attacker
// carries the trigger), otherwise casts, activates, plays lands and attacks
// when it can. It counts the trigger's four +1/+1 counters by watching the
// gorge side's derived power for every battlefield permanent: each +4 rise is
// one resolved trigger (the pool has no other pump).
func triggerAttackerPolicy(seed uint64, played map[string]int, pumps *int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	last := map[state.ObjID]int32{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
					pw := c.G.E.Power(id)
					if old, ok := last[id]; ok && pw > old {
						*pumps += int((pw - old) / 4)
					}
					last[id] = pw
				}
			}
		}
		buf = buf[:0]
		var sabreCast, sabreEquip Canon
		haveCast, haveEquip := false, false
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand, ActAttack:
				buf = append(buf, a)
			}
			if a.Kind() != ActCast && a.Kind() != ActActivate {
				continue
			}
			if c.G == nil {
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := c.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil || ob.Face().Name != "Bone Sabres" {
				continue
			}
			switch a.Kind() {
			case ActCast:
				sabreCast, haveCast = a, true
			case ActActivate:
				if ob.AttachedTo == 0 {
					sabreEquip, haveEquip = a, true
				}
			}
		}
		var a Canon
		switch {
		case haveCast:
			a = sabreCast
		case haveEquip:
			a = sabreEquip
		case len(buf) == 0:
			for _, o := range c.D.Options {
				if o.Kind() == ActPass || o.Kind() == ActDone {
					return o
				}
			}
			a = rnd.Pick(c)
		default:
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActCast && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

func TestTriggeredAttackerSingleLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("triggerattacker")
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 96
	games := 0
	played := map[string]int{}
	pumps := 0
	for mi, md := range triggerAttackerDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("matchup %d: %s not in the corpus", mi, e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			m, err := NewMatchup(reg, [2]string{"triggerattacker-a", "triggerattacker-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, triggerAttackerPolicy(seed, played, &pumps), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier casts %v, Bone Sabres trigger pumps %d", games, played, pumps)
	if played["Bone Sabres"] < 4 {
		t.Errorf("Bone Sabres cast %d times, want at least 4", played["Bone Sabres"])
	}
	if pumps < 8 {
		t.Errorf("Bone Sabres trigger resolved %d times, want at least 8", pumps)
	}
}

// The Attacks per-attacker lockstep Kragma
// Warcaller's "Whenever a Minotaur you control attacks, it gets +2/+0" has a
// ValidCard$ (Minotaur.YouCtrl) that matches several attackers. gorge queues
// one trigger per matching attacker, each binding TriggerCard to that
// attacker (CR 508.3a, 603.2c), so every attacking Minotaur is pumped once
// and a non-Minotaur attacker is untouched; the kernel fires per matching
// attacker too. Before 5f827510c gorge queued one trigger and pumped the
// whole Remembered batch.
var attacksPerAttackerDecks = [2][]cardset.Entry{
	{
		{Name: "Kragma Warcaller", Count: 4}, {Name: "Borderland Minotaur", Count: 4},
		{Name: "Hurloon Minotaur", Count: 4}, {Name: "Minotaur Warrior", Count: 4},
		{Name: "Pensive Minotaur", Count: 4}, {Name: "Felhide Minotaur", Count: 2},
		{Name: "Gray Ogre", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Kragma Warcaller", Count: 4}, {Name: "Hurloon Minotaur", Count: 4},
		{Name: "Borderland Minotaur", Count: 4}, {Name: "Pensive Minotaur", Count: 4},
		{Name: "Minotaur Warrior", Count: 4}, {Name: "Felhide Minotaur", Count: 2},
		{Name: "Goblin Piker", Count: 2}, {Name: "Raging Goblin", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 5},
	},
}

type attacksPerStats struct {
	multi     int // Minotaurs pumped +2 in a declaration with two or more matching attackers
	declMulti int // such declarations
	untouched int // a non-Minotaur attacker left at its power beside a pumped Minotaur
	pumpedAtk int // every pumped Minotaur
}

func isMinotaurName(n string) bool {
	return n == "Kragma Warcaller" || strings.HasSuffix(n, "Minotaur") || n == "Minotaur Warrior"
}

// attacksPerAttackerPolicy plays cast/activate/land/attack options at random
// and, once a declaration's triggers have resolved (the stack is empty with
// attackers still attacking), compares each attacker's power with the power it
// had when first seen attacking.
func attacksPerAttackerPolicy(seed uint64, st *attacksPerStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	first := map[state.ObjID]int32{}
	done := false
	// The attack subset and its submission order are both random; gorge gets
	// the raw order and the kernel queues the triggers in declaration order
	// (perm.bord), so an order decision's occurrence names the same attacker
	// on both sides.
	rngs := seed*0x9e3779b97f4a7c15 | 1
	var plan []Canon
	planned := false
	ordering := false
	return PolicyFunc(func(c *PickCtx) Canon {
		ordering = false
		for _, o := range c.D.Options {
			if o.Kind() == ActOrder {
				ordering = true
			}
		}
		if c.G != nil && !ordering {
			g := c.G.E.G
			var atk []state.ObjID
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if ob := g.Obj(id); ob != nil && ob.IsAttacking {
						atk = append(atk, id)
					}
				}
			}
			if len(atk) == 0 {
				clear(first)
				done = false
			} else {
				for _, id := range atk {
					if _, ok := first[id]; !ok {
						first[id] = c.G.E.Power(id)
					}
				}
				if !done && len(g.Stack) == 0 {
					done = true
					mino, pumped, plain := 0, 0, 0
					for _, id := range atk {
						ob := g.Obj(id)
						if ob == nil || ob.Face() == nil {
							continue
						}
						d := c.G.E.Power(id) - first[id]
						if isMinotaurName(ob.Face().Name) {
							mino++
							if d == 2 {
								pumped++
							}
						} else if d == 0 {
							plain++
						}
					}
					st.pumpedAtk += pumped
					if mino >= 2 {
						st.multi += pumped
						st.declMulti++
					}
					if pumped > 0 {
						st.untouched += plain
					}
				}
			}
		}
		natk := 0
		var doneOpt Canon
		for _, o := range c.D.Options {
			switch o.Kind() {
			case ActAttack:
				natk++
			case ActDone:
				doneOpt = o
			}
		}
		if natk == 0 {
			planned = false
		} else {
			if !planned {
				planned = true
				plan = plan[:0]
				for _, o := range c.D.Options {
					if o.Kind() != ActAttack {
						continue
					}
					rngs ^= rngs << 13
					rngs ^= rngs >> 7
					rngs ^= rngs << 17
					if rngs&3 != 0 {
						plan = append(plan, o)
					}
				}
			}
			// a random planned attacker that is still offered goes next
			var left []Canon
			for _, o := range c.D.Options {
				if o.Kind() == ActAttack && slices.Contains(plan, o) {
					left = append(left, o)
				}
			}
			if len(left) > 0 {
				rngs ^= rngs << 13
				rngs ^= rngs >> 7
				rngs ^= rngs << 17
				return left[rngs%uint64(len(left))]
			}
			if doneOpt != 0 {
				return doneOpt
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand, ActAttack:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runAttacksPerAttacker plays seeds [1, seeds] in order and returns the
// first divergence ("" when none) with the summed statistics.
func runAttacksPerAttacker(t *testing.T, tbl *kop.Table, seeds int, stopAtDiv bool) (string, attacksPerStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("triggerattacker")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range attacksPerAttackerDecks {
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
	if tbl == nil {
		tbl, _, _, err = set.Matchup(attacksPerAttackerDecks[0], attacksPerAttackerDecks[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	var total attacksPerStats
	div := ""
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"triggerattacker-a", "triggerattacker-b"}, decks, 1)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, attacksPerAttackerPolicy(seed, &total), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			div = strings.TrimSpace(d.What + " " + d.Field)
			if !stopAtDiv {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			break
		}
	}
	return div, total
}

func TestAttacksPerAttackerLockstep(t *testing.T) {
	t.Parallel()

	_, st := runAttacksPerAttacker(t, nil, 1024, false)
	t.Logf("1024 games: %+v", st)
	if st.multi < 10 {
		t.Errorf("Minotaurs pumped in a 2+ matching declaration %d times, want at least 10", st.multi)
	}
	if st.untouched < 10 {
		t.Errorf("non-matching attacker left untouched %d times, want at least 10", st.untouched)
	}
}

// TestAttacksPerAttackerMutation: with Kragma Warcaller's trigger moved to
// the once-per-declaration AttackersDeclared event (one trigger per declare
// event, no per-attacker binding) the same games diverge.
func TestAttacksPerAttackerMutation(t *testing.T) {
	t.Parallel()

	corpusDirOrSkip(t)
	set, err := cardset.RawByName("triggerattacker")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(attacksPerAttackerDecks[0], attacksPerAttackerDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	mt := *tbl
	mt.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	moved := 0
	for i := range mt.Cards {
		ab := append([]kop.Ability(nil), mt.Cards[i].Abilities...)
		for j := range ab {
			if ab[j].Kind == kop.AbTriggered && ab[j].Trig.Event == kop.TrigAttacks && len(ab[j].Prog) > 0 && ab[j].Prog[0].Op == kop.OpPumpUntilEOT {
				ab[j].Trig.Event = kop.TrigAttackersDeclared
				moved++
			}
		}
		mt.Cards[i].Abilities = ab
	}
	if moved == 0 {
		t.Fatal("no Attacks pump trigger in the table")
	}
	div, _ := runAttacksPerAttacker(t, &mt, 64, true)
	if div == "" {
		t.Fatal("one trigger per declare event produced no divergence")
	}
	t.Logf("mutation diverged: %s", div)
}
