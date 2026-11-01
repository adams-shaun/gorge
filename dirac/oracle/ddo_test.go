package oracle

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated ddo
// pool. Mode$ DamageDoneOnce lines of every referent shape -- enrage on the
// source itself (Feldon, Innocent Bystander with a DamageAmount$ gate,
// Venus, Hexhaven Invigorator, Strength of Will's granted line), an aura's
// enchanted creature (Sleep Magic), players (The Thing, Professional
// Face-Breaker), no ValidTarget$ (Spinneret and Spiderling) -- beside the
// per-event DamageDone recipient filters the kernel used to ignore (Nested
// Ghoul, Giant's Skewer). Seismic Rupture, Shock, Burst Lightning and
// Fanatical Firebrand deal the noncombat damage; the attack-and-block-eager
// policy builds double blocks, so one combat damage step hits one referent
// from several sources.
//
// The counters read gorge's event log: Damage events (Obj the damaged
// permanent, Player the damaged player) grouped into windows that end at
// the next priority, step, stack or turn boundary, and the TriggerPush
// events of the DamageDoneOnce lines inside the window.
var kddoDecks = [][2][]cardset.Entry{{
	{
		{Name: "Feldon, Ronom Excavator", Count: 1}, {Name: "Innocent Bystander", Count: 3},
		{Name: "Spinneret and Spiderling", Count: 3}, {Name: "Venus, Torn Between Worlds", Count: 2},
		{Name: "Hexhaven Invigorator", Count: 2}, {Name: "The Thing, Ben Grimm", Count: 2},
		{Name: "Professional Face-Breaker", Count: 2}, {Name: "Strength of Will", Count: 3},
		{Name: "Seismic Rupture", Count: 2}, {Name: "Fanatical Firebrand", Count: 2},
		{Name: "Sun Droplet", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Sleep Magic", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Nested Ghoul", Count: 3},
		{Name: "Giant's Skewer", Count: 2}, {Name: "Fanatical Firebrand", Count: 2},
		{Name: "Burst Lightning", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Seismic Rupture", Count: 2}, {Name: "Gray Ogre", Count: 3},
		{Name: "Hill Giant", Count: 2}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Farsight Mask", Count: 2}, {Name: "No Mercy", Count: 2}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 3}, {Name: "Mountain", Count: 4},
	},
}}

// kddoPolicy declares every attacker and blocker the options offer (so a
// double block is common), else plays spells and lands.
func kddoPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	var lastTgt uint32
	haveTgt := false
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		blocks := false
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActAttack || k == ActBlock {
				buf = append(buf, a)
				blocks = blocks || k == ActBlock
			}
		}
		if !blocks {
			haveTgt = false
		}
		if haveTgt { // pile the next blocker onto the attacker the last one blocked
			for _, a := range buf {
				if a.Kind() == ActBlock && uint32(a.Tgt()) == lastTgt {
					return a
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		a := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		if a.Kind() == ActBlock {
			lastTgt, haveTgt = uint32(a.Tgt()), true
		}
		return a
	})
}

type kddoKey int32

type kddoRec struct {
	n   int   // Damage events to the referent in the window
	tot int32 // their total
}

type kddoPush struct {
	card string
	obj  state.ObjID
	once bool   // a DamageDoneOnce line
	vt   string // its ValidTarget$
}

// kddoTally counts the arms from gorge's log.
type kddoTally struct {
	maxLatch int
	evAt     int
	step     state.Step
	win      map[kddoKey]*kddoRec
	order    []kddoKey
	pushes   []kddoPush
	// counters
	objCombat, objNonCombat, coalesced, multiRef, gatePass, gateFail int
	playerRef, auraRef, f2match, ghoulMiss, ncRepeat                 int
	toYouOnce, toYouEvent                                            int
	windows, ddoPushes, grantedPush                                  int
	dump                                                             string
	tail                                                             []string
	sow                                                              []string
	ref                                                              *GorgeSide
}

func kddoMode(g *state.Game, id state.ObjID, idx int32) (string, *cards.Trigger) {
	o := g.Obj(id)
	if o == nil || o.Face() == nil || idx < 0 || int(idx) >= len(o.Face().Triggers) {
		return "", nil
	}
	return o.Face().Name, &o.Face().Triggers[idx]
}

