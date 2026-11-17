package oracle

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"path/filepath"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// hgDecks is the scratch matchup: seat 0 holds one Heap Gate, exactly one
// Plains (a second Plains would let the plain census fund Ysgard's Call's
// second {W} and both sides offer it), Islands as filler, and six Horns of
// Valhalla; seat 1 only Islands. With the single Plains on the battlefield,
// the Adventure face's second {W} is payable only by activating Heap Gate
// (an Island's mana pays the filter's {1} fee).
func hgDecks() [2][]string {
	var d [2][]string
	add := func(s int, name string, n int) {
		for i := 0; i < n; i++ {
			d[s] = append(d[s], name)
		}
	}
	add(0, "Heap Gate", 1)
	add(0, "Plains", 1)
	add(0, "Island", 25)
	add(0, "Horn of Valhalla", 6)
	add(0, "Island", 7)
	add(1, "Island", 40)
	return d
}

func hgEntries(names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: n, Count: 1}
	}
	return es
}

// hgLandPolicy plays one land a turn and otherwise passes (the fallback keeps
// the non-priority asks answered).
func hgLandPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		if len(c.D.Options) == 0 {
			return 0
		}
		return rnd.Pick(c)
	})
}

// hgScanSeeds scans seeds on the gorge side alone (the offer implies the
// divergence: the kernel's face census has no filter route) and reports the
// first seed whose p0 priority decision offers the Adventure face cast.
func hgScanSeeds(reg *cards.Registry, decks [2][]string, first, last uint64) (hit string, ok bool, err error) {
	r := &Repro{Format: reproFormat, Decks: [2]string{"hg_0", "hg_1"}, Cards: decks,
		PassElision: true, Surface: "autopay", Kernel: "custom", Pool: "one-off", MaxTurns: 7}
	for seed := first; seed <= last; seed++ {
		if p := os.Getenv("HG_PROGRESS"); p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join("..", "..", p) // the test binary runs in the package dir
			}
			_ = os.WriteFile(p, []byte(fmt.Sprintf("worker at seed %d\n", seed)), 0o666)
		}
		r.Seed = seed
		m, e := r.Matchup(reg)
		if e != nil {
			return "", false, e
		}
		g, e := NewGorgeSide(m, seed, Config{PassElision: true, GorgeAutopaySurface: true}, nil)
		if e != nil {
			return "", false, e
		}
		pol := hgLandPolicy(seed)
		for step := 0; step < 1500; step++ {
			if over, _ := g.Terminal(); over {
				break
			}
			cdec := g.Step()
			if cdec.Kind == DNone {
				break
			}
			d := g.Pending()
			if d != nil && d.Player == 0 && d.Kind == decision.KPriority {
				for i := range d.Options {
					op := &d.Options[i]
					if op.Kind == "cast" && op.Mode == "adventure_alt" {
						return fmt.Sprintf("seed %d turn %d obj %d label %q", seed, g.E.G.Turn, op.Obj, op.Label), true, nil
					}
				}
			}
			g.Apply(pol.Pick(&PickCtx{D: &cdec, Seq: step, G: g}))
			if g.Err() != nil {
				return "", false, fmt.Errorf("gorge side error (seed %d): %w", seed, g.Err())
			}
		}
	}
	return "", false, nil
}

