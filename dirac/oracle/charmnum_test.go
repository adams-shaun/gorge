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

// The multi-pick charm lockstep (CR 700.2, 601.2b, 603.3c): constructed
// matchups over the generated charmnum pool. The decks carry "choose two",
// "one or both" and "one or more" spells, two triggered multi-pick charms
// (Flash Thompson, Craftwork Crusher), and carriers whose two targeted modes
// can share a creature (Azula Always Lies, Flash Thompson).
//
// The counters come from gorge's event log and decisions, so the test fails
// if the kernel stops matching an arm; the mutations must diverge.
var charmnumDecks = [2][]cardset.Entry{
	{
		{Name: "Azula Always Lies", Count: 2}, {Name: "Flash Thompson, Spider-Fan", Count: 3},
		{Name: "Go Ninja Go", Count: 2}, {Name: "Overwhelming Surge", Count: 2},
		{Name: "Pinecone Strike", Count: 2}, {Name: "Molten Rebuke", Count: 1},
		{Name: "Farewell", Count: 1},
		{Name: "Perfect Intimidation", Count: 1}, {Name: "Ornithopter", Count: 2},
		{Name: "Mind Stone", Count: 1}, {Name: "Hill Giant", Count: 2},
		{Name: "Gray Ogre", Count: 2}, {Name: "Savannah Lions", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Season of Renewal", Count: 2}, {Name: "Return from the Wilds", Count: 2},
		{Name: "Urza's Command", Count: 2}, {Name: "Scour for Scrap", Count: 1},
		{Name: "Craftwork Crusher", Count: 2}, {Name: "Brilliance Unleashed", Count: 1},
		{Name: "Steal the Show", Count: 1}, {Name: "Artistic Refusal", Count: 1},
		{Name: "Avengers Disassembled", Count: 1}, {Name: "Ornithopter", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Gray Ogre", Count: 1},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 5},
	},
}

// charmnumTrigger names the triggered multi-pick carriers.
var charmnumTrigger = map[string]bool{"Flash Thompson, Spider-Fan": true, "Craftwork Crusher": true}

// charmnumOrder maps a carrier whose two modes emit distinct effect events to
// those kinds in printed order: the reversed-answer assertion below requires
// the lower printed mode's first effect event to precede the higher one's.
var charmnumOrder = map[string][2]events.Kind{
	"Flash Thompson, Spider-Fan": {events.Tap, events.Untap},
	"Go Ninja Go":                {events.MoveZone, events.Damage},
}

type charmnumTally struct {
	evAt int
	// the last modes decision seen (by decision Seq)
	mSeq    uint64
	mLabels []string
	mFirstN int
	mSource state.ObjID
	// the last target decision seen
	tSeq uint64
	// Flash/Crusher entries awaiting their mode ask
	pending map[state.ObjID]bool

	multiSpell, multiTrig int // ModeChosen with two or more labels
	doneFirst             int // one label although a second eligible mode existed
	reversed              int // an answer whose picks were not in printed order
	combined              int // KTarget charm_targets
	shared                int // ... listing one object in two groups
	removed               int // a multi-pick trigger that asked no mode
	badOrder              int // multi-label ModeChosen texts listed in answer order, not printed order
	lastPick              int
	pickSeq               uint64
	// the reversed-answer effect-order check: scan the events after a
	// reversed ModeChosen for the two modes' kinds, in printed order
	ordOn      bool
	ordSrc     state.ObjID
	ordStack   state.ObjID
	ordStarted bool
	ordKinds   [2]events.Kind
	ordAt      int
	ordSeen    [2]int
	ordChecked int
	ordBad     int
	brill      int // ModeChosen events from Brilliance Unleashed (deck b)
}

func (c *charmnumTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
		c.pending = map[state.ObjID]bool{}
		c.ordOn = false
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		if c.ordOn && j >= c.ordAt {
			// Only the resolving object's own resolution window counts: from
			// its Resolve to its MoveZone off the stack. A mode whose effect is
			// a no-op emits nothing, so both kinds must appear to check order.
			if !c.ordStarted {
				if e.Kind == events.Resolve && e.Obj == c.ordStack {
					c.ordStarted = true
				} else if e.Kind == events.MoveZone && e.Obj == c.ordStack {
					c.ordOn = false // fizzled before resolving
				}
			} else if e.Kind == events.MoveZone && e.Obj == c.ordStack {
				if c.ordSeen[0] >= 0 && c.ordSeen[1] >= 0 {
					c.ordChecked++
					if c.ordSeen[0] > c.ordSeen[1] {
						c.ordBad++
					}
				}
				c.ordOn = false
			} else {
				for m := 0; m < 2; m++ {
					if c.ordSeen[m] < 0 && e.Kind == c.ordKinds[m] {
						c.ordSeen[m] = j
					}
				}
			}
		}
		switch e.Kind {
		case events.MoveZone:
			if e.To == state.ZBattlefield {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && charmnumTrigger[o.Face().Name] {
					c.pending[e.Obj] = true
				}
			}
		case events.ModeChosen:
			c.pending = map[state.ObjID]bool{} // a trigger that asked its modes was not removed
			if c.ordOn && c.mSource != c.ordSrc {
				c.ordOn = false // the earlier answer never resolved (countered)
			}
			hits, last := 0, -1
			ordered := true
			for i, l := range c.mLabels {
				if l == "" {
					continue
				}
				if at := strings.Index(e.Text, l); at >= 0 {
					hits++
					if at < last {
						ordered = false
					}
					last = at
					_ = i
				}
			}
			trig := false
			if o := g.Obj(c.mSource); o != nil && o.Face() != nil {
				trig = charmnumTrigger[o.Face().Name]
			}
			switch {
			case hits >= 2 && trig:
				c.multiTrig++
			case hits >= 2:
				c.multiSpell++
			case hits == 1 && c.mFirstN >= 2 && len(c.mLabels) >= 2:
				c.doneFirst++
			}
			if hits >= 2 && !ordered {
				c.badOrder++ // the text lists the modes in ANSWER order; resolution is printed order
				if o := g.Obj(c.mSource); o != nil && o.Face() != nil {
					if o.Face().Name == "Brilliance Unleashed" {
						c.brill++
					}
					if kinds, ok := charmnumOrder[o.Face().Name]; ok {
						c.ordOn, c.ordSrc, c.ordStack, c.ordKinds, c.ordAt = true, c.mSource, e.Obj, kinds, j+1
						c.ordStarted, c.ordSeen = false, [2]int{-1, -1}
					}
				}
			}
		}
	}
	c.evAt = len(evs)
	fr := ref.fr
	if fr != nil && fr.d != nil {
		switch string(fr.d.Kind) {
		case "modes":
			if fr.d.Seq != c.mSeq {
				c.mSeq, c.mSource = fr.d.Seq, fr.d.Source
				c.mLabels, c.mFirstN = c.mLabels[:0], len(fr.d.Options)
				for _, o := range fr.d.Options {
					c.mLabels = append(c.mLabels, o.Label)
				}
				delete(c.pending, fr.d.Source)
			}
		case "target":
			if fr.d.ResumeKind == "charm_targets" && fr.d.Seq != c.tSeq {
				c.tSeq = fr.d.Seq
				c.combined++
				seen := map[state.ObjID]bool{}
				for _, o := range fr.d.Options {
					if o.Obj == 0 {
						continue
					}
					if seen[o.Obj] {
						c.shared++
						break
					}
					seen[o.Obj] = true
				}
			}
		case "priority":
			// a trigger put on the stack asks its modes before the next priority
			for id := range c.pending {
				c.removed++
				delete(c.pending, id)
			}
		}
	}
	return nil
}

// charmnumPolicy prefers multi-mode answers on 3 of every 4 seeds (the
// HIGHER mode first on half of them); the rest play RandomPolicy.
func charmnumPolicy(seed uint64, c *charmnumTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
		var pick Canon
		if p.D.Kind == DModes && seed%4 != 0 {
			buf = buf[:0]
			for _, a := range p.D.Options {
				if a.Kind() == ActMode {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				pick = buf[0]
				for _, a := range buf {
					if seed%2 == 1 && a.Mode() > pick.Mode() || seed%2 == 0 && a.Mode() < pick.Mode() {
						pick = a
					}
				}
			} else {
				pick = rnd.Pick(p)
			}
		} else if p.D.Kind == DPriority && seed%4 != 0 {
			// cast and play when possible, so the charms are reached
			buf = buf[:0]
			for _, a := range p.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 {
				pick = rnd.Pick(p)
			} else {
				d := *p.D
				d.Options = buf
				pick = rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			}
		} else {
			pick = rnd.Pick(p)
		}
		if p.D.Kind == DModes && p.G != nil && p.G.fr != nil && pick.Kind() == ActMode {
			if sq := p.G.fr.d.Seq; sq != c.pickSeq {
				c.pickSeq, c.lastPick = sq, -1
			}
			if c.lastPick >= 0 && int(pick.Mode()) < c.lastPick {
				c.reversed++
			}
			c.lastPick = int(pick.Mode())
		}
		return pick
	})
}

func charmnumSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][][2][]*cards.Card, [2]*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("charmnum")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][][2][]*cards.Card
	var tbls [2]*kop.Table
	for mi := 0; mi < 2; mi++ {
		md := [2][]cardset.Entry{charmnumDecks[mi], charmnumDecks[1-mi]}
		var pair [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					pair[s] = append(pair[s], c)
				}
			}
			if len(pair[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(pair[s]))
			}
		}
		decks[mi] = [][2][]*cards.Card{pair}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

func charmnumRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, tally *charmnumTally, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"charmnum-a", "charmnum-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, charmnumPolicy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 120, Check: check})
	CloseSide(k)
	return res
}

const charmnumSeeds = 60

func TestCharmnumLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbls := charmnumSetup(t)
	c := &charmnumTally{pending: map[state.ObjID]bool{}}
	games := 0
	for mi := 0; mi < 2; mi++ {
		for seed := uint64(1); seed <= charmnumSeeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			res := charmnumRun(t, reg, set, decks[mi][0], tbls[mi], seed, cfg, c, c.check)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: %d multi-mode spells, %d multi-mode triggers, %d done after the first pick, %d reversed picks, %d combined target asks (%d sharing an object), %d triggers removed, %d answer-ordered texts, %d effect orders checked (%d out of printed order)",
		games, c.multiSpell, c.multiTrig, c.doneFirst, c.reversed, c.combined, c.shared, c.removed, c.badOrder, c.ordChecked, c.ordBad)
	t.Logf("Brilliance Unleashed mode choices: %d", c.brill)
	for _, n := range []struct {
		name string
		got  int
	}{
		{"multi-mode spells", c.multiSpell}, {"multi-mode triggers", c.multiTrig},
		{"done after the first pick", c.doneFirst}, {"reversed picks", c.reversed},
		{"combined target asks", c.combined}, {"combined asks sharing an object", c.shared},
		{"reversed-answer effect orders", c.ordChecked},
	} {
		if n.got < 10 {
			t.Errorf("%d %s, want at least 10", n.got, n.name)
		}
	}
	if c.ordBad != 0 {
		t.Errorf("%d reversed answers resolved in answer order, want printed order", c.ordBad)
	}
	if c.brill == 0 {
		t.Errorf("Brilliance Unleashed was never cast: deck b does not exercise the fixed return rider")
	}
}

