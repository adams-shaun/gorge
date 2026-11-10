package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: three Defined$ heads.
//
//   - Defined$ OppNonRememberedController: Momentum Breaker (the opponent
//     sacrifices a creature or Vehicle, then discards unless they did) and
//     Entropic Battlecruiser's granted Attacks trigger (discard, then lose 3
//     life unless they discarded);
//   - Defined$ CardOwner: Gandalf shuffles himself into his library, then his
//     owner draws three (gorge keeps the ObjID across the move);
//   - Defined$ You & Valid Permanent.YouCtrl: Dawn's Truce gives hexproof to
//     the controller and each permanent they control.
var kdefheads6Decks = [2][]cardset.Entry{
	{
		{Name: "Momentum Breaker", Count: 4}, {Name: "Gandalf, Wandering Wizard", Count: 3}, {Name: "Dawn's Truce", Count: 3},
		{Name: "Savannah Lions", Count: 6}, {Name: "Smuggler's Copter", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Control Magic", Count: 1}, {Name: "Plains", Count: 7}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Entropic Battlecruiser", Count: 5}, {Name: "Hill Giant", Count: 10}, {Name: "Mind Rot", Count: 2},
		{Name: "Duress", Count: 1}, {Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 1},
		{Name: "Smuggler's Copter", Count: 1}, {Name: "Momentum Breaker", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

// kd6Tally counts, from gorge's event log, what the pool reached.
type kd6Tally struct {
	evAt    int
	seen    bool
	lastSeq uint64
	stack   map[uint32]string // stack object -> the spell's name
	// the trigger or ability being resolved (0 none, 1 Momentum Breaker, 2
	// Battlecruiser, 3 Gandalf), whether its opponent could not sacrifice /
	// has no card in hand, and what its window saw
	pend      int
	resolved  bool // the pending object's Resolve was seen: its effects follow it in the log
	cant      bool
	hand      bool // the opponent holds a card when the trigger is put on the stack
	opp       state.PlayerID
	owner     state.PlayerID
	sawDisc   bool
	sawLoss   bool
	sawDraws  int
	breakCant int // Momentum Breaker discards by an opponent who sacrificed nothing
	breakSac  int // Momentum Breaker resolutions where the opponent sacrificed
	breakNone int // Momentum Breaker resolutions where the opponent did neither
	bcCant    int // Battlecruiser life losses by an opponent with an empty hand
	bcDisc    int // Battlecruiser discards (no life loss)
	bcTrig    int
	bcCast    int
	maxCharge int // the most charge counters a Battlecruiser held
	gandalf   int // Gandalf activations that drew three
	truce     int // Dawn's Truce resolutions
}

func (p *kd6Tally) reset() {
	p.evAt, p.seen = 0, false
	p.stack = map[uint32]string{}
	p.pend, p.resolved = 0, false
}

// kd6OppBoard reports whether player p controls a creature or Vehicle.
func kd6OppBoard(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		ob := g.Obj(id)
		if ob == nil || ob.Face() == nil || ob.Controller != p {
			continue
		}
		for _, t := range ob.Face().Types {
			if t == "Creature" || t == "Vehicle" {
				return true
			}
		}
	}
	return false
}

func (p *kd6Tally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if p.seen && e.Seq <= p.lastSeq {
			continue // never count an event twice
		}
		p.seen, p.lastSeq = true, e.Seq
		switch e.Kind {
		case events.DeclareAttackers:
			// gorge pushes the granted Attacks trigger on a path of its own
			// (no TriggerPush); the declaration is where the opponent's hand
			// is read, the trigger's effects follow its Resolve
			for _, id := range e.IDs {
				if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Entropic Battlecruiser" && ob.Counter("CHARGE") >= 8 {
					p.pend, p.opp, p.resolved = 2, 1-ob.Controller, false
					p.cant = len(g.Zone(state.ZHand, p.opp)) == 0
					p.sawDisc, p.sawLoss = false, false
					p.bcTrig++
				}
			}
		case events.PutOnStack:
			p.stack[uint32(e.Obj)] = chName(g, e.Obj)
			if p.stack[uint32(e.Obj)] == "Entropic Battlecruiser" {
				p.bcCast++
			}
		case events.TriggerPush, events.AbilityPush:
			ob := g.Obj(e.Obj)
			if ob == nil {
				break
			}
			p.sawDisc, p.sawLoss, p.sawDraws = false, false, 0
			switch chName(g, e.Obj) {
			case "Momentum Breaker":
				if e.Kind == events.TriggerPush {
					p.pend, p.opp, p.resolved = 1, 1-ob.Controller, false
					p.cant = !kd6OppBoard(g, p.opp)
					p.hand = len(g.Zone(state.ZHand, p.opp)) > 0
				}
			case "Gandalf, Wandering Wizard":
				if e.Kind == events.AbilityPush {
					p.pend, p.owner, p.resolved = 3, ob.Owner, false
				}
			}
		case events.Draw:
			if p.pend == 3 && e.Player == p.owner {
				p.sawDraws++
			}
		case events.MoveZone:
			if p.pend == 1 || p.pend == 2 {
				if e.From == state.ZHand && e.To == state.ZGraveyard {
					if o := g.Obj(e.Obj); o != nil && o.Owner == p.opp {
						p.sawDisc = true
					}
				}
			}
		case events.LifeChange:
			if p.pend == 2 && e.Player == p.opp && e.Amount < 0 {
				p.sawLoss = true
			}
		case events.Resolve:
			if p.stack[uint32(e.Obj)] == "Dawn's Truce" {
				p.truce++
			}
			delete(p.stack, uint32(e.Obj))
			if p.pend != 0 {
				p.resolved = true
			}
		case events.Priority:
			// the pending ability's effects are logged after its Resolve; the
			// next priority event ends them
			if p.pend == 0 || !p.resolved {
				break
			}
			switch p.pend {
			case 1:
				switch {
				case p.cant && p.hand && p.sawDisc:
					p.breakCant++
				case !p.cant:
					p.breakSac++
				default:
					p.breakNone++
				}
			case 2:
				switch {
				case p.cant && p.sawLoss:
					p.bcCant++
				case p.sawDisc && !p.sawLoss:
					p.bcDisc++
				}
			case 3:
				if p.sawDraws >= 3 {
					p.gandalf++
				}
			}
			p.pend, p.resolved = 0, false
		}
	}
	p.evAt = len(evs)
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Entropic Battlecruiser" {
				p.maxCharge = max(p.maxCharge, int(ob.Counter("CHARGE")))
			}
		}
	}
	return nil
}