// TestScratchHGDump drives seeds through the gorge side, printing every p0
// priority decision's options plus the planner's verdict on every Horn play,
// until one reaches the setup (Heap Gate + Plains + Island untapped, a Horn in
// hand) or the seed budget ends.
func TestScratchHGDump(t *testing.T) {
	if os.Getenv("HG_DUMP") == "" {
		return
	}
	var last uint64
	fmt.Sscan(os.Getenv("HG_DUMP"), &last)
	reg := synthReg(t)
	decks := hgDecks()
	for seed := uint64(1); seed <= last; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"hg_0", "hg_1"}, Cards: decks,
			Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom", Pool: "one-off", MaxTurns: 7}
		m, e := r.Matchup(reg)
		if e != nil {
			t.Fatal(e)
		}
		g, e := NewGorgeSide(m, seed, Config{PassElision: true, GorgeAutopaySurface: true}, nil)
		if e != nil {
			t.Fatal(e)
		}
		pol := hgLandPolicy(seed)
		steps := 0
		for step := 0; step < 800; step++ {
			steps = step
			if over, _ := g.Terminal(); over {
				break
			}
			cdec := g.Step()
			if cdec.Kind == DNone {
				break
			}
			d := g.Pending()
			if d != nil && d.Player == 0 && d.Kind == decision.KPriority {
				gates, plains, islands, horns := 0, 0, 0, 0
				for i := range g.E.G.Objs {
					o := &g.E.G.Objs[i]
					name := g.E.Name(state.ObjID(i + 1))
					switch {
					case o.Zone == state.ZBattlefield && name == "Heap Gate" && !o.Tapped:
						gates++
					case o.Zone == state.ZBattlefield && name == "Plains" && !o.Tapped:
						plains++
					case o.Zone == state.ZBattlefield && name == "Island" && !o.Tapped:
						islands++
					case o.Zone == state.ZHand && name == "Horn of Valhalla":
						horns++
					}
				}
				if horns == 0 {
					g.Apply(pol.Pick(&PickCtx{D: &cdec, Seq: step, G: g}))
					continue
				}
				t.Logf("seed %d turn %d: board gates %d plains %d islands %d, horns in hand %d, %d options:",
					seed, g.E.G.Turn, gates, plains, islands, horns, len(d.Options))
				for i := range d.Options {
					op := &d.Options[i]
					if op.Kind == "cast" || op.Mode != "" || op.Label == "Activate Heap Gate for mana" {
						t.Logf("  opt kind %q mode %q obj %d ability %d label %q", op.Kind, op.Mode, op.Obj, op.Ability, op.Label)
					}
				}
				for _, pp := range g.E.PotentialPaymentPlans(0) {
					if !strings.Contains(pp.Action.Label, "Horn") && !strings.Contains(pp.Action.Label, "Ysgard") {
						continue
					}
					t.Logf("  plan %q mode %q: reason %q detail %q script %d", pp.Action.Label, pp.Action.Mode, pp.Reason, pp.Detail, len(pp.Script))
					for _, st := range pp.Script {
						for _, sp := range st.Picks {
							t.Logf("    step kind %q obj %d ability %d label %q mana %q", sp.Kind, sp.Obj, sp.Ability, sp.Label, sp.ManaSymbol)
						}
					}
				}
				t.Logf("  unpayable casts %v plays %d", d.PotentialUnpayableCasts, len(d.PotentialUnpayablePlays))
				if gates > 0 && plains > 0 && islands > 0 {
					t.Fatalf("setup window reached (dump above)")
				}
			}
			g.Apply(pol.Pick(&PickCtx{D: &cdec, Seq: step, G: g}))
			if g.Err() != nil {
				t.Fatalf("gorge side error (seed %d): %v", seed, g.Err())
			}
		}
		over, reason := g.Terminal()
		t.Logf("seed %d ended: steps %d over %v reason %q", seed, steps, over, reason)
	}
	t.Fatalf("no seed reached the setup window in %d", last)
}

// TestScratchHGSide scans seeds for the face-cast offer (parallel workers).
// Runs only with HG_RECORD=1.
func TestScratchHGSide(t *testing.T) {
	if os.Getenv("HG_RECORD") == "" {
		return
	}
	reg := synthReg(t)
	decks := hgDecks()
	const workers = 6
	found := make(chan string, 1)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			first, last := uint64(1+w*300), uint64((w+1)*300)
			hit, ok, e := hgScanSeeds(reg, decks, first, last)
			if e != nil {
				select {
				case found <- e.Error():
				default:
				}
				return
			}
			if ok {
				select {
				case found <- hit:
				default:
				}
			}
		}(w)
	}
	wg.Wait()
	select {
	case hit := <-found:
		t.Fatalf("face option offered: %s", hit)
	default:
		t.Fatal("no seed offers the face option")
	}
}

// TestScratchHGScan runs the toy matchup in lockstep (kernel custom, autopay)
// with the lands-only policy over seeds, logging every divergence and
// recording the first one whose gorge options hold a castface of Horn of
// Valhalla. Runs only with HG_RECORD=1.
func TestScratchHGScan(t *testing.T) {
	if os.Getenv("HG_RECORD") == "" {
		return
	}
	reg := synthReg(t)
	decks := hgDecks()
	set := printedDelveSet(t, decks)
	tbl, _, _, err := set.Matchup(hgEntries(decks[0]), hgEntries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	start := uint64(1)
	if v := os.Getenv("HG_SEED"); v != "" {
		fmt.Sscan(v, &start)
	}
	for seed := start; seed <= start+400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"hg_0", "hg_1"}, Cards: decks,
			Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom", Pool: "one-off", MaxTurns: 18}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{PassElision: true, GorgeAutopaySurface: true})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, hgLandPolicy(seed), r.RunConfig())
		CloseSide(k)
		if res.Div == nil {
			continue
		}
		ref := strings.Join(res.Div.RefOptions, " ")
		ker := strings.Join(res.Div.KerOptions, " ")
		t.Logf("seed %d: div at %d %s %s | ref: %s | ker: %s", seed, res.Div.At, res.Div.What, res.Div.Field, ref, ker)
		if res.Div.Field != "options" || !strings.Contains(ref, "castface") {
			continue
		}
		r.setActions(res.Actions)
		r.Divergence = res.Div
		r.Why = "gorge offers Ysgard's Call's second {W} through Heap Gate's filter; the kernel's face census has no filter route"
		sh := Shrink(&Corpus{Reg: reg}, r, SetSide(set, tbl, Config{PassElision: true, GorgeAutopaySurface: true}), 60)
		name := "horn_of_valhalla_face_second_white_through_a_filter"
		if err := sh.Write(filepath.Join("testdata", "repro", name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("recorded seed %d: %d -> %d actions, div %v", seed, len(res.Actions), len(sh.Actions), sh.Divergence)
		return
	}
	t.Fatal("no seed diverged with a castface option")
}
