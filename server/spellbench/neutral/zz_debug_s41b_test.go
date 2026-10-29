package neutral

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type dbgLine struct {
	Seed      int64 `json:"seed"`
	Spellbench struct {
		Kind    string `json:"kind"`
		Index   int    `json:"index"`
		Seat    string `json:"seat"`
		Request struct {
			Decision json.RawMessage `json:"decision"`
		} `json:"request"`
	} `json:"spellbench"`
}

func TestZZDebugS41B(t *testing.T) {
	f, err := os.Open("../../../.ds4/scratch/k41.jsonl")
	if err != nil { t.Skip(err) }
	defer f.Close()
	reg := testutil.CorpusRegistry(t)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var l dbgLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil { t.Fatal(err) }
		if l.Seed != 5640536404233877349 && l.Seed != 1073112203691670766 && l.Seed != 4276977129001154572 { continue }
		var sd protocol.SeatDecision
		if err := json.Unmarshal(l.Spellbench.Request.Decision, &sd); err != nil { t.Fatal(err) }
		s := NewSession(reg)
		p, err := s.Payload(&sd)
		if err != nil { t.Fatalf("payload: %v", err) }
		fmt.Printf("\n=== seed=%d kind=%s idx=%d seat=%s sub=%d/%d\n", l.Seed, l.Spellbench.Kind, l.Spellbench.Index, l.Spellbench.Seat, sd.Group.SubstepIndex, sd.Group.SubstepCount)
		for i, o := range p.Decision.Options {
			fmt.Printf(" opt %d kind=%s obj=%d atk=%d player=%d label=%q\n", i, o.Kind, o.Obj, o.Attacker, o.Player, o.Label)
		}
		for i, op := range p.Ops {
			fmt.Printf(" op %d %s option=%d unit=%d\n", i, op.Op, op.Option, op.Unit)
		}
	}
	if err := sc.Err(); err != nil { t.Fatal(err) }
}
