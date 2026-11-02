package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The self BecomesTarget lockstep (CR 603.2): the four
// "whenever this becomes the target of a spell or ability an opponent
// controls" carriers (Altanak, the Thrice-Called; Ruric Thar, Biomagus; Old
// Fat Spider; Tenured Concocter) beside targeted removal and pumps on both
// sides and Prodigal Sorcerer's activated ping. Gorge's matcher pairs the
// line's ValidTarget$ Card.Self with the TargetsChosen event's chosen Ids
// and its ValidSource$ SpellAbility.OppCtrl with the targeting stack
// object's controller, so the trigger fires once per chosen target, only
// for an opponent's spell or ability. Concocter's optional draw is answered
// both ways. The expected answer at every decision is gorge's.
var kbtselfDecks = [2][]cardset.Entry{
	{
		{Name: "Altanak, the Thrice-Called", Count: 1}, {Name: "Ruric Thar, Biomagus", Count: 2},
		{Name: "Old Fat Spider", Count: 2}, {Name: "Tenured Concocter", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Giant Growth", Count: 3}, {Name: "Disfigure", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Altanak, the Thrice-Called", Count: 1}, {Name: "Ruric Thar, Biomagus", Count: 2},
		{Name: "Old Fat Spider", Count: 2}, {Name: "Tenured Concocter", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Giant Growth", Count: 3}, {Name: "Disfigure", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5},
	},
}

// kbtselfStats is the event-derived census plus the optional answers.
type kbtselfStats struct {
	self   int // TriggerPush of a self BecomesTarget line (accepted triggers)
	own    int // ... whose targeting source's controller was the trigger's own
	ownTgt int // TargetsChosen events naming a self carrier, by its controller
	yes    int // Tenured Concocter optional draws accepted
	no     int // ... declined
}

// kbtselfCarrier reports a card name carrying the self BecomesTarget line.
func kbtselfCarrier(name string) bool {
	switch name {
	case "Altanak, the Thrice-Called", "Ruric Thar, Biomagus", "Old Fat Spider", "Tenured Concocter":
		return true
	}
	return false
}

// kbtselfSelfBT reports whether a face prints the self BecomesTarget line.
func kbtselfSelfBT(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for _, tr := range f.Triggers {
		if tr.Mode == "BecomesTarget" &&
			strings.EqualFold(tr.ParamStr(cards.PKValidTarget), "Card.Self") &&
			strings.EqualFold(tr.ParamStr(cards.PKValidSource), "SpellAbility.OppCtrl") {
			return true
		}
	}
	return false
}

// kbtselfName is the name of the card an action casts or activates.
func kbtselfName(c *PickCtx, a Canon) string {
	if c.G == nil {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok || o.Obj == 0 {
		return ""
	}
	if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

// kbtselfSelfOptional reports whether a trigger_optional decision is a
// Tenured Concocter self BecomesTarget trigger ("you may draw a card").
func kbtselfSelfOptional(c *PickCtx) bool {
	if c.G == nil || len(c.G.E.G.Stack) == 0 {
		return false
	}
	st := c.G.E.G.Obj(c.G.E.G.Stack[len(c.G.E.G.Stack)-1])
	if st == nil || st.Ability == nil {
		return false
	}
	src := c.G.E.G.Obj(st.Source)
	if src == nil {
		return false
	}
	f := src.Face()
	if f == nil {
		return false
	}
	for _, tr := range f.Triggers {
		if tr.Effect == st.Ability {
			return tr.Mode == "BecomesTarget" &&
				strings.EqualFold(tr.ParamStr(cards.PKValidTarget), "Card.Self")
		}
	}
	return false
}

// kbtselfProbe answers the optional draw both ways, steers target choices to
// carriers (about a third at the chooser's own, so the negative case is
// exercised), and otherwise plays like castFirstPolicy.
type kbtselfProbe struct {
	st  *kbtselfStats
	rnd Policy
	ask int
}

func (p *kbtselfProbe) Pick(c *PickCtx) Canon {
	if c.D.Kind == DTriggerOptional && kbtselfSelfOptional(c) {
		p.ask++
		want := ActYes
		if p.ask%2 == 0 {
			want = ActNo
		}
		for _, a := range c.D.Options {
			if a.Kind() != want {
				continue
			}
			if want == ActYes {
				p.st.yes++
			} else {
				p.st.no++
			}
			return a
		}
	}
	if c.D.Kind == DTarget && c.G != nil {
		own := c.Seq%3 == 0
		var best []Canon
		for _, a := range c.D.Options {
			if a.Kind() != ActTarget {
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok || o.Obj == 0 {
				continue
			}
			ob := c.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil || !kbtselfCarrier(ob.Face().Name) {
				continue
			}
			if (ob.Controller == state.PlayerID(c.D.Player)) == own {
				best = append(best, a)
			}
		}
		if len(best) > 0 {
			return best[int(c.Seq)%len(best)]
		}
	}
	var buf []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActCast, ActCastFace, ActActivate, ActPlayLand:
			buf = append(buf, a)
		}
	}
	if len(buf) == 0 {
		return p.rnd.Pick(c)
	}
	d := *c.D
	d.Options = buf
	return p.rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
}

// kbtselfScan consumes gorge's new events: a TriggerPush of a self line is
// one accepted trigger, and its cause is the most recent TargetsChosen event
// naming the same source. Gorge's matcher rejects the trigger's own
// controller as the targeting source, so own must stay zero.
func kbtselfScan(ref *GorgeSide, seen *int, st *kbtselfStats) {
	evs := ref.E.L.Events
	last := map[state.ObjID]state.PlayerID{}
	for ; *seen < len(evs); *seen++ {
		ev := evs[*seen]
		switch ev.Kind {
		case events.TargetsChosen:
			if len(ev.IDs) == 0 {
				continue
			}
			cause := ref.E.G.Obj(ev.Obj)
			if cause == nil {
				continue
			}
			for _, id := range ev.IDs {
				tgt := ref.E.G.Obj(id)
				if tgt == nil || !kbtselfSelfBT(tgt.Face()) {
					continue
				}
				last[id] = cause.Controller
				if cause.Controller == tgt.Controller {
					st.ownTgt++
				}
			}
		case events.TriggerPush:
			o := ref.E.G.Obj(ev.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			f := o.Face()
			if ev.Amount < 0 || int(ev.Amount) >= len(f.Triggers) {
				continue
			}
			tr := f.Triggers[int(ev.Amount)]
			if tr.Mode != "BecomesTarget" ||
				!strings.EqualFold(tr.ParamStr(cards.PKValidTarget), "Card.Self") ||
				!strings.EqualFold(tr.ParamStr(cards.PKValidSource), "SpellAbility.OppCtrl") {
				continue
			}
			st.self++
			if cause, ok := last[ev.Obj]; ok && cause == ev.Player {
				st.own++
			}
		}
	}
}

// kbtselfSetup builds the matchup's corpus decks and table.
func kbtselfSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("btself")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kbtselfDecks {
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
	tbl, _, _, err := set.Matchup(kbtselfDecks[0], kbtselfDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// kbtselfRun plays seeds 1..seeds and returns the divergent games, kernel
// capacity stalls and the census. report logs each divergence; mutation runs
// stop at the first.
func kbtselfRun(t *testing.T, seeds uint64, report bool) (div, stalls int, st kbtselfStats) {
	t.Helper()
	reg, set, tbl, decks := kbtselfSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kbtself-a", "kbtself-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &kbtselfProbe{st: &st, rnd: RandomPolicy(seed)}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			kbtselfScan(ref, &seen, &st)
			return nil
		}
		res := Lockstep(m, seed, k, probe, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Stall == "decisions" {
			stalls++
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s %s)", seed, d.At, d.What, d.Field)
				return div, stalls, st
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, stalls, st
}

func TestBtselfLockstep(t *testing.T) {
	div, stalls, st := kbtselfRun(t, 256, true)
	t.Logf("256 games, btself %+v, %d divergences, %d stalls", st, div, stalls)
	if div != 0 {
		t.Errorf("%d divergences", div)
	}
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.self, 40, "self BecomesTarget triggers"},
		{st.yes, 10, "accepted optional draws"},
		{st.no, 10, "declined optional draws"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
	if st.own != 0 {
		t.Errorf("self BecomesTarget fired %d times for its controller's own targeting, want 0", st.own)
	}
	if st.ownTgt == 0 {
		t.Error("the pool never targeted a self carrier with its own controller's spell or ability: the own=0 assertion is vacuous")
	}
}

// TestBtselfMutation: with the self line dropped from btWatchers some seed
// diverges (gorge still fires the trigger).
func TestBtselfMutation(t *testing.T) {
	dirac.DebugBtSelfOff = true
	defer func() { dirac.DebugBtSelfOff = false }()
	if div, _, _ := kbtselfRun(t, 64, false); div == 0 {
		t.Fatal("DebugBtSelfOff: no divergence in 64 seeds")
	}
}
