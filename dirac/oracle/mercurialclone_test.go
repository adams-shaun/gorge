package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Clone that copies a Mercurial Pretender that is a copy of a creature is
// that creature with the Pretender's "{2}{U}{U}: Return this creature to its
// owner's hand": the AddAbilities$ exception is part of the Pretender's
// copiable values (CR 707.9b), so its copy has it (CR 707.3). gorge folds the
// grant into the copy's basis face; the kernel carries the Pretender's
// as-enters grant record to the Clone's slot (copy_asenters.go
// copyGrantsAdd). The expected answer at every decision is gorge's.
var kmercurialCloneDecks = [2][]cardset.Entry{
	{
		{Name: "Mercurial Pretender", Count: 6}, {Name: "Clone", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Island", Count: 14}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 16},
	},
}

// kmercurialClonePolicy is kmercurialPolicy that, at an as-enters copy
// election offering a Mercurial Pretender as the template, picks it.
func kmercurialClonePolicy(m *Matchup, seed uint64, offers *int) Policy {
	base := kmercurialPolicy(m, seed, offers)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority && c.G != nil {
			// a Clone once a Pretender copy is out, else the Pretender itself
			want := "Mercurial Pretender"
			if kmercurialCopyOut(c.G) {
				want = "Clone"
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
					return o
				}
			}
		}
		if c.D.Kind != DPriority && c.G != nil && kmercurialCopyOut(c.G) {
			for _, o := range c.D.Options {
				if o.Kind() >= ActChoose && o.Mode() == 0 && m.InstanceCard(int(o.Src())) == "Mercurial Pretender" {
					return o
				}
			}
		}
		if c.D.Kind == DPriority && c.G != nil {
			// the Clone's bounce is the row's point: take it, and keep the
			// Pretender in play (its own bounce is TestMercurialPretenderLockstep's)
			var keep []Canon
			for _, o := range c.D.Options {
				if o.Kind() == ActActivate {
					if m.InstanceCard(int(o.Src())) == "Clone" {
						return o
					}
					if m.InstanceCard(int(o.Src())) == "Mercurial Pretender" {
						continue
					}
				}
				keep = append(keep, o)
			}
			d := *c.D
			d.Options = keep
			return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		return base.Pick(c)
	})
}

func kcloneIsClone(o *state.Object) bool {
	return o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Clone"
}

func kcloneReturnAbilities(o *state.Object) (n int) {
	for _, sa := range o.Face().Abilities {
		if sa.API == "ChangeZone" {
			n++
		}
	}
	return n
}

// TestMercurialPretenderCloneLockstep: at most 50 games, 0 divergences, with
// Clones copying a Pretender copy, the grant offered on the Clone and
// resolving.
func TestMercurialPretenderCloneLockstep(t *testing.T) {
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
	for s, d := range kmercurialCloneDecks {
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
	tbl, _, _, err := set.Matchup(kmercurialCloneDecks[0], kmercurialCloneDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	const n = 50
	granted, bounces, offers, div := 0, 0, 0, 0
	_ = offers
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"mpc-a", "mpc-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		seenObj := map[state.ObjID]bool{}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			for _, id := range ref.E.G.Zone(state.ZBattlefield, 0) {
				if o := ref.E.G.Obj(id); kcloneIsClone(o) && o.Face() != nil {
					if !seenObj[id] && kcloneReturnAbilities(o) > 0 {
						seenObj[id] = true
						granted++
					}
				}
			}
			evs := ref.E.L.Events
			if len(evs) < seen {
				seen = 0
			}
			for ; seen < len(evs); seen++ {
				ev := &evs[seen]
				o := ref.E.G.Obj(ev.Obj)
				if ev.Kind != events.MoveZone || !kcloneIsClone(o) {
					continue
				}
				if ev.From == state.ZBattlefield && ev.To == state.ZHand {
					bounces++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, kmercurialClonePolicy(m, seed, &offers), RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d divergences, %d Clones with the grant, %d bounces of a Clone", n, div, granted, bounces)
	// measured: 14 Clones with the grant, 9 bounces over the 50 seeds
	if granted < 5 || bounces < 3 {
		t.Errorf("%d Clones with the grant, %d Clone bounces: want at least 5 and 3 (the chain must be exercised)", granted, bounces)
	}
}
