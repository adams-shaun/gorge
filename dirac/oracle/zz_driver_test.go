package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

var zzDecks = [2][]cardset.Entry{
	{
		{Name: "Mercurial Pretender", Count: 4}, {Name: "Flamestick Courier", Count: 4},
		{Name: "Signpost Scarecrow", Count: 4}, {Name: "Sandstone Needle", Count: 4},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 20},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 16},
	},
}

func zzPolicy(m *Matchup, seed uint64, log func(string, ...any)) Policy {
	inner := zzPolicy0(m, seed, log)
	return PolicyFunc(func(c *PickCtx) Canon {
		o := inner.Pick(c)
		if c.D.Kind == DPriority && c.D.Player == 0 {
			log("PICK seq %d kind %d mode %d src %s (of %d)", c.Seq, o.Kind(), o.Mode(), m.InstanceCard(int(o.Src())), len(c.D.Options))
		}
		return o
	})
}

func zzPolicy0(m *Matchup, seed uint64, log func(string, ...any)) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		if c.D.Kind != DPriority {
			for _, o := range c.D.Options {
				log("  nonprio seq %d opt kind %d mode %d src %s", c.Seq, o.Kind(), o.Mode(), m.InstanceCard(int(o.Src())))
			}
			for _, o := range c.D.Options {
				if o.Kind() >= ActChoose && m.InstanceCard(int(o.Src())) == "Flamestick Courier" {
					return o
				}
			}
			return rnd.Pick(c)
		}
		var hasCreature, hasScare, hasIsland bool
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
			if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Flamestick Courier" {
				hasCreature = true
			}
			if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Signpost Scarecrow" {
				hasScare = true
			}
			if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Island" {
				hasIsland = true
			}
		}
		nIsl := 0
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
			if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Island" {
				nIsl++
			}
		}
		if false {
			log("seq %d seat? player %d islands %d nopts %d", c.Seq, c.D.Player, nIsl, len(c.D.Options))
		}
		if kmercurialCopyOut(c.G) {
			for _, o := range c.D.Options {
				if o.Kind() == ActActivate && o.Mode() == 255 && m.InstanceCard(int(o.Src())) == "Mercurial Pretender" {
					return o
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActActivate {
					log("seq %d activate mode %d src %s tgt %d", c.Seq, o.Mode(), m.InstanceCard(int(o.Src())), o.Tgt())
				}
			}
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActPlayLand {
				if m.InstanceCard(int(o.Src())) == "Island" && hasIsland {
					continue
				}
				return o
			}
		}
		pri := []string{"Flamestick Courier", "Signpost Scarecrow", "Sandstone Needle"}
		if hasCreature && hasScare {
			pri = append(pri, "Mercurial Pretender")
		}
		for _, w := range pri {
			for _, o := range c.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == w {
					return o
				}
			}
		}
		for _, o := range c.D.Options {
			log("OPT kind %d mode %d src %s", o.Kind(), o.Mode(), m.InstanceCard(int(o.Src())))
		}
		var keep []Canon
		for _, o := range c.D.Options {
			if o.Kind() == ActActivate {
				continue
			}
			if o.Kind() == ActPlayLand && hasIsland && m.InstanceCard(int(o.Src())) == "Island" {
				continue
			}
			keep = append(keep, o)
		}
		d := *c.D
		d.Options = keep
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

var zzOut []string

func TestZZDriver(t *testing.T) {
	defer func() {
		os.WriteFile("/home/sadams/projects/gorge/.worktrees/agent-20261123T120257Z-5c9b6014/.ds4/scratch/zz.out", []byte(strings.Join(zzOut, "\n")), 0o644)
	}()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range zzDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s missing", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	tbl, _, _, err := set.Matchup(zzDecks[0], zzDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 2; seed++ {
		m, err := NewMatchup(reg, [2]string{"mp-a", "mp-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{GorgeAutopaySurface: true})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		bounce, seen := 0, 0
		_ = 0
		bfSeen := map[string]int{}
		var lines []string
		logf := func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			cnt := map[string]int{}
			for _, id := range ref.E.G.Zone(state.ZBattlefield, 0) {
				if o := ref.E.G.Obj(id); o != nil && o.Face() != nil {
					cnt[o.Face().Name]++
				}
			}
			for n, c := range cnt {
				if c > bfSeen[n] {
					bfSeen[n] = c
				}
			}
			for ; seen < len(evs); seen++ {
				ev := &evs[seen]
				o := ref.E.G.Obj(ev.Obj)
				if ev.Kind == events.MoveZone && o != nil && kmercurialPrinted(o) && ev.From == state.ZBattlefield && ev.To == state.ZHand {
					taps := ""
					for j := seen; j >= 0 && j > seen-80; j-- {
						e2 := &evs[j]
						if e2.Kind == events.Tap {
							if o2 := ref.E.G.Obj(e2.Obj); o2 != nil && o2.Face() != nil {
								taps += o2.Face().Name + ","
							}
						}
					}
					_ = taps
					bounce++
				}
			}
			return nil
		}
		rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, MaxDecisions: 2000, MaxTurns: 40, Check: check}
		res := Lockstep(m, seed, k, zzPolicy(m, seed, logf), rc)
		CloseSide(k)
		zzOut = append(zzOut, fmt.Sprintf("seed %d: decisions %d div %v bounces %d stall %q bf0 %v", seed, res.Decisions, res.Div != nil, bounce, res.Stall, bfSeen))
		if res.Div != nil {
			r := NewRepro(m, seed, rc, &res)
			r.Pool, r.Kernel = "custom", "custom"
			if err := r.Write(fmt.Sprintf("/home/sadams/projects/gorge/.worktrees/agent-20261123T120257Z-5c9b6014/.ds4/scratch/div%d.repro.json", seed)); err != nil {
				t.Fatal(err)
			}
		}
		if seed == 2 {
			for _, l := range lines {
				if strings.HasPrefix(l, "PICK") {
					zzOut = append(zzOut, l)
				}
			}
			seenOpt := map[string]bool{}
			for _, l := range lines {
				if strings.HasPrefix(l, "OPT") && !seenOpt[l] {
					seenOpt[l] = true
					zzOut = append(zzOut, l)
				}
			}
		}
		if d := res.Div; d != nil {
			zzOut = append(zzOut, fmt.Sprintf("  DIV seed %d at %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions))
		}
		if bounce > 0 && res.Div == nil {
			_ = lines
			if seed == 2 {
				r := NewRepro(m, seed, rc, &res)
				r.setActions(res.Actions[:461])
				r.MaxTurns = 24
				r.Pool, r.Kernel = "custom", "custom"
				if err := r.Write(filepath.Join("testdata", "repro", "granted_bounce_paid_through_fee_filter.repro.json")); err != nil {
					t.Fatal(err)
				}
			}
			if os.Getenv("ZZ_WRITE") != "" {
				return
			}
		}
	}
}
