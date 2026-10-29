package neutral_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestZZDebug47e(t *testing.T) {
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

		// Synthesize the substep-0 request: same board, the first blocker's
		// own candidates (Scrabbling Skullcrab blocks Automatic Librarian / none).
		raw0, _ := json.Marshal(sd.Candidates)
		s0 := strings.ReplaceAll(string(raw0), "Waterfront Bouncer", "Scrabbling Skullcrab")
		s0 = strings.ReplaceAll(s0, "o-04921f536b43992a", "o-853ad98dd0279004")
		var cands0 []protocol.Candidate
		if err := json.Unmarshal([]byte(s0), &cands0); err != nil {
			t.Fatal(err)
		}
		sd0 := sd
		sd0.Group.SubstepIndex = 0
		sd0.Candidates = cands0

		sess := neutral.NewSession(reg)
		p0, err := sess.Payload(&sd0)
		if err != nil {
			t.Fatalf("payload0: %v", err)
		}
		p1, err := sess.Payload(&sd)
		if err != nil {
			t.Fatalf("payload1: %v", err)
		}
		v0, d0 := agent.Rebuild(p0)
		v1, d1 := agent.Rebuild(p1)

		bot, _ := bots.New("bot", bots.Options{Seed: seed, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
		in0, err := bot.Decide(context.Background(), v0, d0)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("nativeIndex0=%d nativeIndex1=%d\n", p0.NativeIndex, p1.NativeIndex)
		fmt.Printf("SUB0 kind=%v player=%v max=%d opts=%d intent=%+v\n", d0.Kind, d0.Player, d0.Max, len(d0.Options), in0)
		for i, o := range d0.Options {
			fmt.Printf("  O0 %d obj=%v att=%v label=%q\n", i, o.Obj, o.Attacker, o.Label)
		}
		for i, op := range p0.Ops {
			fmt.Printf("  OP0 %d op=%s option=%d unit=%d\n", i, op.Op, op.Option, op.Unit)
		}
		fmt.Printf("SUB1 kind=%v player=%v max=%d opts=%d\n", d1.Kind, d1.Player, d1.Max, len(d1.Options))
		for i, o := range d1.Options {
			fmt.Printf("  O1 %d obj=%v att=%v label=%q\n", i, o.Obj, o.Attacker, o.Label)
		}
		for i, op := range p1.Ops {
			fmt.Printf("  OP1 %d op=%s option=%d unit=%d\n", i, op.Op, op.Option, op.Unit)
		}
		in1, err := bot.Decide(context.Background(), v1, d1)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("SUB1 fresh intent=%+v\n", in1)

		// Replay Pick with the substep-0 plan against substep 1.
		sems := make([]map[string]any, len(sd.Candidates))
		for i, c := range sd.Candidates {
			b, _ := json.Marshal(c.Semantic)
			var m map[string]any
			json.Unmarshal(b, &m)
			sems[i] = m
		}
		pl := agent.NewPlan(p0.NativeIndex, in0)
		i, miss := agent.Pick(p1, sems, pl, func(decision.Decision) decision.Intent { return decision.Intent{} })
		fmt.Printf("PICK with sub0 plan: candidate=%d miss=%q\n", i, miss)
		_ = xview.Payload{}
	}
}
