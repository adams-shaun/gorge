package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The BecomesTarget spell-source lockstep (CR 603.2): Labyrinth Guardian
// (ValidSource$ Spell: sacrificed by any targeting spell), Stormchaser Drake
// (Spell.YouCtrl: draws when its controller's spell targets it) and Opaline
// Sliver (Spell.OppCtrl: an opponent's spell) beside targeted spells and
// Prodigal Sorcerer's activated ping on both sides. Gorge's
// becomesTargetSourceMatches reads the ValidSource$ against the targeting
// stack object; the bare Spell base is a card on the stack (CR 112.1), so
// Prodigal Sorcerer's ping (an ability, CR 113.3b) does NOT fire Labyrinth
// Guardian, while Spell.YouCtrl / Spell.OppCtrl add the controller gate.
// The expected answer at every decision is gorge's.
var kbtssDeck = []cardset.Entry{
	{Name: "Labyrinth Guardian", Count: 3}, {Name: "Stormchaser Drake", Count: 3},
	{Name: "Opaline Sliver", Count: 3}, {Name: "Unsummon", Count: 3},
	{Name: "Shock", Count: 3}, {Name: "Giant Growth", Count: 3},
	{Name: "Prodigal Sorcerer", Count: 3}, {Name: "Grizzly Bears", Count: 3},
	{Name: "Island", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
}

func kbtssCarrier(name string) bool {
	switch name {
	case "Labyrinth Guardian", "Stormchaser Drake", "Opaline Sliver":
		return true
	}
	return false
}

// kbtssStats counts the targetings of each carrier and the triggers pushed,
// keyed "<card>|spell or ability|own or opp" (the targeting object's kind and
// its controller relative to the carrier's).
type kbtssStats struct{ targ, push map[string]int }

func newKbtssStats() *kbtssStats {
	return &kbtssStats{map[string]int{}, map[string]int{}}
}

func kbtssKey(name string, ab, own bool) string {
	k := name + "|spell|"
	if ab {
		k = name + "|ability|"
	}
	if own {
		return k + "own"
	}
	return k + "opp"
}

// kbtssProbe steers target choices to carriers (a third at the chooser's own)
// and otherwise casts, activates and plays lands.
type kbtssProbe struct{ rnd Policy }

func (p *kbtssProbe) Pick(c *PickCtx) Canon {
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
			if ob == nil || ob.Face() == nil || !kbtssCarrier(ob.Face().Name) {
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

// kbtssScan reads gorge's new events: each TargetsChosen naming a carrier is
// counted by its cause's kind and controller, and a TriggerPush whose source
// is a carrier is attributed to the most recent targeting of it.
func kbtssScan(ref *GorgeSide, seen *int, st *kbtssStats) {
	evs := ref.E.L.Events
	last := map[state.ObjID]string{}
	for ; *seen < len(evs); *seen++ {
		ev := evs[*seen]
		switch ev.Kind {
		case events.TargetsChosen:
			cause := ref.E.G.Obj(ev.Obj)
			if cause == nil {
				continue
			}
			for _, id := range ev.IDs {
				tgt := ref.E.G.Obj(id)
				if tgt == nil || tgt.Face() == nil || !kbtssCarrier(tgt.Face().Name) {
					continue
				}
				k := kbtssKey(tgt.Face().Name, cause.Ability != nil, cause.Controller == tgt.Controller)
				last[id] = k
				st.targ[k]++
			}
		case events.TriggerPush, events.GrantTriggerPush: // Opaline Sliver's trigger is a granted one
			if k, ok := last[ev.Obj]; ok {
				st.push[k]++
			}
		}
	}
}

func TestBecomesTargetSpellSourceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("btspellsrc")
	if err != nil {
		t.Fatal(err)
	}
	decks := [2][]cardset.Entry{kbtssDeck, kbtssDeck}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	st := newKbtssStats()
	div, stalls := 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kbtss-a", "kbtss-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &kbtssProbe{rnd: RandomPolicy(seed)}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			kbtssScan(ref, &seen, st)
			return nil
		}
		res := Lockstep(m, seed, k, probe, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Stall == "decisions" {
			stalls++
		}
		if d := res.Div; d != nil {
			div++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v)", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("256 games: %d divergences, %d stalls; targeted %v pushed %v", div, stalls, st.targ, st.push)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	// the preconditions each assertion depends on: the carrier was targeted
	// from the side the assertion is about
	for _, k := range []string{
		"Labyrinth Guardian|spell|opp", "Labyrinth Guardian|ability|opp",
		"Stormchaser Drake|spell|own", "Stormchaser Drake|spell|opp", "Stormchaser Drake|ability|opp",
		"Opaline Sliver|spell|own", "Opaline Sliver|spell|opp",
	} {
		if st.targ[k] == 0 {
			t.Errorf("no targeting %q in 256 games: the lockstep proves nothing for it", k)
		}
	}
	// the gates themselves, read off gorge's pushes: Spell.YouCtrl never
	// fires for an opponent's targeting, Spell.OppCtrl never for the
	// controller's own, and each fires for the side it names
	for _, k := range []string{"Labyrinth Guardian|ability|opp", "Stormchaser Drake|spell|opp", "Stormchaser Drake|ability|opp", "Opaline Sliver|spell|own"} {
		if st.push[k] != 0 {
			t.Errorf("%q pushed %d triggers, want 0", k, st.push[k])
		}
	}
	for _, k := range []string{"Labyrinth Guardian|spell|opp", "Stormchaser Drake|spell|own", "Opaline Sliver|spell|opp"} {
		if st.push[k] == 0 {
			t.Errorf("%q pushed no trigger", k)
		}
	}
}