func (p *kddoTally) flush(g *state.Game) {
	if len(p.win) == 0 && len(p.pushes) == 0 {
		return
	}
	combat := p.step == state.StepCombatDamage
	nDDO, ghoul := 0, false
	for k, r := range p.win {
		if k > 0 {
			o := g.Obj(state.ObjID(k))
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Nested Ghoul":
				ghoul = true
			case "Innocent Bystander":
				if r.tot < 3 {
					pushed := false
					for _, ps := range p.pushes {
						pushed = pushed || ps.obj == state.ObjID(k) && ps.once
					}
					if !pushed {
						p.gateFail++
					}
				}
			}
		}
		if !combat && r.n >= 2 {
			p.ncRepeat++
		}
	}
	if ghoul {
		for k := range p.win {
			if o := g.Obj(state.ObjID(k)); k > 0 && o != nil && o.Face() != nil && o.Face().Name != "Nested Ghoul" && o.Face().IsCreature() {
				p.ghoulMiss++
			}
		}
	}
	for _, ps := range p.pushes {
		if !ps.once {
			if ps.card == "Nested Ghoul" || ps.card == "Giant's Skewer" {
				p.f2match++
			}
			if ps.vt == "You" {
				p.toYouEvent++
			}
			continue
		}
		nDDO++
		p.ddoPushes++
		var rec *kddoRec
		switch ps.vt {
		case "Card.Self":
			rec = p.win[kddoKey(ps.obj)]
			if combat {
				p.objCombat++
			} else {
				p.objNonCombat++
			}
		case "Card.AttachedBy":
			p.auraRef++
			if o := g.Obj(ps.obj); o != nil {
				rec = p.win[kddoKey(o.AttachedTo)]
			}
		case "Player":
			p.playerRef++
		case "You":
			p.toYouOnce++
		}
		if rec != nil && rec.n >= 2 {
			p.coalesced++
		}
		if ps.card == "Innocent Bystander" || ps.card == "Spinneret and Spiderling" {
			p.gatePass++
		}
	}
	if !combat && nDDO >= 2 {
		p.multiRef++
	}
	p.windows++
	p.win = map[kddoKey]*kddoRec{}
	p.pushes = p.pushes[:0]
}

func (p *kddoTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	p.ref = ref
	if len(evs) < p.evAt {
		p.evAt = 0
		p.sow = nil
		p.win = map[kddoKey]*kddoRec{}
	}
	if p.win == nil {
		p.win = map[kddoKey]*kddoRec{}
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Damage:
			if e.Amount <= 0 {
				continue
			}
			k := kddoKey(-1 - int32(e.Player))
			if e.Obj != 0 {
				k = kddoKey(e.Obj)
			}
			r := p.win[k]
			if r == nil {
				r = &kddoRec{}
				p.win[k] = r
				p.order = append(p.order, k)
			}
			r.n++
			r.tot += e.Amount
		case events.TriggerPush:
			name, tr := kddoMode(g, e.Obj, e.Amount)
			if tr != nil && (tr.Mode == "DamageDoneOnce" || tr.Mode == "DamageDone") {
				p.pushes = append(p.pushes, kddoPush{name, e.Obj, tr.Mode == "DamageDoneOnce", tr.Params["ValidTarget"]})
			}
		case events.GrantTriggerPush:
			p.grantedPush++
		case events.StepChange:
			p.flush(g)
			p.step = e.Step
		case events.Priority, events.TurnChange, events.PutOnStack, events.Resolve, events.DeclareAttackers, events.DeclareBlockers:
			p.flush(g)
		}
		if os.Getenv("KDDO_DEBUG") != "" {
			nm := ""
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil {
				nm = o.Face().Name
			}
			if nm == "Strength of Will" || e.Kind == events.TargetsChosen || e.Kind == events.TurnChange {
				p.sow = append(p.sow, fmt.Sprintf("%v obj=%d(%s) pl=%d amt=%d ids=%v pairs=%v", e.Kind, e.Obj, nm, e.Player, e.Amount, e.IDs, e.Pairs))
			}
			p.tail = append(p.tail, fmt.Sprintf("%v obj=%d(%s) pl=%d amt=%d step=%d from=%d to=%d txt=%q ids=%v", e.Kind, e.Obj, nm, e.Player, e.Amount, e.Step, e.From, e.To, e.Text, e.IDs))
			if len(p.tail) > 70 {
				p.tail = p.tail[1:]
			}
		}
	}
	p.evAt = len(evs)
	if os.Getenv("KDDO_DEBUG") != "" {
		p.dump = ""
		for id := state.ObjID(1); int(id) <= len(g.Objs); id++ {
			if o := g.Obj(id); o != nil && o.Zone == state.ZBattlefield {
				p.dump += fmt.Sprintf("%d:%s t=%v sick=%v ctl=%d dmg=%d; ", id, o.Face().Name, o.Tapped, o.SummonSick, o.Controller, o.Damage)
			}
		}
	}
	return nil
}

func kddoSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ddo")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(kddoDecks))
	tbls := make([]*kop.Table, len(kddoDecks))
	for mi, md := range kddoDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

func kddoRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"ddo-a", "ddo-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := kddoPolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 60, Check: check})
	CloseSide(k)
	return res
}

var _ = dirac.StallRefLatch

func kddoGames(t *testing.T, p *kddoTally, stopAtDiv bool) (games, div int) {
	dirac.DebugRefLatchN = func(n int) {
		if n > p.maxLatch {
			p.maxLatch = n
		}
	}
	defer func() { dirac.DebugRefLatchN = nil }()
	reg, set, decks, tbls := kddoSetup(t)
	for mi := range kddoDecks {
		for seed := uint64(1); seed <= 480; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			res := kddoRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, p.check)
			games++
			if res.Stall != "" && !stopAtDiv {
				// two Sleep Magic can lock a board for the turn cap, and a long
				// game can fill the remember pool: capacity ends, not divergences
				if res.Stall != "turns" && res.Stall != KernelStallPrefix+"remember" {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				} else {
					t.Logf("matchup %d seed %d: capacity stall: %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				div++
				if stopAtDiv {
					return
				}
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				if os.Getenv("KDDO_DEBUG") != "" {
					t.Logf("board: %s", p.dump)
					for _, l := range p.sow {
						t.Log("SOW ", l)
					}
					for _, l := range p.tail[max(0, len(p.tail)-8):] {
						t.Log(l)
					}
					g := p.ref.E.G
					for _, e := range p.ref.E.L.Events[p.evAt:] {
						nm := ""
						if o := g.Obj(e.Obj); o != nil && o.Face() != nil {
							nm = o.Face().Name
						}
						t.Logf("NEW %v obj=%d(%s) pl=%d amt=%d from=%d to=%d txt=%q ids=%v", e.Kind, e.Obj, nm, e.Player, e.Amount, e.From, e.To, e.Text, e.IDs)
					}
				}
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestDdoLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	var p kddoTally
	games, div := kddoGames(t, &p, false)
	t.Logf("%d games, %d diverged: %d object-referent combat, %d noncombat, %d coalesced, %d multi-referent ops, %d gate passed, %d gate failed, %d player, %d aura, %d granted, %d filter2 matched, %d filter2 missed; noncombat repeats %d (windows %d, ddo pushes %d); Tf2ToYou: %d once, %d per-event; max refLatch %d",
		games, div, p.objCombat, p.objNonCombat, p.coalesced, p.multiRef, p.gatePass, p.gateFail, p.playerRef, p.auraRef, p.grantedPush, p.f2match, p.ghoulMiss, p.ncRepeat, p.windows, p.ddoPushes, p.toYouOnce, p.toYouEvent, p.maxLatch)
	for _, c := range []struct {
		name string
		n    int
	}{{"object-referent combat", p.objCombat}, {"object-referent noncombat", p.objNonCombat}, {"coalesced", p.coalesced},
		{"one op, several referents", p.multiRef}, {"gate passed", p.gatePass}, {"gate failed", p.gateFail},
		{"player referent", p.playerRef}, {"aura referent", p.auraRef}, {"granted", p.grantedPush},
		{"to-you once (Sun Droplet)", p.toYouOnce}, {"to-you per event (Farsight Mask)", p.toYouEvent}, {"filter2 matched", p.f2match}, {"filter2 not matched", p.ghoulMiss}} {
		if c.n < 10 {
			t.Errorf("%s: %d, want at least 10", c.name, c.n)
		}
	}
	if p.ncRepeat != 0 {
		t.Errorf("%d noncombat batches with two events to one referent, want 0", p.ncRepeat)
	}
}

// TestDdoMutation: each debug switch must make the lockstep diverge, so the
// counters above are not vacuous.
func TestDdoMutation(t *testing.T) {
	for _, m := range []struct {
		name string
		flag *bool
	}{{"no latch", &dirac.DebugDdoNoLatch}, {"no gate", &dirac.DebugDdoNoGate}, {"ignore filter2", &dirac.DebugDmgIgnoreFilter2}} {
		*m.flag = true
		var p kddoTally
		games, div := kddoGames(t, &p, true)
		*m.flag = false
		t.Logf("mutation %q: diverged=%d after %d games", m.name, div, games)
		if div == 0 {
			t.Errorf("mutation %q did not diverge", m.name)
		}
	}
}
