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

type dbg47bLine struct {
	Seed       uint64 `json:"seed"`
	Spellbench struct {
		Kind    string `json:"kind"`
		Index   int    `json:"index"`
		Request struct {
			Decision json.RawMessage `json:"decision"`
		} `json:"request"`
	} `json:"spellbench"`
}

func TestZZDebug47b(t *testing.T) {
	f, err := os.Open("../../../.ds4/scratch/f47b.jsonl")
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	reg := testutil.CorpusRegistry(t)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var l dbg47bLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		if l.Seed != 15685514748697234467 || l.Spellbench.Index != 756 {
			continue
		}
		var sd protocol.SeatDecision
		if err := json.Unmarshal(l.Spellbench.Request.Decision, &sd); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("acting=%v group=%+v\n", sd.ActingSeat, sd.Group)
		for _, p := range sd.Observation.Players {
			for _, r := range p.Battlefield {
				if r.Permanent == nil || r.Characteristics == nil {
					continue
				}
				fmt.Printf("obs %s %s id=%s statuses=%v types=%v tapped=%v\n", p.Seat, r.ObjectRef.ObjectID, r.ObjectID, r.Permanent.Statuses, r.Characteristics.Types, r.Permanent.Tapped)
			}
		}
		fmt.Printf("candidates=%d\n", len(sd.Candidates))
		for i, c := range sd.Candidates {
			b, _ := json.Marshal(c.Semantic)
			fmt.Printf(" cand %d %s\n", i, b)
		}
		p, err := neutral.NewSession(reg).Payload(&sd)
		if err != nil {
			t.Fatalf("payload: %v", err)
		}
		var ext xview.Payload
		if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &ext); err != nil {
			t.Fatal(err)
		}
		vv, vd := agent.Rebuild(ext)
		fmt.Printf("native view players:\n")
		for _, p := range vv.Players {
			for _, cv := range p.Battlefield {
				fmt.Printf("  p%d %s id=%d tapped=%v sick=%v att=%v kw=%v\n", p.ID, cv.Name, cv.ID, cv.Tapped, cv.SummonSick, cv.Attacking, cv.Keywords)
			}
		}
		for i, o := range vd.Options {
			fmt.Printf(" native opt %d kind=%s obj=%d atk=%d req=%v player=%d label=%q\n", i, o.Kind, o.Obj, o.Attacker, o.Required, o.Player, o.Label)
		}
		nv, nd := agent.Rebuild(p)
		fmt.Printf("neutral view players:\n")
		for _, p := range nv.Players {
			for _, cv := range p.Battlefield {
				fmt.Printf("  p%d %s id=%d tapped=%v sick=%v att=%v kw=%v\n", p.ID, cv.Name, cv.ID, cv.Tapped, cv.SummonSick, cv.Attacking, cv.Keywords)
			}
		}
		for i, o := range nd.Options {
			fmt.Printf(" neutral opt %d kind=%s obj=%d atk=%d req=%v player=%d label=%q\n", i, o.Kind, o.Obj, o.Attacker, o.Required, o.Player, o.Label)
		}
		bot, err := bots.New("bot", bots.Options{Seed: 1, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
		if err != nil {
			t.Fatal(err)
		}
		vi, _ := bot.Decide(context.Background(), vv, vd)
		ni, _ := bot.Decide(context.Background(), nv, nd)
		fmt.Printf("native intent=%v neutral intent=%v\n", vi.Choices, ni.Choices)
	}
}
