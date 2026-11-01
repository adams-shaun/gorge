package session_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/session"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

func startAudited(t *testing.T, deck string, maxSteps uint64) *session.Game {
	t.Helper()
	return startConfig(t, deck, maxSteps, session.Config{Reg: testcorpus.Registry(t), Ext: xview.New(), Audit: true})
}

func startExtended(t *testing.T, deck string, maxSteps uint64) *session.Game {
	t.Helper()
	return startConfig(t, deck, maxSteps, session.Config{Reg: testcorpus.Registry(t), Ext: xview.New()})
}

func startConfig(t *testing.T, deck string, maxSteps uint64, cfg session.Config) *session.Game {
	t.Helper()
	d, _ := catalog.ByID(deck)
	cs, err := catalog.Resolve(cfg.Reg, d)
	if err != nil {
		t.Fatal(err)
	}
	seat := "p0"
	req := &protocol.ResetReq{GameID: "g-w", Format: "pauper-bo1", MaxDecisions: 100000, MaxSteps: maxSteps,
		Rules: protocol.Rules{Mulligan: "london", StartingPlayer: "host_assigned", StartingSeat: &seat, Names: catalog.PoolNames()}}
	g, err := session.Start(cfg, "g-w", req, secrets.NewGame(make([]byte, 32)), [2][]*cards.Card{cs, cs})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// phaseRank orders phase_step values after declare_blockers, so a pose past
// the declare_blockers step proves the turn's combat damage was assigned.
var phaseRank = map[string]int{"untap": 0, "upkeep": 1, "draw": 2, "precombat_main": 3,
	"beginning_of_combat": 4, "declare_attackers": 5, "declare_blockers": 6, "combat_damage": 7,
	"end_of_combat": 8, "postcombat_main": 9, "end_step": 10, "cleanup": 11}

// divisionNeeding mirrors gorge rules.divisionNeeding from the public
// observation: a non-trample attacker with power above zero and two or more
// live blockers, whose legal divisions fit the engine's 128-option bound.
func divisionNeeding(obs protocol.Observation) (out []string) {
	blockedBy := map[string][]string{} // attacker object_id -> live blocker ids
	power := map[string]int32{}
	trample := map[string]bool{}
	for _, p := range obs.Players {
		for _, bf := range p.Battlefield {
			if bf.Permanent == nil {
				continue
			}
			for _, a := range bf.Permanent.BlockedAttackers {
				blockedBy[a.ObjectID] = append(blockedBy[a.ObjectID], bf.ObjectID)
			}
			if bf.Permanent.Attacking && bf.Characteristics != nil && bf.Characteristics.Power != nil {
				power[bf.ObjectID] = *bf.Characteristics.Power
				trample[bf.ObjectID] = slices.Contains(bf.Characteristics.Keywords, "trample")
			}
		}
	}
	for a, bs := range blockedBy {
		if len(bs) < 2 || power[a] <= 0 || trample[a] {
			continue
		}
		// C(power+n-1, n-1), saturating above gorge's maxDivisionOptions 128.
		n, pw := len(bs), int(power[a])
		k, total := n-1, pw+n-1
		if k > total-k {
			k = total - k
		}
		count := int64(1)
		for i := 0; i < k && count <= 128; i++ {
			count = count * int64(total-i) / int64(i+1)
		}
		if count <= 128 {
			out = append(out, a)
		}
	}
	return out
}

// runWitnessGame plays one audited uniform game for a deck under the fixed
// policy seed and reports what the game witnessed: a real declare_block
// answer (the consistency audit's declare_block branch), realized block
// decisions, and combat damage divisions answered internally.
func runWitnessGame(t *testing.T, deck string, seed uint64) (blocks, realizedBlocks, divisions int) {
	t.Helper()
	g := startAudited(t, deck, 4000)
	r := rand.New(rand.NewPCG(seed, 7))
	pending := map[string]bool{} // divisionNeeding attackers of this turn's last declare_blockers pose
	turn := uint32(0)
	blockedAttackers := map[string]bool{} // attackers this declare_blockers step already has a blocker for
	usedBlockers := map[string]bool{}
	for {
		dec, term := g.Pending()
		if term != nil {
			if term.Classification == "halted" {
				t.Fatalf("%s seed %d halted: %s", deck, seed, term.Reason)
			}
			break
		}
		obs := dec.SeatDecision.Observation
		if obs.Turn != turn {
			pending, turn = map[string]bool{}, obs.Turn
		}
		if obs.PhaseStep != "declare_blockers" {
			blockedAttackers, usedBlockers = map[string]bool{}, map[string]bool{}
		}
		switch {
		case phaseRank[obs.PhaseStep] == 6:
			pending = map[string]bool{} // recompute at every pose; the last one before damage stands
			for _, a := range divisionNeeding(obs) {
				pending[a] = true
			}
		case phaseRank[obs.PhaseStep] > 6 && len(pending) > 0:
			// The turn passed damage assignment with these attackers
			// double-blocked: gorge posed each division and the session
			// answered it internally, or the game would have halted.
			divisions += len(pending)
			pending = map[string]bool{}
		}
		k := r.IntN(len(dec.SeatDecision.Candidates))
		// Bias the random policy toward piling a second blocker on an
		// already-blocked attacker: a uniform pick almost never double
		// blocks, and a double block is what poses the damage division.
		if obs.PhaseStep == "declare_blockers" {
			for i, cand := range dec.SeatDecision.Candidates {
				if cand.Semantic.Kind != "declare_block" {
					continue
				}
				a, _ := cand.Semantic.Fields["attacker"].(*protocol.ObjectRef)
				b, _ := cand.Semantic.Fields["blocker"].(protocol.ObjectRef)
				if a != nil && b.ObjectID != "" && blockedAttackers[a.ObjectID] && !usedBlockers[b.ObjectID] {
					k = i
					break
				}
			}
		}
		c := dec.SeatDecision.Candidates[k]
		if c.Semantic.Kind == "declare_block" {
			if a, _ := c.Semantic.Fields["attacker"].(*protocol.ObjectRef); a != nil {
				blocks++ // a real block answer: the audit's declare_block branch ran
				blockedAttackers[a.ObjectID] = true
				if b, _ := c.Semantic.Fields["blocker"].(protocol.ObjectRef); b.ObjectID != "" {
					usedBlockers[b.ObjectID] = true
				}
			}
		}
		if perr := g.Step(&protocol.StepReq{GameID: "g-w", ExpectedStep: dec.Step, CandidateID: uint64(k),
			Echo: echo(t, c.Semantic)}); perr != nil {
			t.Fatalf("%s seed %d: %v", deck, seed, perr)
		}
	}
	if g.Leaks() != 0 || g.Inconsistent() != 0 {
		t.Fatalf("%s seed %d: %d leak-scan hits, %d inconsistent candidates",
			deck, seed, g.Leaks(), g.Inconsistent())
	}
	for _, rl := range g.Realized() {
		if rl.Kind == decision.KBlockers {
			realizedBlocks++
		}
	}
	return blocks, realizedBlocks, divisions
}

// Task 16's parked acceptance item: the audited live runs must witness a
// declare_block candidate answered (the consistency audit's declare_block
// branch) and a combat damage division answered (the engine-order internal
// answer, whose failure would halt the game), in uniform games of the five
// catalog decks — not only in fixtures.

// witnessSeeds is the audited uniform walk's seed list. The old single seed
// (3, 7) never reached the CawGates transform; (4, 7) is the regression that
// exposed the public printed-front name being scored as a hidden-name leak.
// runWitnessGame pairs each entry with the historical second word 7.
//
// One fixed policy seed is trajectory-fragile: a correct engine change that
// removes a decision (a singleton self-discard ask) shifts every later
// random pick, and the single double block that used to pose a division can
// disappear. Search a small fixed seed list instead, stopping at the first
// seed whose games witness both.
var witnessSeeds = []uint64{3, 4}

func TestAuditedUniformGamesWitnessBlocksAndDivisions(t *testing.T) {
	// Seed 3 is the historical baseline every deck plays; seed 4 is the
	// CawGates transform regression. Deeper seeds are searched per deck,
	// stopping as soon as the assertions are witnessed.
	const lastSeed = uint64(8)
	decks := []string{"Rally", "Spy", "Burn", "CawGates", "Wildfire"}
	blocks, realizedBlocks, divisions := 0, 0, 0
	play := func(deck string, seed uint64) {
		b, rb, d := runWitnessGame(t, deck, seed)
		blocks += b
		realizedBlocks += rb
		divisions += d
		t.Logf("%s seed %d: %d answered declare_block candidates, %d realized block decisions, %d answered divisions",
			deck, seed, b, rb, d)
	}
	for _, deck := range decks {
		for _, seed := range witnessSeeds {
			play(deck, seed)
		}
	}
	witnessSeed := uint64(0)
	for _, deck := range decks {
		for seed := witnessSeeds[len(witnessSeeds)-1] + 1; seed <= lastSeed && witnessSeed == 0; seed++ {
			play(deck, seed)
			if blocks > 0 && realizedBlocks > 0 && divisions > 0 {
				witnessSeed = seed
				t.Logf("seed %d witnessed the declare_block and combat damage division", seed)
			}
		}
	}
	if blocks == 0 || realizedBlocks == 0 {
		t.Fatalf("no declare_block witness in live audited games: %d answered candidates, %d realized block decisions",
			blocks, realizedBlocks)
	}
	if divisions == 0 {
		t.Fatal("no combat damage division was answered in live audited games")
	}
}
