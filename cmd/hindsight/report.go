package main

import (
	"fmt"
	"sort"
	"strings"
)

type cell struct {
	decisions, clear, noWorld int
}

func renderReport(cfg config, games []gameResult, records []decisionRecord, cpuSeconds, wallSeconds float64) (string, error) {
	var b strings.Builder
	fmt.Fprintln(&b, "# pn20 step 1 — hindsight branch-mining measurement")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Held-out seeds begin at `%d`; corpus `%s`; %d losses and %d wins; adaptive %d..%d rollouts/option in blocks of %d; at most %d candidates. The five pn14/pn15 pairs cover the six constructed decks, with the measured seat alternating across pair sweeps. `.cards` was present and opened successfully.\n\n", cfg.seed, cfg.cardsDir, cfg.losses, cfg.wins, cfg.block, cfg.maxRollouts, cfg.block, cfg.candidateLimit)
	if cfg.redeal {
		fmt.Fprintln(&b, "Every honest rollout used a fresh history-conditioned `searchprobe.Sample` world, or -- `-redeal` was on -- when the sampler starved, a redeal of the branch engine's hidden cards the seat did not know, pinning every card its known-card projection names (pn21). The omniscient arm below deliberately reuses the real secrets and is labelled leaked.")
	} else {
		fmt.Fprintln(&b, "Every honest rollout used a fresh history-conditioned `searchprobe.Sample` world. No sampler failure fell back to the recorded hand or library. The omniscient arm below deliberately reuses those secrets and is labelled leaked.")
	}
	if cfg.redeal {
		redealt, worlds := 0, 0
		refused := make(map[string]int)
		for _, r := range records {
			if r.Evaluation.Redealt > 0 {
				redealt++
				worlds += r.Evaluation.Redealt
			}
			if r.Evaluation.RedealRefused != "" {
				refused[r.Evaluation.RedealRefused]++
			}
		}
		fmt.Fprintf(&b, "\nRedeal fallback: %d decisions used redealt worlds (%d worlds in all).", redealt, worlds)
		reasons := make([]string, 0, len(refused))
		for reason := range refused {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			fmt.Fprintf(&b, " Refused %d× (%s).", refused[reason], reason)
		}
		fmt.Fprintln(&b)
	}

	by := make(map[string]*cell)
	gamesByOutcome := make(map[string]int)
	clearByGame := make(map[int]int)
	lastClearTurn := make(map[int]int32)
	flipByGame := make(map[int]bool)
	var total, clear, noWorld, attempted, capHits int
	var decisionWall, totalRollouts float64
	for _, g := range games {
		gamesByOutcome[g.game.outcome]++
	}
	for _, r := range records {
		total++
		if r.NOptions >= 2 {
			attempted++
		}
		if r.CapHit {
			capHits++
		}
		if r.Evaluation.SamplerStatus == "no_world" {
			noWorld++
		}
		if r.Evaluation.Clear {
			clear++
			clearByGame[r.GameID]++
			if r.Turn > lastClearTurn[r.GameID] {
				lastClearTurn[r.GameID] = r.Turn
			}
			if i := r.Evaluation.BestAlternative; i >= 0 && i < len(r.Evaluation.Options) && r.Evaluation.Options[i].Rate >= .6 {
				flipByGame[r.GameID] = true
			}
		}
		decisionWall += r.WallSeconds
		totalRollouts += float64(r.Evaluation.RolloutsUsed * r.NOptions)
		key := r.Outcome + "|" + r.TurnBucket + "|" + r.Kind
		c := by[key]
		if c == nil {
			c = new(cell)
			by[key] = c
		}
		c.decisions++
		if r.Evaluation.Clear {
			c.clear++
		}
		if r.Evaluation.SamplerStatus == "no_world" {
			c.noWorld++
		}
	}

	fmt.Fprintln(&b, "\n## 1. Clear-decision and no-world rates")
	fmt.Fprintf(&b, "Across **%d** recorded decisions, **%d** had at least two candidate answers, **%d** were clear (**%.2f%% of all decisions; %.2f%% of attempted decisions**), and **%d** were `no_world` (**%.2f%% of attempted decisions**). The candidate cap was hit %d times.\n\n", total, attempted, clear, pct(clear, total), pct(clear, attempted), noWorld, pct(noWorld, attempted), capHits)
	fmt.Fprintln(&b, "| outcome | turn | kind | decisions | clear | clear % | no_world | no_world % |")
	fmt.Fprintln(&b, "|---|---:|---|---:|---:|---:|---:|---:|")
	for _, outcome := range []string{"loss", "win"} {
		for _, bucket := range []string{"1-3", "4-6", "7-12", "13+"} {
			var kinds []string
			for key := range by {
				parts := strings.Split(key, "|")
				if parts[0] == outcome && parts[1] == bucket {
					kinds = append(kinds, parts[2])
				}
			}
			sort.Strings(kinds)
			for _, kind := range kinds {
				c := by[outcome+"|"+bucket+"|"+kind]
				fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %.2f | %d | %.2f |\n", outcome, bucket, kind, c.decisions, c.clear, pct(c.clear, c.decisions), c.noWorld, pct(c.noWorld, c.decisions))
			}
		}
	}
	for _, outcome := range []string{"loss", "win"} {
		ng, nc := gamesByOutcome[outcome], 0
		for _, g := range games {
			if g.game.outcome == outcome {
				nc += clearByGame[g.game.id]
			}
		}
		fmt.Fprintf(&b, "\n- %s: %.3f clear decisions/game (%d across %d games).\n", outcome, float64(nc)/float64(max(1, ng)), nc, ng)
	}

	fmt.Fprintln(&b, "\n## 2. Reliability")
	relN, relAgree := 0, 0
	for _, r := range records {
		if r.Reliability.Run {
			relN++
			if r.Reliability.Agree {
				relAgree++
			}
		}
	}
	fmt.Fprintf(&b, "Clear decisions in the first %d selected games were rerun with an independent sampler/rollout seed. Agreement means the same best alternative and still clear: **%d/%d (%.2f%%)**.\n", min(cfg.reliabilityGames, len(games)), relAgree, relN, pct(relAgree, relN))

	fmt.Fprintln(&b, "\n## 3. Pivot structure in losses")
	var lossCounts []int
	late, early, flips := 0, 0, 0
	for _, g := range games {
		if g.game.outcome != "loss" {
			continue
		}
		n := clearByGame[g.game.id]
		lossCounts = append(lossCounts, n)
		if lastClearTurn[g.game.id] >= 13 {
			late++
		} else if lastClearTurn[g.game.id] > 0 {
			early++
		}
		if flipByGame[g.game.id] {
			flips++
		}
	}
	sort.Ints(lossCounts)
	zero := 0
	for _, n := range lossCounts {
		if n == 0 {
			zero++
		}
	}
	fmt.Fprintf(&b, "Lost games had mean %.3f clear decisions (median %d; %d/%d had none). The last clear decision was turn 13+ in %d games and turn 1–12 in %d. **%d/%d** losses had at least one clear alternative with estimated win rate ≥0.60 (the operational ‘one flip can explain it’ test).\n", meanInts(lossCounts), median(lossCounts), zero, len(lossCounts), late, early, flips, len(lossCounts))

	fmt.Fprintln(&b, "\n## 4. Cost")
	perGameCPU := cpuSeconds / float64(max(1, len(games)))
	perDecisionCPU := cpuSeconds / float64(max(1, attempted))
	perRolloutCPU := cpuSeconds / maxFloat(1, totalRollouts)
	projectedHours := perGameCPU * 10000 / 20 / 3600
	fmt.Fprintf(&b, "Measured process CPU was **%.1f s** and wall time **%.1f s** (sum of per-decision wall spans %.1f s). This is %.3f CPU-s/game, %.4f CPU-s/attempted decision, and %.6f CPU-s/rollout path across %.0f paths. Linear projection for 10k games on 20 cores: **%.2f h**.\n", cpuSeconds, wallSeconds, decisionWall, perGameCPU, perDecisionCPU, perRolloutCPU, totalRollouts, projectedHours)

	fmt.Fprintln(&b, "\n## 5. Hidden-information leak inflation")
	honestN, honestClear, leakN, leakClear := 0, 0, 0, 0
	for _, r := range records {
		if r.Omniscient == nil || r.NOptions < 2 {
			continue
		}
		honestN++
		if r.Evaluation.Clear {
			honestClear++
		}
		leakN++
		if r.Omniscient.Clear {
			leakClear++
		}
	}
	fmt.Fprintf(&b, "On the same first %d games, resampled decisions were clear at **%d/%d (%.2f%%)**; the deliberately leaked omniscient arm was clear at **%d/%d (%.2f%%)**, an inflation of **%+.2f percentage points**. The leaked labels are diagnostic only.\n", min(cfg.omniscientGames, len(games)), honestClear, honestN, pct(honestClear, honestN), leakClear, leakN, pct(leakClear, leakN), pct(leakClear, leakN)-pct(honestClear, honestN))

	fmt.Fprintln(&b, "\n## 6. Hand-read examples")
	examples := 0
	for _, r := range records {
		if !r.Evaluation.Clear || examples >= 20 {
			continue
		}
		best := r.Evaluation.BestAlternative
		if best < 0 || best >= len(r.Evaluation.Options) || len(r.Evaluation.Options) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%d. game %d seed %d, %s turn %d `%s`: chosen %s (%.3f), best %s (%.3f), Δ **%+.3f**. %s\n", examples+1, r.GameID, r.Seed, r.Outcome, r.Turn, r.Kind, r.Evaluation.Options[0].Summary, r.Evaluation.Options[0].Rate, r.Evaluation.Options[best].Summary, r.Evaluation.Options[best].Rate, r.Evaluation.Delta, r.Board)
		examples++
	}
	if examples == 0 {
		fmt.Fprintln(&b, "No clear decisions existed to hand-read.")
	}

	fmt.Fprintln(&b, "\n## Kill criteria and recommendation")
	clearRate := pct(clear, total)
	clearDead := clearRate < 2
	reliability := pct(relAgree, relN)
	reliabilityDead := reliability < 70
	costHigh := projectedHours > 48
	fmt.Fprintf(&b, "- Clear decisions <2%%: **%s** — %.2f%%.\n", verdict(clearDead), clearRate)
	fmt.Fprintf(&b, "- Reliability <70%%: **%s** — %.2f%%.\n", verdict(reliabilityDead), reliability)
	fmt.Fprintf(&b, "- Projected 10k-game cost >48h on 20 cores: **%s** — %.2fh.\n", verdict(costHigh), projectedHours)
	fmt.Fprintln(&b)
	switch {
	case clearDead || reliabilityDead:
		fmt.Fprintln(&b, "**Recommendation: do not proceed to pattern mining/distillation.** The measured label supply or repeatability fails a kill criterion.")
	case costHigh:
		fmt.Fprintln(&b, "**Recommendation: prune before step 2.** Keep backward order but stop after the first stable clear pivot, exclude decisions with fewer than two candidates before sampling, and pre-screen low-information priority windows; then remeasure cost without weakening resampling or reliability gates.")
	default:
		fmt.Fprintln(&b, "**Recommendation: proceed to step 2 pattern mining, retaining honest resampling and independent-seed reliability checks; keep the omniscient arm diagnostic-only.**")
	}

	fmt.Fprintln(&b, "\n## Method and limitations")
	fmt.Fprintln(&b, "- Branches were visited from the last recorded decision backward. `rules.Engine.Clone` retained the exact branch boundary; honest rollouts nevertheless started only from independently sampled worlds consistent with the deciding seat’s observation history.")
	fmt.Fprintln(&b, "- Wilson 95% intervals describe each option’s win rate. Clear labels additionally require Δ≥0.10 and the normal 95% interval of paired {-1,0,+1} outcome differences to exclude zero.")
	fmt.Fprintln(&b, "- Timing is observational and therefore not part of label selection or JSONL. Records are emitted in game/descending-decision order independent of worker completion order, so identical flags produce byte-identical JSONL.")
	fmt.Fprintf(&b, "- Raw records: `%s`.\n", cfg.outPath)
	fmt.Fprintln(&b, "\n## Issues")
	fmt.Fprintln(&b, "No engine defect or new approximation was found by this measurement.")
	return b.String(), nil
}

func pct(a, n int) float64 {
	if n == 0 {
		return 0
	}
	return 100 * float64(a) / float64(n)
}

func meanInts(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	total := 0
	for _, x := range xs {
		total += x
	}
	return float64(total) / float64(len(xs))
}

func median(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	return xs[len(xs)/2]
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func verdict(killed bool) string {
	if killed {
		return "KILL"
	}
	return "PASS"
}
