package neutral_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestZZDebug47d(t *testing.T) {
	f, err := os.Open("../../../.ds4/scratch/f47c.jsonl")
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	reg := testutil.CorpusRegistry(t)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var l dbg47cLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		if l.Seed != 1917441493038280081 {
			continue
		}
		var sd protocol.SeatDecision
		if err := json.Unmarshal(l.Spellbench.Request.Decision, &sd); err != nil {
			t.Fatal(err)
		}
		seed := secrets.AgentSeed(make([]byte, 32), l.Seed, sd.ActingSeat)
		fmt.Printf("\n=== seed=%d idx=%d seat=%s botseed=%d\n", l.Seed, l.Spellbench.Index, sd.ActingSeat, seed)

		var ext xview.Payload
		if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &ext); err != nil {
			t.Fatal(err)
		}
		nv, nd := agent.Rebuild(ext)
		np, err := neutral.NewSession(reg).Payload(&sd)
		if err != nil {
			t.Fatalf("payload: %v", err)
		}
		mv, md := agent.Rebuild(np)

		nbot, err := bots.New("bot", bots.Options{Seed: seed, Deps: bots.Deps{Cards: reg}})
		if err != nil {
			t.Fatal(err)
		}
		nin, err := nbot.Decide(context.Background(), nv, nd)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("NATIVE kind=%v player=%v max=%d opts=%d intent=%+v\n", nd.Kind, nd.Player, nd.Max, len(nd.Options), nin)
		for i, o := range nd.Options {
			fmt.Printf("  NOPT %d kind=%s obj=%v att=%v grp=%q label=%q\n", i, o.Kind, o.Obj, o.Attacker, o.Group, o.Label)
		}

		mbot, err := bots.New("bot", bots.Options{Seed: seed, Deps: bots.Deps{Cards: reg}})
		if err != nil {
			t.Fatal(err)
		}
		min, err := mbot.Decide(context.Background(), mv, md)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("NEUTRAL kind=%v player=%v max=%d opts=%d intent=%+v\n", md.Kind, md.Player, md.Max, len(md.Options), min)
		for i, o := range md.Options {
			fmt.Printf("  MOPT %d kind=%s obj=%v att=%v grp=%q label=%q\n", i, o.Kind, o.Obj, o.Attacker, o.Group, o.Label)
		}
	}
}

var _ = view.View{}
