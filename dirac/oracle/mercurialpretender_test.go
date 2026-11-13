package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mercurial Pretender: "You may have this enter as a copy of a creature you
// control, except it has '{2}{U}{U}: Return this creature to its owner's
// hand.'" gorge offers the as-enters copy election (AddAbilities$ is inside
// its ETB Clone scope) and folds the grant into the copy's copiable values
// (CR 707.9b; effects/clone.go): one of the copy's own abilities, priced off
// every source the controller can tap; the kernel applies it as a
// permanent override record from the printed card when a template is picked,
// hosts the ability on the copy and reads it from the printed card (dirac
// copy_asenters.go, chars_gen.go grantCardAt). The expected answer at every
// decision is gorge's: the copy election, the bounce's offer and its
// resolution.
//
// Seat 0 also floats {B}{B}{U}{U} with a Dark Ritual and a Manamorphose (blue
// twice) once a Pretender copy is out: the bounce is then paid from the pool.
// mercurialclone_test.go adds a Clone copying the Pretender copy.
var kmercurialDecks = [2][]cardset.Entry{
	{
		{Name: "Mercurial Pretender", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Dark Ritual", Count: 5}, {Name: "Manamorphose", Count: 5},
		{Name: "Island", Count: 10}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 16},
	},
}

// kmercurialCopyOut reports whether seat 0 controls a Mercurial Pretender
// that entered as a copy.
func kmercurialCopyOut(g *GorgeSide) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && kmercurialPrinted(o) && o.Face() != nil && o.Face().Name != "Mercurial Pretender" {
			return true
		}
	}
	return false
}

func kmercurialPrinted(o *state.Object) bool {
	return o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Mercurial Pretender"
}

// kmercurialPolicy: activate whenever offered (the granted bounce is the
// decks' only canonical activation); with a Pretender copy out, cast Dark
// Ritual, then Manamorphose, and name blue for its mana; else cast or play
// anything but the two rituals; else random.
func kmercurialPolicy(m *Matchup, seed uint64, offers *int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || c.G == nil {
			if c.D.Kind != DPriority && c.G != nil && kmercurialCopyOut(c.G) {
				for _, o := range c.D.Options {
					if o.Kind() >= ActChoose && o.Mode() == 1 { // blue, in WUBRG order
						return o
					}
				}
			}
			return rnd.Pick(c)
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActActivate {
				*offers++
				return o
			}
		}
		if kmercurialCopyOut(c.G) {
			for _, want := range []string{"Dark Ritual", "Manamorphose"} {
				for _, o := range c.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
		}
		buf = buf[:0]
		for _, o := range c.D.Options {
			switch o.Kind() {
			case ActCast:
				if n := m.InstanceCard(int(o.Src())); n == "Dark Ritual" || n == "Manamorphose" {
					continue
				}
				buf = append(buf, o)
			case ActPlayLand:
				buf = append(buf, o)
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

// TestMercurialPretenderLockstep: 50 games, 0 divergences, with Pretenders
// entering as copies and their granted bounce offered and resolving.
func TestMercurialPretenderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range kmercurialDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	tbl, _, _, err := set.Matchup(kmercurialDecks[0], kmercurialDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	const n = 50
	copies, bounces, offers, div := 0, 0, 0, 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"mp-a", "mp-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			if len(evs) < seen {
				seen = 0
			}
			for ; seen < len(evs); seen++ {
				ev := &evs[seen]
				o := ref.E.G.Obj(ev.Obj)
				if ev.Kind != events.MoveZone || o == nil || !kmercurialPrinted(o) {
					continue
				}
				switch {
				case ev.From == state.ZBattlefield && ev.To == state.ZHand:
					bounces++
				case ev.To == state.ZBattlefield && o.Face() != nil && o.Face().Name != "Mercurial Pretender":
					copies++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, kmercurialPolicy(m, seed, &offers), RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d divergences, %d Pretender copies, %d bounce offers, %d granted bounces", n, div, copies, offers, bounces)
	// measured: 33 copies, 2 offers, 2 bounces over the 50 seeds
	if copies < 10 || offers < 2 || bounces < 2 {
		t.Errorf("%d copies, %d bounce offers, %d granted bounces: want at least 10, 2 and 2 (the grant must be exercised)", copies, offers, bounces)
	}
}
