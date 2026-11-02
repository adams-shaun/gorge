package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cmd/sbv2dirac/sbv2"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
)

var (
	v2RegOnce sync.Once
	v2Reg     *cards.Registry
	v2RegErr  error
)

// v2Corpus opens the full gorge corpus once for the test binary, and skips a
// test when the corpus or the FDN pool is not on this box.
func v2Corpus(t *testing.T) (*cards.Registry, string, string) {
	t.Helper()
	v2RegOnce.Do(func() {
		dir, err := cardset.CorpusDir()
		if err != nil {
			v2RegErr = err
			return
		}
		v2Reg, v2RegErr = cards.OpenCorpus(dir)
	})
	if v2RegErr != nil {
		t.Skip("no gorge corpus: ", v2RegErr)
	}
	p, err := oracle.PoolByName("fdn")
	if err != nil {
		t.Skip("no fdn pool: ", err)
	}
	pool := filepath.Join(p.Inputs, "pool.tsv")
	decks := filepath.Join(p.Inputs, "decks")
	if _, err := os.Stat(pool); err != nil {
		t.Skip("no fdn pool.tsv: ", err)
	}
	return v2Reg, pool, decks
}

func v2TestOpts(t *testing.T, answerers ...string) v2TranscriptOpts {
	t.Helper()
	reg, pool, decks := v2Corpus(t)
	return v2TranscriptOpts{
		Corpus:       &oracle.Corpus{Reg: reg, DecksDir: decks},
		PoolPath:     pool,
		Split:        "all",
		DeckDir:      decks,
		Games:        20,
		Seed:         1,
		Answerers:    answerers,
		Workers:      4,
		MaxDecisions: 300,
		Quiet:        true,
	}
}

// TestV2TranscriptEqualOnCleanPool plays 20 seeded games per answerer and
// requires byte-identical transcripts over the whole game, rand included: the
// gorge host follows the dirac kernel's recorded chance draw for draw, so
// in-game randomness no longer parts the hosts.
func TestV2TranscriptEqualOnCleanPool(t *testing.T) {
	for _, answerer := range []string{"first", "rand"} {
		t.Run(answerer, func(t *testing.T) {
			sum, err := v2Run(v2TestOpts(t, answerer))
			if err != nil && len(sum.Diffs) == 0 {
				t.Fatal(err)
			}
			if sum.Games != 20 || sum.ProtocolErrors != 0 || sum.Divergences != 0 {
				t.Fatalf("%s: %d games, %d protocol errors, %d divergences", answerer, sum.Games, sum.ProtocolErrors, sum.Divergences)
			}
			for _, d := range sum.Diffs {
				t.Fatalf("%s: %d transcript diffs, first: %s", answerer, len(sum.Diffs), d)
			}
			if sum.DiracOutcomes["truncated"]+sum.GorgeOutcomes["truncated"] == 0 {
				t.Fatalf("%s: expected the decision cap to truncate, outcomes %v/%v", answerer, sum.DiracOutcomes, sum.GorgeOutcomes)
			}
		})
	}
}

// TestV2TranscriptDigFollowup pins pair 64: Vivien Reid's dig asks one v2
// arrangement but commits two native intents (choose/dig then dig_bottom);
// the dirac host must replay the folded follow-up instead of halting.
func TestV2TranscriptDigFollowup(t *testing.T) {
	opts := v2TestOpts(t, "rand")
	opts.Games, opts.FirstGame = 1, 64
	opts.MaxDecisions = 500 // the halt is at decision 323; v2TestOpts caps at 300
	sum, err := v2Run(opts)
	if err != nil && len(sum.Diffs) == 0 {
		t.Fatal(err)
	}
	if len(sum.Diffs) != 0 {
		t.Fatalf("pair 64: %d diffs, first: %s", len(sum.Diffs), sum.Diffs[0])
	}
}

