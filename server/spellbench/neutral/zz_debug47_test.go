package neutral_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type dbg47Line struct {
	Seed       uint64 `json:"seed"`
	Spellbench struct {
		Kind    string `json:"kind"`
		Index   int    `json:"index"`
		Seat    string `json:"seat"`
		Request struct {
			Decision json.RawMessage `json:"decision"`
		} `json:"request"`
	} `json:"spellbench"`
}

func TestZZDebug47(t *testing.T) {
	f, err := os.Open("../../../.ds4/scratch/f47b.jsonl")
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	reg := testutil.CorpusRegistry(t)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var l dbg47Line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		if l.Seed != 15685514748697234467 && l.Seed != 11407309480946905114 && l.Seed != 1917441493038280081 && l.Seed != 15595749276196986914 {
			continue
		}
		var sd protocol.SeatDecision
		if err := json.Unmarshal(l.Spellbench.Request.Decision, &sd); err != nil {
			t.Fatal(err)
		}
		p, err := neutral.NewSession(reg).Payload(&sd)
		if err != nil {
			t.Fatalf("payload: %v", err)
		}
		var ext xview.Payload
		if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &ext); err != nil {
			t.Fatal(err)
		}
		nv, nd := agent.Rebuild(p)
		vv, vd := agent.Rebuild(ext)
		bot, err := bots.New("bot", bots.Options{Seed: 1, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
		if err != nil {
			t.Fatal(err)
		}
		ni, err := bot.Decide(context.Background(), nv, nd)
		if err != nil {
			t.Fatal(err)
		}
		vi, err := bot.Decide(context.Background(), vv, vd)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("\n=== seed=%d idx=%d kind=%s sub=%d/%d\n", l.Seed, l.Spellbench.Index, l.Spellbench.Kind, sd.Group.SubstepIndex, sd.Group.SubstepCount)
		fmt.Printf(" native intent=%v neutral intent=%v\n", vi.Choices, ni.Choices)
		for i := 0; i < len(vd.Options) || i < len(nd.Options); i++ {
			var a, b string
			if i < len(vd.Options) {
				a = fmt.Sprintf("%s obj=%d atk=%d player=%d %q", vd.Options[i].Kind, vd.Options[i].Obj, vd.Options[i].Attacker, vd.Options[i].Player, vd.Options[i].Label)
			}
			if i < len(nd.Options) {
				b = fmt.Sprintf("%s obj=%d atk=%d player=%d %q", nd.Options[i].Kind, nd.Options[i].Obj, nd.Options[i].Attacker, nd.Options[i].Player, nd.Options[i].Label)
			}
			if a != b {
				fmt.Printf(" optdiff %d native=%s neutral=%s\n", i, a, b)
			}
		}
		nf, vf := facts(nv), facts(vv)
		in := map[string]bool{}
		for _, s := range nf {
			in[s] = true
		}
		for _, s := range vf {
			if !in[s] {
				fmt.Printf(" view only native: %s\n", s)
			}
		}
		in = map[string]bool{}
		for _, s := range vf {
			in[s] = true
		}
		for _, s := range nf {
			if !in[s] {
				fmt.Printf(" view only neutral: %s\n", s)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}
