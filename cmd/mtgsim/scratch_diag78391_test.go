package main

// SCRATCH diagnostic for the seed-78411 cmdr4 object-cap livelock -- not part
// of the fix, deleted before commit. Plays the game with the same seating and
// drive loop, catches the watcher's LivelockError, and dumps the event tail
// plus arena composition.

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestScratchDiag(t *testing.T) {
	names := []string{"valgavoth-endless-punishment", "hearthhull-worldseed-landfall", "b3-hearthhull-the-worldseed", "b3-cloud-ex-soldier"}
	policies := []string{"forge-heuristic", "lethal-pressure", "bot", "bot"}
	mulls := 0
	seed := uint64(78391)
	if s := os.Getenv("GORGE_DIAG_SEED"); s == "78411" {
		names = []string{"b3-flubs-the-fool", "b3-gavi-nest-warden", "b3-ulalek-fused-atrocity", "b1-ruxa-patient-professor"}
		policies = []string{"lethal-pressure", "lethal-pressure", "forge-heuristic-hold", "bot"}
		mulls = 1
		seed = 78411
	}
	reg := testutil.CorpusRegistry(t)
	decks := make([][]*cards.Card, len(names))
	var cmds [][]int
	for i, name := range names {
		f, err := testutil.LoadRepoDeckFile(name)
		if err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, f.CommanderIndices())
		d, err := testutil.LoadRepoDeck(reg, name)
		if err != nil {
			t.Fatal(err)
		}
		decks[i] = d
	}
	cfg := gameConfig(seed, names, decks, reg, rules.FormatCommander, cmds, mulls)
	if capovr := os.Getenv("GORGE_DIAG_CAPOVR"); capovr != "" {
		var n int
		fmt.Sscanf(capovr, "%d", &n)
		if n > 0 {
			cfg.LoopGuard = &rules.LoopGuard{MaxObjs: n}
		} else {
			cfg.LoopGuard = nil
		}
	} else {
		cfg.LoopGuard = rules.WithDefaultObjectCap(cfg.LoopGuard)
	}
	e := rules.NewStartingPlayerChoice(cfg)
	e.AskStartingPlayer()
	mseats, err := matchSeats(e, reg, seed, names, policies, "commander", mulls, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	route := newSeatRoute(cfg, mseats)
	f, err := os.Create(fmt.Sprintf("/home/sadams/projects/gorge/.worktrees/hunt-cmdr4-livelock-livelock-detected-object-cap-ob/.ds4/scratch/diag-%d.log", seed))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dump := func(e *rules.Engine) {
		evs := e.L.Events
		fmt.Fprintf(f, "total events=%d turn=%d over=%v\n", len(evs), e.G.Turn, e.G.Over)
		zoneCounts := map[state.Zone]int{}
		live := 0
		for i := range e.G.Objs {
			z := e.G.Objs[i].Zone
			if z != state.ZCeased {
				live++
			}
			zoneCounts[z]++
		}
		fmt.Fprintf(f, "live=%d arena=%d zones=%v stack=%d\n", live, len(e.G.Objs), zoneCounts, len(e.G.Stack))
		// kind histogram over the last 3000 events
		counts := map[events.Kind]int{}
		lo := len(evs) - 3000
		if lo < 0 {
			lo = 0
		}
		for i := lo; i < len(evs); i++ {
			counts[evs[i].Kind]++
		}
		type kc struct {
			k events.Kind
			n int
		}
		var kcs []kc
		for k, n := range counts {
			kcs = append(kcs, kc{k, n})
		}
		sort.Slice(kcs, func(i, j int) bool { return kcs[i].n > kcs[j].n })
		fmt.Fprintf(f, "last-3000 kind histogram:\n")
		for _, x := range kcs {
			fmt.Fprintf(f, "  %-24s %d\n", x.k, x.n)
		}
		// arena composition
		byName := map[string]int{}
		for i := range e.G.Objs {
			ob := e.G.Obj(state.ObjID(i))
			if ob == nil || ob.Face() == nil {
				byName["<nil>"]++
				continue
			}
			byName[ob.Face().Name]++
		}
		type nc struct {
			n string
			c int
		}
		var ncs []nc
		for n, c := range byName {
			ncs = append(ncs, nc{n, c})
		}
		sort.Slice(ncs, func(i, j int) bool { return ncs[i].c > ncs[j].c })
		fmt.Fprintf(f, "arena %d objects by name (top 40):\n", len(e.G.Objs))
		for i, x := range ncs {
			if i >= 40 {
				break
			}
			fmt.Fprintf(f, "  %6d  %s\n", x.c, x.n)
		}
		// what is object 2303
		o := e.G.Obj(2303)
		if o != nil && o.Face() != nil {
			fmt.Fprintf(f, "obj 2303: %s zone=%v\n", o.Face().Name, o.Zone)
		}
		// triggers per swarm face: a sample of swarms on the battlefield
		type trigInfo struct {
			id    state.ObjID
			isTok bool
			nf    int
			tm    []string
		}
		sample := map[state.ObjID]bool{273: true, 274: true, 275: true, 1000: true, 2303: true}
		var infos []trigInfo
		nbf := 0
		for i := range e.G.Objs {
			ob := e.G.Obj(state.ObjID(i))
			if ob == nil || ob.Zone != state.ZBattlefield {
				continue
			}
			f := ob.Face()
			if f == nil || f.Name != "Scute Swarm" {
				continue
			}
			nbf++
			if !sample[state.ObjID(i)] && len(infos) >= 8 {
				continue
			}
			tm := make([]string, 0, len(f.Triggers))
			for _, tr := range f.Triggers {
				tm = append(tm, tr.Mode)
			}
			infos = append(infos, trigInfo{id: state.ObjID(i), isTok: ob.IsToken, nf: 0, tm: tm})
		}
		fmt.Fprintf(f, "swarms on battlefield=%d, sample:\n", nbf)
		for _, ti := range infos {
			fmt.Fprintf(f, "  obj=%d token=%v faces=%d triggers=%v\n", ti.id, ti.isTok, ti.nf, strings.Join(ti.tm, " | "))
		}
		// what is object 277 (every trigger names ids=[277])
		o277 := e.G.Obj(277)
		if o277 != nil && o277.Face() != nil {
			fmt.Fprintf(f, "obj 277: %s zone=%v\n", o277.Face().Name, o277.Zone)
		}
		// token/land entries in the last 12000 events
		lo2 := len(evs) - 12000
		if lo2 < 0 {
			lo2 = 0
		}
		entries := 0
		for i := lo2; i < len(evs); i++ {
			k := evs[i].Kind
			if k == events.TokenCreate || k == events.Resolve || k == events.MoveZone || k == events.CounterChange || k == events.CardToken || k == events.PutOnStack || k == events.LandPlayed {
				fmt.Fprintf(f, "  ctx seq %d %s obj=%d p=%d text=%q\n", evs[i].Seq, k, evs[i].Obj, evs[i].Player, evs[i].Text)
			}
			_ = k
			_ = entries
		}
		// full events since seq 53000
		fmt.Fprintf(f, "full since 53000:\n")
		for i := range evs {
			ev := evs[i]
			if ev.Seq < 1000 {
				continue
			}
			s := fmt.Sprintf("  seq %d %s", ev.Seq, ev.Kind)
			if ev.Obj != 0 {
				if ob := e.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
					s += fmt.Sprintf(" obj=%d(%s)", ev.Obj, ob.Face().Name)
				} else {
					s += fmt.Sprintf(" obj=%d", ev.Obj)
				}
			}
			if ev.Player != 0 {
				s += fmt.Sprintf(" p=%d", ev.Player)
			}
			if len(ev.IDs) > 0 {
				s += fmt.Sprintf(" ids=%v", ev.IDs)
			}
			txt := ev.Text
			if len(txt) > 100 {
				txt = txt[:100] + "..."
			}
			if txt != "" {
				s += fmt.Sprintf(" %q", strings.TrimSpace(txt))
			}
			fmt.Fprintln(f, s)
		}
	}
	// drive loop like driveMatch, catching the watcher's panic
	n := 0
	liveCap := 0
	if s := os.Getenv("GORGE_DIAG_LIVECAP"); s != "" {
		fmt.Sscanf(s, "%d", &liveCap)
	}
	liveCount := func() int {
		n := 0
		for i := range e.G.Objs {
			if e.G.Objs[i].Zone != state.ZCeased {
				n++
			}
		}
		return n
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(f, "PANIC: %v\n", r)
			if liveCap > 0 {
				fmt.Fprintf(f, "live at abort=%d arena=%d\n", liveCount(), len(e.G.Objs))
			}
			dump(e)
		}
	}()
	ctx := t.Context()
	for !e.G.Over && e.Pending() != nil && n < maxIntents {
		if liveCap > 0 {
			if n%20000 == 0 {
				fmt.Fprintf(f, "progress n=%d seq=%d turn=%d live=%d arena=%d stack=%d\n", n, len(e.L.Events), e.G.Turn, liveCount(), len(e.G.Objs), len(e.G.Stack))
			}
			if liveCount() > liveCap {
				fmt.Fprintf(f, "LIVECAP abort at n=%d seq=%d\n", n, len(e.L.Events))
				return
			}
		}
		d := e.Pending()
		route.observe(e)
		if consumer, ok := mseats[int(d.Player)].(engineseat.PaymentPlanConsumer); ok && consumer.WantsPaymentActions() {
			e.EnsurePaymentActions()
		}
		if consumer, ok := mseats[int(d.Player)].(engineseat.TargetHintConsumer); ok && consumer.WantsTargetHints() {
			e.EnsureTargetHints()
		}
		in, err := route.decide(ctx, e, mseats[int(d.Player)], d)
		if err != nil {
			t.Fatalf("bot error at %d: %v", n, err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d rejected: %v", n, err)
		}
		n++
	}
	fmt.Fprintf(f, "finished: n=%d turn=%d over=%v winner=%d draw=%v\n", n, e.G.Turn, e.G.Over, e.G.Winner, e.G.Draw)
}