// kd6Run plays seeds 1..seeds (odd seeds with the seats swapped); stopAtDiv
// ends it at the first divergence.
func kd6Run(t *testing.T, seeds uint64, stopAtDiv bool) (tally *kd6Tally, games, divs, stalls int, first string) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("definedoppnonrem")
	if err != nil {
		t.Fatal(err)
	}
	var tbls [2]*kop.Table
	var decks [2][2][]*cards.Card
	for sw := 0; sw < 2; sw++ {
		ents := [2][]cardset.Entry{kdefheads6Decks[sw], kdefheads6Decks[1-sw]}
		for s, d := range ents {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[sw][s] = append(decks[sw][s], c)
				}
			}
			if len(decks[sw][s]) != 40 {
				t.Fatalf("deck %d: %d cards", s, len(decks[sw][s]))
			}
		}
		if tbls[sw], _, _, err = set.Matchup(ents[0], ents[1]); err != nil {
			t.Fatal(err)
		}
	}
	tally = &kd6Tally{}
	for seed := uint64(1); seed <= seeds; seed++ {
		sw := int(seed & 1)
		cfg := Config{GorgeAutopaySurface: seed%4 == 0, PassElision: true}
		m, err := NewMatchup(reg, [2]string{"kd6-a", "kd6-b"}, decks[sw], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[sw], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.reset()
		pol := kd6Policy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tally.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if first == "" {
				first = "seed " + strconv.FormatUint(seed, 10) + ": " + d.What + " " + d.Field
			}
			lo := max(d.At-60, 0)
			t.Logf("seed %d %s %s at %d last actions %v\ndiff %v %s\nref %v\nker %v", seed, d.What, d.Field, d.At, actionStrings(res.Actions[max(lo, d.At-8):d.At]), d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			if stopAtDiv {
				return
			}
		}
	}
	return
}

// kd6Policy is castFirstPolicy that also takes a special action (Station's
// tap of another creature, CR 702.184a) whenever one is offered.
func kd6Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate || k == ActPlayLand || k == ActSpecial || k == ActAttack {
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

func TestDefinedHeadsOppNonRememberedLockstep(t *testing.T) {
	t.Parallel()
	tally, games, divs, stalls, first := kd6Run(t, 192, false)
	t.Logf("%d games, %d divergences, %d stalls; Momentum Breaker: can't-sacrifice discards %d, sacrificed %d, neither %d; Battlecruiser: granted attack triggers %d, can't-discard life losses %d, discards %d; Gandalf draws %d; Dawn's Truce resolutions %d; most charge counters %d (cast %d)",
		games, divs, stalls, tally.breakCant, tally.breakSac, tally.breakNone, tally.bcTrig, tally.bcCant, tally.bcDisc, tally.gandalf, tally.truce, tally.maxCharge, tally.bcCast)
	if divs != 0 {
		t.Fatalf("%d divergences, first %s", divs, first)
	}
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"Momentum Breaker can't-sacrifice discards", tally.breakCant, 1},
		{"Battlecruiser can't-discard life losses", tally.bcCant, 1},
		{"Gandalf draws", tally.gandalf, 3},
		{"Dawn's Truce resolutions", tally.truce, 3},
	} {
		if c.got < c.want {
			t.Errorf("%s: %d, want at least %d", c.name, c.got, c.want)
		}
	}
}

// TestDefinedHeadsOppNonRememberedMutation: with oppsNonRemCtrl ignoring the remembered set (the
// opponent who sacrificed or discarded is still named), the lockstep must
// diverge within 128 seeds. Not parallel: it sets a package-level hook.
func TestDefinedHeadsOppNonRememberedMutation(t *testing.T) {
	dirac.DebugOppNonRemFirstOnly = true
	defer func() { dirac.DebugOppNonRemFirstOnly = false }()
	_, games, divs, _, first := kd6Run(t, 128, true)
	if divs == 0 {
		t.Fatalf("no divergence in %d games with the remembered controllers ignored", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, first)
}
