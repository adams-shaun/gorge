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
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type dbgLine2 struct {
	Seed      uint64 `json:"seed"`
	Spellbench struct {
		Kind    string `json:"kind"`
		Index   int    `json:"index"`
		Seat    string `json:"seat"`
		Request struct {
			Decision json.RawMessage `json:"decision"`
		} `json:"request"`
	} `json:"spellbench"`
}

func TestZZDebugS41BExt(t *testing.T) {
	f, err := os.Open("../../../.ds4/scratch/f47b.jsonl")
	if err != nil { t.Skip(err) }
	defer f.Close()
	reg := testutil.CorpusRegistry(t)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var l dbgLine2
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil { t.Fatal(err) }
		if l.Seed != 15685514748697234467 && l.Seed != 11407309480946905114 && l.Seed != 1917441493038280081 && l.Seed != 15595749276196986914 { continue }
		var sd protocol.SeatDecision
		if err := json.Unmarshal(l.Spellbench.Request.Decision, &sd); err != nil { t.Fatal(err) }
		s := neutral.NewSession(reg)
		p, err := s.Payload(&sd)
		if err != nil { t.Fatalf("payload: %v", err) }
		v, d := agent.Rebuild(p)
		bot, err := bots.New("bot", bots.Options{Seed: 1, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
		if err != nil { t.Fatal(err) }
		in, err := bot.Decide(context.Background(), v, d)
		if err != nil { t.Fatal(err) }
		fmt.Printf("\n=== seed=%d kind=%s dkind=%s idx=%d sub=%d/%d intent=%v\n", l.Seed, l.Spellbench.Kind, d.Kind, l.Spellbench.Index, sd.Group.SubstepIndex, sd.Group.SubstepCount, in.Choices)
		for _, pv := range v.Players { for _, cv := range pv.Battlefield { if cv.Power > 0 || cv.Toughness > 0 { fmt.Printf("  bf id=%d name=%q %d/%d types=%q tapped=%v face=%v\n", cv.ID, cv.Name, cv.Power, cv.Toughness, cv.Types, cv.Tapped, cv.FaceDown) } } }
		for i, o := range d.Options {
			fmt.Printf(" opt %d kind=%s obj=%d atk=%d player=%d label=%q\n", i, o.Kind, o.Obj, o.Attacker, o.Player, o.Label)
		}
		for i, op := range p.Ops {
			fmt.Printf(" op %d %s option=%d unit=%d\n", i, op.Op, op.Option, op.Unit)
		}
	}
	if err := sc.Err(); err != nil { t.Fatal(err) }
}
