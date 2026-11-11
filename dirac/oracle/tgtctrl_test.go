package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: TargetsWithDefinedController$
// TriggeredTarget under a damage trigger whose recipient is an opponent
// ("target creature that player controls"), lowered by kgen as an
// opponent-controlled slot (dirac/kgen/lower_targetset.go tgtCtrlTriggeredTarget):
//
//   - Satyr Firedancer: an instant or sorcery you control deals damage to an
//     opponent (Shock, Lightning Bolt at the face);
//   - Arm with Aether: the granted "deals damage to an opponent" bounce
//     trigger of each creature you control, until end of turn;
//   - Fear of Burning Alive: noncombat damage to an opponent under delirium
//     (Stitcher's Supplier mills for it).
//
// gorge binds TriggeredTarget to the damaged opponent (rules/target_legal.go
// definedControllerPlayer) and rechecks it on resolution (CR 608.2b).
var ktgtctrlDeckA = []cardset.Entry{
	{Name: "Satyr Firedancer", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 4},
	{Name: "Fear of Burning Alive", Count: 2}, {Name: "Stitcher's Supplier", Count: 4}, {Name: "Raging Goblin", Count: 3},
	{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 3},
}

var ktgtctrlDeckB = []cardset.Entry{
	{Name: "Arm with Aether", Count: 5}, {Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 4},
	{Name: "Stitcher's Supplier", Count: 3}, {Name: "Satyr Firedancer", Count: 4}, {Name: "Shock", Count: 4},
	{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 2},
}

var ktgtctrlMatchups = [][2][]cardset.Entry{
	{ktgtctrlDeckA, ktgtctrlDeckB}, {ktgtctrlDeckB, ktgtctrlDeckA}, {ktgtctrlDeckA, ktgtctrlDeckA}, {ktgtctrlDeckB, ktgtctrlDeckB},
}

// ktgtctrlCarrier reports whether sa is a TriggeredTarget carrier body.
func ktgtctrlCarrier(sa *cards.SA) bool {
	return sa != nil && sa.Params["TargetsWithDefinedController"] == "TriggeredTarget"
}

// ktgtctrlEntry is one carrier trigger seen on gorge's stack.
type ktgtctrlEntry struct {
	ctl     state.PlayerID
	tgtCtl  state.PlayerID
	targets int
	src     string // the trigger source's name
}

// ktgtctrlTally counts the carrier triggers by how they left gorge's stack,
// and the carrier target asks that offered one of the controller's own
// permanents (gorge offers what the kernel offers: the lockstep compares the
// option lists).
type ktgtctrlTally struct {
	seq                     uint64
	live                    map[state.ObjID]ktgtctrlEntry
	burns                   map[state.ObjID]state.PlayerID // a spell on the stack aimed at its caster's opponent
	asks, ownOffers         int
	oppTargeted, noTarget   int
	ownTargeted, playerTgts int
	emptyTarget             int // a carrier that reached the stack with no target
	bySatyr, byArm, byFear  int // oppTargeted by carrier
}

func ktgtctrlHasSatyr(g *state.Game, pl state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, pl) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Satyr Firedancer" {
			return true
		}
	}
	return false
}

func ktgtctrlHasCreature(g *state.Game, pl state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, pl) {
		if o := g.Obj(id); o != nil && o.EffectiveIsCreature() {
			return true
		}
	}
	return false
}

func (p *ktgtctrlTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if d := ref.Pending(); d != nil && d.Kind == decision.KTarget && uint64(d.Seq) != p.seq && len(g.Stack) > 0 {
		p.seq = uint64(d.Seq)
		if top := g.Obj(g.Stack[len(g.Stack)-1]); top != nil && ktgtctrlCarrier(top.Ability) {
			p.asks++
			for _, o := range d.Options {
				if ob := g.Obj(o.Obj); o.Obj != 0 && ob != nil && ob.Controller == top.Controller {
					p.ownOffers++
				}
			}
		}
	}
	on := map[state.ObjID]bool{}
	carriers := 0
	for _, sid := range g.Stack {
		ob := g.Obj(sid)
		if ob != nil && ob.Ability == nil {
			for _, tg := range ob.Targets {
				if tg.IsPlayer && tg.Player != ob.Controller {
					p.burns[sid] = ob.Controller
					on[sid] = true
				}
			}
		}
		if ob == nil || !ktgtctrlCarrier(ob.Ability) {
			continue
		}
		carriers++
		on[sid] = true
		e := ktgtctrlEntry{ctl: ob.Controller, tgtCtl: ob.Controller}
		if so := g.Obj(ob.Source); so != nil && so.Face() != nil {
			e.src = so.Face().Name
		}
		for _, tg := range ob.Targets {
			if tg.IsPlayer {
				p.playerTgts++
				continue
			}
			if to := g.Obj(tg.Obj); to != nil {
				e.targets++
				e.tgtCtl = to.Controller
			}
		}
		if old, ok := p.live[sid]; ok && old.targets > e.targets {
			e = old // a target that left keeps its pick for the tally
		}
		p.live[sid] = e
	}
	// "no legal target": a burn spell left the stack (resolved at the
	// opponent's face) while its caster controls Satyr Firedancer and the
	// opponent controls no creature; gorge never puts that trigger on the
	// stack (CR 603.3d), so it is counted from the board.
	for sid, ctl := range p.burns {
		if on[sid] {
			continue
		}
		delete(p.burns, sid)
		if carriers == 0 && ktgtctrlHasSatyr(g, ctl) && !ktgtctrlHasCreature(g, 1-ctl) {
			p.noTarget++
		}
	}
	for sid, e := range p.live {
		if on[sid] {
			continue
		}
		switch {
		case e.targets == 0:
			p.emptyTarget++
		case e.tgtCtl != e.ctl:
			p.oppTargeted++
			switch e.src {
			case "Satyr Firedancer":
				p.bySatyr++
			case "Fear of Burning Alive":
				p.byFear++
			default: // Arm with Aether's grant rides the recipient creature
				p.byArm++
			}
		default:
			p.ownTargeted++
		}
		delete(p.live, sid)
	}
	return nil
}