// v2PlayTestPair builds and plays one pool pair exactly as v2Run's tasks do.
func v2PlayTestPair(t *testing.T, pair int) v2Played {
	t.Helper()
	reg, poolPath, decks := v2Corpus(t)
	pool, err := oracle.ReadPool(poolPath, "all")
	if err != nil {
		t.Fatal(err)
	}
	pp := oracle.PairingAt(pool, pair, 1)
	a, err := sbv2.ReadDeck(filepath.Join(decks, pp.A+".json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := sbv2.ReadDeck(filepath.Join(decks, pp.B+".json"))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := v2GameSecret(pair)
	if err != nil {
		t.Fatal(err)
	}
	return v2PlayTask(reg, v2Task{answerer: "rand", pair: pair, seed: pp.Seed, a: a, b: b, secret: secret, maxDec: 500})
}

// TestV2TranscriptChancePrefixPairs pins the games whose rand players reached
// an ordinary (non-shuffle) chance draw: pairs 65/93/94 used to halt the gorge
// host at gamecfg's unplanned-randomness contract while the dirac kernel
// legitimately played on. The gorge host now takes every draw from the
// kernel's whole recorded transcript, so it must finish identically; a halt or
// a diff means the transcript did not cover a draw.
func TestV2TranscriptChancePrefixPairs(t *testing.T) {
	for _, pair := range []int{65, 93, 94} {
		t.Run(fmt.Sprint(pair), func(t *testing.T) {
			r := v2PlayTestPair(t, pair)
			if r.err != nil {
				t.Fatal(r.err)
			}
			if r.game.Gorge.Outcome == "halted" && r.game.Gorge.Reason == "engine_contract_failure:unplanned_randomness" {
				t.Fatal("gorge halted on unplanned randomness: the kernel's transcript did not cover a draw")
			}
			if d := v2DiffStreams("rand", pair, 0, r.game.Dirac.Msgs, r.game.Gorge.Msgs); len(d) != 0 {
				t.Fatalf("pair %d: %d diffs, first: %s", pair, len(d), d[0])
			}
			if got, want := v2Outcome(r.game.Dirac), v2Outcome(r.game.Gorge); got != want {
				t.Fatalf("pair %d: dirac outcome %s, gorge outcome %s", pair, got, want)
			}
			opts := v2TestOpts(t, "rand")
			opts.Games, opts.FirstGame, opts.MaxDecisions = 1, pair, 500
			sum, err := v2Run(opts)
			if err != nil && len(sum.Diffs) == 0 {
				t.Fatal(err)
			}
			if len(sum.Diffs) != 0 {
				t.Fatalf("pair %d: %d diffs, first: %s", pair, len(sum.Diffs), sum.Diffs[0])
			}
		})
	}
}

// TestV2TranscriptPair25Seed7 pins the post-pin-45 regression. gorge's env
// now plans non-shuffle draws from the match secret, so the old replay
// trigger (gamecfg's UnplannedDraws) stopped firing for the kernel's ordinary
// draws: pair 25 seed 7 diverged at decision 382 with the hidden draws made
// from the secret instead of the kernel's record. The harness now installs the
// kernel's transcript as a positional chance planner, so every draw is the
// kernel's.
func TestV2TranscriptPair25Seed7(t *testing.T) {
	opts := v2TestOpts(t, "rand")
	opts.Games, opts.FirstGame, opts.Seed, opts.MaxDecisions = 1, 25, 7, 500
	sum, err := v2Run(opts)
	if err != nil && len(sum.Diffs) == 0 {
		t.Fatal(err)
	}
	if sum.Games != 1 || sum.ProtocolErrors != 0 || sum.Divergences != 0 {
		t.Fatalf("%d games, %d protocol errors, %d divergences", sum.Games, sum.ProtocolErrors, sum.Divergences)
	}
	if len(sum.Diffs) != 0 {
		t.Fatalf("pair 25 seed 7: %d diffs, first: %s", len(sum.Diffs), sum.Diffs[0])
	}
}

// TestV2TranscriptPair64Seed7 pins the probe half of the same fix: pair 64's
// Evolving Wilds search offers finish_selection only when the empty-answer
// probe accepts it, and that probe shuffles a clone library. The clone shares
// the chance planner but not the real draw position, so a planner that insists
// on the kernel's next bound rejects the finish and drops the candidate. The
// planner falls back to gamecfg's own secret-stream value for such a clone
// draw; the real game's 500 decisions stay the kernel's.
func TestV2TranscriptPair64Seed7(t *testing.T) {
	opts := v2TestOpts(t, "rand")
	opts.Games, opts.FirstGame, opts.Seed, opts.MaxDecisions = 1, 64, 7, 500
	sum, err := v2Run(opts)
	if err != nil && len(sum.Diffs) == 0 {
		t.Fatal(err)
	}
	if sum.Games != 1 || sum.ProtocolErrors != 0 || sum.Divergences != 0 {
		t.Fatalf("%d games, %d protocol errors, %d divergences", sum.Games, sum.ProtocolErrors, sum.Divergences)
	}
	if len(sum.Diffs) != 0 {
		t.Fatalf("pair 64 seed 7: %d diffs, first: %s", len(sum.Diffs), sum.Diffs[0])
	}
}

// TestV2TranscriptGameEndPair65 is one pair: the game ends on
// the same decision on both hosts, because the chance prefix covers the
// ordinary draw that used to halt gorge.
func TestV2TranscriptGameEndPair65(t *testing.T) {
	TestV2TranscriptChancePrefixPairs(t)
}

// TestV2TranscriptGraveyardOrder pins pair 11's surveil, the dominant
// observation diff before the rest-order fix: the v2 answer's pile-B order
// must reach the kernel's graveyard, so at decision 401 both hosts list the
// same cards in the same slots. The old code kept the offered order on the
// dirac side (graveyard[10] and [11] swapped) and failed here.
func TestV2TranscriptGraveyardOrder(t *testing.T) {
	opts := v2TestOpts(t, "rand")
	opts.Games, opts.FirstGame, opts.MaxDecisions = 1, 11, 500
	graves := map[string][]string{}
	v2MessageHook = func(host string, index int, raw []byte) []byte {
		if index != 401 {
			return raw
		}
		var m struct {
			SeatDecision struct {
				Observation struct {
					Players []struct {
						Graveyard []struct {
							CardName string `json:"card_name"`
						} `json:"graveyard"`
					} `json:"players"`
				} `json:"observation"`
			} `json:"seat_decision"`
		}
		if err := json.Unmarshal(raw, &m); err != nil || len(m.SeatDecision.Observation.Players) == 0 {
			return raw
		}
		for _, c := range m.SeatDecision.Observation.Players[0].Graveyard {
			graves[host] = append(graves[host], c.CardName)
		}
		return raw
	}
	v2PlayRefused = true
	t.Cleanup(func() { v2MessageHook, v2PlayRefused = nil, false })
	sum, err := v2Run(opts)
	if err != nil && len(sum.Diffs) == 0 {
		t.Fatal(err)
	}
	if sum.ProtocolErrors != 0 || sum.Divergences != 0 {
		t.Fatalf("pair 11: %d protocol errors, %d divergences", sum.ProtocolErrors, sum.Divergences)
	}
	if len(sum.Diffs) != 0 {
		t.Fatalf("pair 11: %d transcript diffs, first: %s", len(sum.Diffs), sum.Diffs[0])
	}
	dirac, gorge := graves["dirac"], graves["gorge"]
	if len(dirac) < 12 || len(gorge) < 12 {
		t.Fatalf("decision 401 graveyards: dirac %v, gorge %v (want at least 12 cards each)", dirac, gorge)
	}
	if !slices.Equal(dirac, gorge) {
		t.Fatalf("decision 401 graveyard order differs: dirac %v, gorge %v", dirac, gorge)
	}
}

// TestV2TranscriptSurveilArrangement pins the pile-B order of a scry/surveil
// (CR 404.3/701.22a): the agent's arrangement of the cards it did not keep on
// top must reach both hosts, since the graveyard order is public. Pairs 11
// and 28 (Lightshell Duo) and 16 (Cephalid Inkmage) diffed at decisions
// 401/369/213 before Config.ArrangeRestOrder was wired through the host.
func TestV2TranscriptSurveilArrangement(t *testing.T) {
	for _, pair := range []int{11, 16, 28} {
		opts := v2TestOpts(t, "rand")
		opts.Games, opts.FirstGame = 1, pair
		opts.MaxDecisions = 500 // pair 11 reaches its arrangement at decision 401
		sum, err := v2Run(opts)
		if err != nil && len(sum.Diffs) == 0 {
			t.Fatal(err)
		}
		if len(sum.Diffs) != 0 {
			t.Fatalf("pair %d: %d diffs, first: %s", pair, len(sum.Diffs), sum.Diffs[0])
		}
	}
}

// TestV2TranscriptNoChanceDiffs is the negative control for the chance
// following: with the gorge host on gamecfg's secret-stream chance the genesis
// deal no longer matches the dirac kernel's PCG shuffle, so the first
// decision's observation must differ. A clean run would mean the transcript is
// not what makes the hosts agree (or the diffing is a no-op).
func TestV2TranscriptNoChanceDiffs(t *testing.T) {
	opts := v2TestOpts(t, "first")
	opts.Games = 1
	opts.NoChance = true
	sum, err := v2Run(opts)
	if err != nil && len(sum.Diffs) == 0 {
		t.Fatal(err)
	}
	if sum.Games != 1 {
		t.Fatalf("%d games, want 1", sum.Games)
	}
	if len(sum.Diffs) == 0 {
		t.Fatal("independent genesis shuffles produced no diff; the kernel transcript is not what the summary says")
	}
	if d := sum.Diffs[0]; d.Bucket != v2BucketObservation || d.Index != 0 {
		t.Fatalf("first unfollowed diff is %s, want observation_field at decision 0", d)
	}
}

// TestV2GorgeHostMatchesEnv pins the port: with no kernel transcript,
// v2GorgeHost is gorge's own env.Server, byte for byte, over whole seeded
// games. A following run's diff is then the dirac host's, not the port's.
func TestV2GorgeHostMatchesEnv(t *testing.T) {
	reg, pool, decks := v2Corpus(t)
	p, err := oracle.ReadPool(pool, "all")
	if err != nil {
		t.Fatal(err)
	}
	for _, answerer := range []string{"first", "rand"} {
		for pair := 0; pair < 6; pair++ {
			pp := oracle.PairingAt(p, pair, 1)
			a, err := sbv2.ReadDeck(filepath.Join(decks, pp.A+".json"))
			if err != nil {
				t.Fatal(err)
			}
			b, err := sbv2.ReadDeck(filepath.Join(decks, pp.B+".json"))
			if err != nil {
				t.Fatal(err)
			}
			secret := secrets.GameSecret(v2RunSecret, uint64(pair))
			names := v2Names(a, b)
			gs := env.New(reg, nil)
			gs.EnableAutoPay()
			gs.AddDeck(a)
			gs.AddDeck(b)
			h := newV2GorgeHost(reg, nil)
			h.AddDeck(a)
			h.AddDeck(b)
			want := v2PlayHost("env", gs.Handle, a, b, names, pair, secret, pp.Seed, answerer, 120)
			got := v2PlayHost("port", h.Handle, a, b, names, pair, secret, pp.Seed, answerer, 120)
			if want.Err != nil || got.Err != nil {
				t.Fatalf("%s pair %d: env err %v, port err %v", answerer, pair, want.Err, got.Err)
			}
			if d := v2DiffStreams(answerer, pair, pp.Seed, want.Msgs, got.Msgs); len(d) != 0 || len(want.Msgs) == 0 {
				t.Fatalf("%s pair %d: port differs from env.Server in %d places over %d messages; first: %v", answerer, pair, len(d), len(want.Msgs), d)
			}
		}
	}
}

// TestV2TranscriptDetectsDiff perturbs one observation field in the dirac
// host's first decision through the test hook and requires exactly one
// observation_field diff at that decision.
func TestV2TranscriptDetectsDiff(t *testing.T) {
	opts := v2TestOpts(t, "first")
	opts.Games = 1
	perturbed := false
	v2MessageHook = func(host string, index int, raw []byte) []byte {
		if host != "dirac" || perturbed {
			return raw
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil || m["response_type"] != "decision" {
			return raw
		}
		sd, _ := m["seat_decision"].(map[string]any)
		obs, _ := sd["observation"].(map[string]any)
		turn, ok := obs["turn"].(float64)
		if !ok {
			return raw
		}
		obs["turn"] = turn + 1
		perturbed = true
		out, err := json.Marshal(m)
		if err != nil {
			return raw
		}
		return out
	}
	t.Cleanup(func() { v2MessageHook = nil })
	sum, err := v2Run(opts)
	if err != nil && len(sum.Diffs) == 0 {
		t.Fatal(err)
	}
	if !perturbed {
		t.Fatal("hook did not see a dirac decision")
	}
	if len(sum.Diffs) != 1 {
		t.Fatalf("%d diffs, want exactly 1: %v", len(sum.Diffs), sum.Diffs)
	}
	d := sum.Diffs[0]
	// The game is whichever pair played first: one holding a card refused by name is skipped.
	if d.Bucket != v2BucketObservation || d.Path != "seat_decision.observation.turn" {
		t.Fatalf("diff %+v, want one observation_field at observation.turn", d)
	}
}
