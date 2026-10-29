package neutral_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestZZDebug47f(t *testing.T) {
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
		if l.Seed != 15595749276196986914 {
			continue
		}
		var sd protocol.SeatDecision
		if err := json.Unmarshal(l.Spellbench.Request.Decision, &sd); err != nil {
			t.Fatal(err)
		}
		seed := secrets.AgentSeed(make([]byte, 32), l.Seed, sd.ActingSeat)
		fmt.Printf("\n=== seed=%d idx=%d seat=%s botseed=%d group=%+v\n", l.Seed, l.Spellbench.Index, sd.ActingSeat, seed, sd.Group)

		var xp xview.Payload
		if raw := sd.Extensions["x_gorge_view_v1"]; raw != nil {
			if err := json.Unmarshal(raw, &xp); err != nil {
				t.Fatal(err)
			}
		}
		refs := map[string]protocol.ObjectRef{}
		for _, p := range sd.Observation.Players {
			for _, o := range p.Battlefield {
				if o.CardName != nil {
					refs[*o.CardName] = o.ObjectRef
				}
			}
		}
		blockers := []string{"Expendable Troops", "Watchdog", "Inquisitor Exarch", "Guardians of Akrasa"}
		attackers := []string{"Gateway Sneak", "Gearsmith Guardian"}
		raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

		sess := neutral.NewSession(reg)
		bot, _ := bots.New("bot", bots.Options{Seed: seed, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
		var plan *agent.Plan
		var planPayload = struct {
			idx uint64
		}{}
		for step, bname := range blockers {
			bsd := sd
			bsd.Group.SubstepIndex = uint32(step)
			bsd.Candidates = nil
			for i, aname := range attackers {
				bsd.Candidates = append(bsd.Candidates, protocol.Candidate{
					CandidateID: uint32(i),
					Semantic: protocol.Semantic{Kind: "declare_block", Fields: map[string]any{
						"blocker": raw(refs[bname]), "attacker": raw(refs[aname]),
					}},
				})
			}
			bsd.Candidates = append(bsd.Candidates, protocol.Candidate{
				CandidateID: uint32(len(attackers)),
				Semantic: protocol.Semantic{Kind: "declare_block", Fields: map[string]any{
					"blocker": raw(refs[bname]), "attacker": nil,
				}},
			})
			p, err := sess.Payload(&bsd)
			if err != nil {
				t.Fatalf("payload step %d: %v", step, err)
			}
			v, d := agent.Rebuild(p)
			fmt.Printf("SUB%d cur=%s nativeIndex=%d opts=%d player=%v life=%v min=%d max=%d maxsum=%d\n", step, bname, p.NativeIndex, len(d.Options), d.Player, v.Players[d.Player].Life, d.Min, d.Max, d.MaxSum)
			for _, pl := range v.Players {
				for _, cv := range pl.Battlefield {
					fmt.Printf("    BF seat=%d id=%d %s p=%d t=%d dmg=%d kw=%v tap=%v atk=%v\n", pl.ID, cv.ID, cv.Name, cv.Power, cv.Toughness, cv.Damage, cv.Keywords, cv.Tapped, cv.Attacking)
				}
			}
			for i, o := range d.Options {
				fmt.Printf("  O%d %d obj=%v att=%v label=%q\n", step, i, o.Obj, o.Attacker, o.Label)
			}
			for i, op := range p.Ops {
				fmt.Printf("  OP%d %d op=%s option=%d unit=%d\n", step, i, op.Op, op.Option, op.Unit)
			}
			if plan == nil {
				brd := heuristic.BoardFromView(v)
				nbrd := heuristic.BoardFromView(xp.View)
				for _, cv := range v.Players[1].Battlefield {
					fmt.Printf("  NEU %s mana=%q cmc=%d cardsCMC=%d\n", cv.Name, cv.ManaCost, botpolicy.CmcOf(cv.ManaCost), brd.Cards.Get(cv.ID).CMC)
				}
				for _, cv := range xp.View.Players[1].Battlefield {
					fmt.Printf("  NAT %s mana=%q cmc=%d cardsCMC=%d\n", cv.Name, cv.ManaCost, botpolicy.CmcOf(cv.ManaCost), nbrd.Cards.Get(cv.ID).CMC)
				}
				for _, cv := range v.Players[0].Battlefield {
					cr := brd.Creatures.Get(cv.ID)
					fmt.Printf("  NEUC %s p=%d t=%d dmg=%d kw=%q atk=%v def=%d ctrl=%d blocks=%d\n", cv.Name, cr.Power, cr.Toughness, cr.Damage, cr.Keywords, cr.Attacking, cr.Defender, cr.Controller, cr.Blocks)
				}
				for _, cv := range xp.View.Players[0].Battlefield {
					cr := nbrd.Creatures.Get(cv.ID)
					fmt.Printf("  NATC %s p=%d t=%d dmg=%d kw=%q atk=%v def=%d ctrl=%d blocks=%d\n", cv.Name, cr.Power, cr.Toughness, cr.Damage, cr.Keywords, cr.Attacking, cr.Defender, cr.Controller, cr.Blocks)
				}
				for _, cv := range v.Players[1].Battlefield {
					cr := brd.Creatures.Get(cv.ID)
					fmt.Printf("  NEUA %s p=%d t=%d dmg=%d kw=%q atk=%v def=%d ctrl=%d blocks=%d\n", cv.Name, cr.Power, cr.Toughness, cr.Damage, cr.Keywords, cr.Attacking, cr.Defender, cr.Controller, cr.Blocks)
				}
				for _, cv := range xp.View.Players[1].Battlefield {
					cr := nbrd.Creatures.Get(cv.ID)
					fmt.Printf("  NATA %s p=%d t=%d dmg=%d kw=%q atk=%v def=%d ctrl=%d blocks=%d\n", cv.Name, cr.Power, cr.Toughness, cr.Damage, cr.Keywords, cr.Attacking, cr.Defender, cr.Controller, cr.Blocks)
				}
				fmt.Printf("  BOARD life0=%d life1=%d\n", brd.Life.Get(0), brd.Life.Get(1))
				for _, id := range []int{4, 8, 10, 12, 16, 20} {
					cr := brd.Creatures.Get(state.ObjID(id))
					fmt.Printf("    CR id=%d p=%d t=%d dmg=%d kw=%v atk=%v def=%d ctrl=%d\n", id, cr.Power, cr.Toughness, cr.Damage, cr.Keywords, cr.Attacking, cr.Defender, cr.Controller)
				}
				in, err := bot.Decide(context.Background(), v, d)
				if err != nil {
					t.Fatal(err)
				}
				plan = agent.NewPlan(p.NativeIndex, in)
				planPayload.idx = p.NativeIndex
				fmt.Printf("  PLAN intent=%+v\n", in)
			}
			if step == len(blockers)-1 {
				fresh, _ := bots.New("bot", bots.Options{Seed: seed, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
				freshIn, err := fresh.Decide(context.Background(), v, d)
				if err != nil {
					t.Fatal(err)
				}
				fmt.Printf("  FRESH intent=%+v\n", freshIn)
				nbot, _ := bots.New("bot", bots.Options{Seed: seed, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
				nativeIn, err := nbot.Decide(context.Background(), xp.View, xp.Decision)
				if err != nil {
					t.Fatal(err)
				}
				fmt.Printf("  NATBOT intent=%+v\n", nativeIn)
				for i, o := range xp.Decision.Options {
					fmt.Printf("  NOPT %d idx=%d kind=%q obj=%v att=%v minb=%d maxb=%d label=%q\n", i, o.Index, o.Kind, o.Obj, o.Attacker, o.MinBlockers, o.MaxBlockers, o.Label)
				}
				for i, o := range d.Options {
					fmt.Printf("  MOPT %d idx=%d kind=%q obj=%v att=%v minb=%d maxb=%d label=%q\n", i, o.Index, o.Kind, o.Obj, o.Attacker, o.MinBlockers, o.MaxBlockers, o.Label)
				}
				sems := make([]map[string]any, len(bsd.Candidates))
				for i, c := range bsd.Candidates {
					b, _ := json.Marshal(c.Semantic)
					var m map[string]any
					json.Unmarshal(b, &m)
					sems[i] = m
				}
				i, miss := agent.Pick(p, sems, plan, func(decision.Decision) decision.Intent { return decision.Intent{} })
				fmt.Printf("  PICK=%d miss=%q\n", i, miss)
			}
		}
	}
}