// ktgtctrlPolicy is ktgtsetPolicy (attack, cast, activate when offered).
func ktgtctrlPolicy(seed uint64) Policy { return ktgtsetPolicy(seed) }

// ktgtctrlRun plays seeds 1..seeds of every matchup with the generated table,
// or the one mut returns from it; stopAtDiv ends it at the first divergence.
func ktgtctrlRun(t *testing.T, seeds uint64, stopAtDiv bool, mut func(*kop.Table) *kop.Table) (tally *ktgtctrlTally, games, divs, stalls int, first string) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tgtctrl")
	if err != nil {
		t.Fatal(err)
	}
	tally = &ktgtctrlTally{live: map[state.ObjID]ktgtctrlEntry{}, burns: map[state.ObjID]state.PlayerID{}}
	for mi, md := range ktgtctrlMatchups {
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
		if mut != nil {
			tbl = mut(tbl)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			m, err := NewMatchup(reg, [2]string{"ktgtctrl-a", "ktgtctrl-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tally.seq = 0
			clear(tally.live)
			clear(tally.burns)
			pol := ktgtctrlPolicy(seed)
			if seed%8 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tally.check})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				if first == "" {
					first = "matchup " + strconv.Itoa(mi) + " seed " + strconv.FormatUint(seed, 10) + ": " + d.What + " " + d.Field
				}
				lo := max(d.At-60, 0)
				t.Logf("matchup %d seed %d %s %s at %d last actions %v\ndiff %v %s\nref %v\nker %v", mi, seed, d.What, d.Field, d.At, actionStrings(res.Actions[max(lo, d.At-8):d.At]), d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				if stopAtDiv {
					return
				}
			}
		}
	}
	return
}

func TestTgtctrlLockstep(t *testing.T) {
	t.Parallel()
	tally, games, divs, stalls, first := ktgtctrlRun(t, 256, false, nil)
	t.Logf("%d games, %d divergences, %d stalls; carrier asks %d (own-creature offers %d), resolved carriers: opponent's creature %d, own creature %d, player targets %d, empty %d; opponent's creature by Satyr Firedancer %d, Arm with Aether %d, Fear of Burning Alive %d; Satyr triggers with no legal target %d",
		games, divs, stalls, tally.asks, tally.ownOffers, tally.oppTargeted, tally.ownTargeted, tally.playerTgts, tally.emptyTarget,
		tally.bySatyr, tally.byArm, tally.byFear, tally.noTarget)
	if divs != 0 {
		t.Fatalf("%d divergences, first %s", divs, first)
	}
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	if tally.oppTargeted < 20 {
		t.Errorf("carriers that targeted an opponent's creature %d, want at least 20", tally.oppTargeted)
	}
	if tally.noTarget < 5 {
		t.Errorf("carriers with no legal target %d, want at least 5", tally.noTarget)
	}
	if tally.ownOffers != 0 || tally.ownTargeted != 0 || tally.playerTgts != 0 || tally.emptyTarget != 0 {
		t.Errorf("own-creature offers %d, own-creature targets %d, player targets %d, empty carriers %d: want 0",
			tally.ownOffers, tally.ownTargeted, tally.playerTgts, tally.emptyTarget)
	}
}

// ktgtctrlAnyCtrl is a TABLE mutation: every carrier slot (a damage-to-an-
// opponent trigger's object target) gets a copy of its filter with the
// controller predicate cleared, so it admits any creature. The predicate is
// either the OppCtrl filter (Ctrl == CtrlOpp) or, since kgen lowers
// "ControlledBy TriggeredTarget" as the damaged player's control at any seat
// count, kop.PredTrigPlayerCtrl; both are cleared.
func ktgtctrlAnyCtrl(tbl *kop.Table) *kop.Table {
	t2 := *tbl
	t2.Filters = append([]kop.Filter(nil), tbl.Filters...)
	t2.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range t2.Cards {
		c := &t2.Cards[i]
		c.Abilities = append([]kop.Ability(nil), c.Abilities...)
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigDamageDone || a.Trig.Flags2&kop.Tf2ToOpponent == 0 {
				continue
			}
			for s := 0; s < int(a.NTargets); s++ {
				ts := &a.Targets[s]
				if ts.Kind != kop.TgtObject {
					continue
				}
				f := t2.Filters[ts.Filter]
				hit := false
				if f.Ctrl == kop.CtrlOpp {
					f.Ctrl, hit = kop.CtrlAny, true
				}
				if f.Pred == kop.PredTrigPlayerCtrl {
					f.Pred, hit = kop.PredNone, true
				}
				if !hit {
					continue
				}
				ts.Filter = kop.FilterID(len(t2.Filters))
				t2.Filters = append(t2.Filters, f)
			}
		}
	}
	return &t2
}

// TestTgtctrlMutation: with the carrier slots admitting any creature, some
// seed diverges (the kernel offers the controller's own creatures).
func TestTgtctrlMutation(t *testing.T) {
	_, games, divs, _, first := ktgtctrlRun(t, 64, true, ktgtctrlAnyCtrl)
	if divs == 0 {
		t.Fatalf("no divergence in %d games with the carrier controller predicate cleared", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, first)
}
