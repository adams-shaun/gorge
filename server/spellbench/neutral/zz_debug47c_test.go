package neutral_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type dbg47cLine struct {
	Seed       uint64 `json:"seed"`
	Spellbench struct {
		Kind    string `json:"kind"`
		Index   int    `json:"index"`
		Request struct {
			Decision json.RawMessage `json:"decision"`
		} `json:"request"`
	} `json:"spellbench"`
}

func TestZZDebug47c(t *testing.T) {
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
		if l.Seed != 1917441493038280081 && l.Seed != 15595749276196986914 {
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
		nv, _ := agent.Rebuild(p)
		vv, _ := agent.Rebuild(ext)
		fmt.Printf("\n=== seed=%d idx=%d kind=%s\n", l.Seed, l.Spellbench.Index, l.Spellbench.Kind)
		nb, _ := json.Marshal(nv)
		vb, _ := json.Marshal(vv)
		fmt.Printf("native view: %s\n", vb)
		fmt.Printf("neutral view: %s\n", nb)
	}
}