// TestCharmnumMutation: each mutation must diverge on some seed.
//
// DebugCharmAnyFeasible cannot: the offer gate and the trigger removal differ
// from "any one mode" only for a charm whose minimum exceeds the modes the
// board allows, and every lifted card has minimum 1 or two untargeted modes
// (checked over the corpus: Cryptic Command, Ojutai's Command and Primal
// Command each keep two untargeted or player-targeted modes). The unit tests
// TestCharmnumChooseTwoOffer and TestCharmnumTriggerRemoved cover the arm;
// here it must at least not diverge spuriously.
func TestCharmnumMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set, decks, tbls := charmnumSetup(t)
	type mut struct {
		name    string
		set     func(bool)
		unreach bool
	}
	muts := []mut{
		{name: "no Done", set: func(b bool) { dirac.DebugCharmNoDone = b }},
		{name: "any-feasible offer gate", set: func(b bool) { dirac.DebugCharmAnyFeasible = b }, unreach: true},
		{name: "slot-by-slot targets", set: func(b bool) { dirac.DebugCharmSlotOrder = b }},
	}
	for _, mu := range muts {
		mu.set(true)
		diverged := false
		for mi := 0; mi < 2 && !diverged; mi++ {
			for seed := uint64(1); seed <= charmnumSeeds && !diverged; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				res := charmnumRun(t, reg, set, decks[mi][0], tbls[mi], seed, cfg, &charmnumTally{}, nil)
				if diverged = res.Div != nil; diverged {
					t.Logf("mutation %q: matchup %d seed %d diverged at decision %d: %s %s", mu.name, mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				}
			}
		}
		mu.set(false)
		if !diverged && !mu.unreach {
			t.Errorf("mutation %q never diverged", mu.name)
		}
		if !diverged && mu.unreach {
			t.Logf("mutation %q did not diverge: unreachable in the lifted corpus (see the test comment)", mu.name)
		}
	}
}
