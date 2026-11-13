package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// IsSingleTarget$ True (Leyline of Resonance), DamageDone's
// ValidCause$ Spell.numTargets EQ1 + TargetRelativeToCause$
// Creature.targetedBy (Imodane, the Pyrohammer) and Sacrificed's inert
// Origin$/Destination$ (The Sackville-Bagginses).
//
// The matchups pair the carriers with the triggers they need: red burn and
// pumps for Leyline/Imodane (Shock, Lightning Bolt, Giant Growth, Twin Bolt,
// Pyroclasm), and the black token/sacrifice engine for Sackville (Dragon
// Fodder, Carrion Feeder, Viscera Seer, Bone Splinters).

var ktrigp4Decks = [][2][]cardset.Entry{
	{ // M1: Leyline + Imodane
		{
			{Name: "Leyline of Resonance", Count: 4}, {Name: "Imodane, the Pyrohammer", Count: 4},
			{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Giant Growth", Count: 1},
			{Name: "Twin Bolt", Count: 2}, {Name: "Pyroclasm", Count: 1},
			{Name: "Arc Trail", Count: 3}, {Name: "Radiating Lightning", Count: 4},
			{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4},
			{Name: "Child of Night", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 4},
		},
	},
	{ // M2: Sackville
		{
			{Name: "The Sackville-Bagginses", Count: 4}, {Name: "Dragon Fodder", Count: 4},
			{Name: "Carrion Feeder", Count: 4}, {Name: "Viscera Seer", Count: 4}, {Name: "Bone Splinters", Count: 4},
			{Name: "Child of Night", Count: 4}, {Name: "Grizzly Bears", Count: 2},
			{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4},
			{Name: "Child of Night", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 4},
		},
	},
}

// ktrigp4Stats is what the gorge-side event log says each qualifier decided.
type ktrigp4Stats struct {
	arcCasts  int // Arc Trail reached the stack (the chained-spell carrier)
	radCasts  int // Radiating Lightning reached the stack (the one-target-not-recipient carrier)
	radImo    int // a resolving Radiating Lightning damaged a creature with Imodane out
	leyCopies int // a StackCopy of an instant or sorcery (Leyline triggered)
	leyNo     int // a resolving instant/sorcery had a non-single ROOT target list while Leyline was out
	leyChain  int // a chained spell resolved with a single ROOT target and a SubAbility$ link target while Leyline was out: the TgfDeclSub exclusion decided the cast check
	imoTrig   int // Imodane's DamageDone trigger was put on the stack
	imoNo     int // a resolving spell damaged a creature it did not single-target (Imodane stayed silent)
	imoHitNo  int // that spell had exactly ONE target, a different creature: the TargetRelativeToCause$ hit test decided it
	sackDrain int // a resolving Sackville Sacrificed trigger made an opponent lose life
}

// ktrigp4Cast is one stack object's target list reconstructed from the
// TargetsChosen events as they were emitted. Reading obj.Targets later is
// wrong: the object has left the stack by the time the deferred tally scans,
// and gorge clears Targets/SubTargets on the zone change (apply_zone.go).
type ktrigp4Cast struct {
	ctrl    state.PlayerID
	rootN   int           // entries in gorge's obj.Targets (the root declarations)
	subN    int           // entries in obj.SubTargets (the SubAbility$ links)
	objIDs  []state.ObjID // every object target, root and chain, for TargetRelativeToCause$
	copy    bool
	instant bool
}

func (c *ktrigp4Cast) add(e *events.Event, sub bool) {
	n := len(e.IDs)
	if e.Amount == 1 || e.Amount == 3 { // a player target
		n = 1
	}
	if sub {
		c.subN += n
	} else {
		c.rootN += n
	}
	c.objIDs = append(c.objIDs, e.IDs...)
}

type ktrigp4Tally struct {
	evAt      int
	resolving state.ObjID // the stack object being resolved
	sackTrig  state.ObjID // the Sackville trigger being resolved
	casts     map[state.ObjID]*ktrigp4Cast
}

func ktrigp4Controls(g *state.Game, pl state.PlayerID, name string) bool {
	for _, id := range g.Zone(state.ZBattlefield, pl) {
		if chName(g, id) == name {
			return true
		}
	}
	return false
}

func (p *ktrigp4Tally) scan(ref *GorgeSide, st *ktrigp4Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt, p.resolving, p.sackTrig = 0, 0, 0
		p.casts = nil
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.PutOnStack:
			switch chName(g, e.Obj) {
			case "Arc Trail":
				st.arcCasts++
			case "Radiating Lightning":
				st.radCasts++
			}
		case events.TargetsChosen:
			c := p.casts[e.Obj]
			if c == nil {
				if p.casts == nil {
					p.casts = make(map[state.ObjID]*ktrigp4Cast)
				}
				c = &ktrigp4Cast{}
				if o := g.Obj(e.Obj); o != nil {
					c.ctrl, c.copy = o.Controller, o.IsCopy
					c.instant = o.Face() != nil && (o.Face().IsInstant() || o.Face().IsSorcery())
				}
				p.casts[e.Obj] = c
			}
			c.add(e, e.Text == events.SubTargetNotice)
		case events.StackCopy:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && (o.Face().IsInstant() || o.Face().IsSorcery()) {
				st.leyCopies++
			}
		case events.TriggerPush:
			if chName(g, e.Obj) == "Imodane, the Pyrohammer" {
				st.imoTrig++
			}
		case events.Resolve:
			p.resolving, p.sackTrig = e.Obj, 0
			// The reconstructed list is exactly what Leyline's TargetsValid$ and
			// IsSingleTarget$ read at cast (gorge's obj.Targets; a copy is not a
			// cast and is skipped) and what the cause qualifier counts (Targets
			// plus SubTargets).
			if c := p.casts[e.Obj]; c != nil && c.instant && !c.copy &&
				ktrigp4Controls(g, c.ctrl, "Leyline of Resonance") {
				if c.rootN != 1 {
					st.leyNo++
				}
				if c.rootN == 1 && c.subN > 0 {
					// A chained spell's root list is single while its link's
					// declaration rides SubTargets: exactly the castTargetsAll
					// slot the TgfDeclSub exclusion must skip.
					st.leyChain++
				}
			}
			if o := g.Obj(e.Obj); o != nil && o.Source != 0 && chName(g, o.Source) == "The Sackville-Bagginses" {
				p.sackTrig = e.Obj
			}
		case events.Damage:
			if p.resolving == 0 || e.Obj == 0 {
				break
			}
			c := p.casts[p.resolving]
			if c == nil || !c.instant {
				break
			}
			if !ktrigp4Controls(g, c.ctrl, "Imodane, the Pyrohammer") {
				break
			}
			d := g.Obj(e.Obj)
			if d == nil || d.Face() == nil || !d.Face().IsCreature() {
				break
			}
			if chName(g, p.resolving) == "Radiating Lightning" {
				st.radImo++
			}
			n := c.rootN + c.subN
			hit := false
			for _, id := range c.objIDs {
				hit = hit || id == e.Obj
			}
			if n != 1 || !hit {
				st.imoNo++
			}
			if n == 1 && !hit {
				st.imoHitNo++
			}
		case events.LifeChange:
			if p.sackTrig == 0 || e.Amount >= 0 {
				break
			}
			if so := g.Obj(p.sackTrig); so != nil && so.Source != 0 && chName(g, so.Source) == "The Sackville-Bagginses" {
				st.sackDrain++
			}
		case events.Priority, events.StepChange:
			p.resolving, p.sackTrig = 0, 0
		case events.TurnChange:
			p.resolving, p.sackTrig, p.casts = 0, 0, nil
		}
	}
	p.evAt = len(evs)
}

// ktrigp4Policy plays at random among casting, activating and playing lands,
// but leans toward casts (so the burn and pumps are used) and toward targeting
// a creature its controller owns (Leyline's "only a single creature you
// control" and Giant Growth).
func ktrigp4Policy(seed uint64, st *ktrigp4Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6b7472696770)) // "ktrigp"
	var buf []Canon
	var tl ktrigp4Tally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		switch c.D.Kind {
		case DPriority:
			if c.G != nil {
				buf = buf[:0]
				for _, a := range c.D.Options {
					if a.Kind() == ActCast {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 && prng.IntN(4) != 0 {
					return buf[prng.IntN(len(buf))]
				}
			}
		case DTarget:
			if c.G != nil {
				buf = buf[:0]
				for _, a := range c.D.Options {
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Obj == 0 {
						continue
					}
					ob := c.G.E.G.Obj(o.Obj)
					if ob != nil && ob.Face() != nil && ob.Face().IsCreature() && ob.Controller == state.PlayerID(c.D.Player) {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 {
					return buf[prng.IntN(len(buf))]
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runTrigsingletarget plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the qualifier counts. mutate
// inverts both kernel checks (the mutation must diverge).
func runTrigsingletarget(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st ktrigp4Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigsingletarget")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugTrigParams4Flip
	dirac.DebugTrigParams4Flip = mutate
	defer func() { dirac.DebugTrigParams4Flip = old }()
	for mi, md0 := range ktrigp4Decks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
			}
			var decks [2][]*cards.Card
			for s, d := range md {
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
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"ktrigp4-a", "ktrigp4-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, ktrigp4Policy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					divs++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestSingleTargetTriggerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 48 seeds per seating per matchup: 192 games
	games, divs, stalls, st := runTrigsingletarget(t, 48, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Arc Trail casts", st.arcCasts}, {"Radiating Lightning casts", st.radCasts},
		{"Leyline copies", st.leyCopies}, {"Leyline non-single casts", st.leyNo},
		{"Leyline chained-spell exclusions", st.leyChain},
		{"Imodane triggers", st.imoTrig}, {"Imodane non-triggers", st.imoNo},
		{"Imodane one-target-not-recipient", st.imoHitNo},
		{"Sackville drains", st.sackDrain},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestSingleTargetTriggerMutation inverts both single-target checks: the games must
// diverge. A green mutation means the lockstep never reaches a cast or a
// damage event the qualifiers shape.
func TestSingleTargetTriggerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runTrigsingletarget(t, 16, true)
	if divs == 0 {
		t.Fatalf("mutation inverted both single-target checks in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
