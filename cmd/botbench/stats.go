package main

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"text/tabwriter"

	"github.com/adams-shaun/gorge/decision"
)

// decisionStat is one measured row of the -decision-stats histogram: how
// often one (kind, sub-kind) decision was asked across a run, and three
// per-instance signals the leverage analysis reads off it.
//
//   - count / sumOpts feed the count and mean-option-count columns.
//   - single is the share of instances where the decision offered exactly
//     one legal option -- an answer to a one-option decision carries no
//     information (the only legal move is forced), so a kind that is almost
//     always a singleton is not a leverage point no matter how often it is
//     asked.
//   - first is the share of instances whose chosen answer was the first
//     offered option (in.Choices[0] == d.Options[0].Index). This is the
//     flatness signal: a policy that answers Options[0] almost always is
//     not ranking anything, it is just picking the first thing it is shown.
type decisionStat struct {
	count   int64
	sumOpts int64
	single  int64
	first   int64
}

// decisionStats accumulates the histogram across a whole bench run. Games
// are played in parallel (benchWithPool / playOnePairWithPool), so every
// mutating method takes the mutex; keys are never ranged over for output
// in map order -- write() sorts them -- so the report is deterministic
// regardless of how the goroutines interleave. It is pure observation: it
// reads only the Decision the engine offered and the Intent the seat
// returned, and never calls into the policy, the engine or any rng source,
// so enabling it cannot perturb what the bot decides.
type decisionStats struct {
	mu    sync.Mutex
	games int64
	rows  map[string]*decisionStat
}

func newDecisionStats() *decisionStats {
	return &decisionStats{rows: make(map[string]*decisionStat)}
}

// game counts one game against the run (the mean-per-game denominator).
// Called once per played match, including stalled ones.
func (c *decisionStats) game() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.games++
	c.mu.Unlock()
}

// record observes one decision the engine asked and the answer the seat
// returned. It must run after Decide and before Submit so the Intent is the
// answer that actually reached the engine, and it must not touch anything
// the policy read.
func (c *decisionStats) record(d *decision.Decision, in decision.Intent) {
	if c == nil {
		return
	}
	key := statKey(d, in)
	c.mu.Lock()
	st := c.rows[key]
	if st == nil {
		st = &decisionStat{}
		c.rows[key] = st
	}
	st.count++
	st.sumOpts += int64(len(d.Options))
	if len(d.Options) == 1 {
		st.single++
	}
	if len(in.Choices) > 0 && len(d.Options) > 0 && in.Choices[0] == d.Options[0].Index {
		st.first++
	}
	c.mu.Unlock()
}

// priorityBranch classifies what the policy DID with a KPriority decision by
// the option kind it selected in its answer. botpolicy/policy.go's KPriority
// branch returns (in order) a tap-for-mana "activate" option, a "play_land"
// option, a "cast" option, an "ability" option, then the explicit "pass"
// scan -- so the chosen option's Kind recovers the branch exactly, with no
// hook into botpolicy/ (which the brief forbids touching).
func priorityBranch(d *decision.Decision, in decision.Intent) string {
	if len(in.Choices) == 0 || len(d.Options) == 0 {
		return "other"
	}
	idx := in.Choices[0]
	if idx < 0 || idx >= len(d.Options) {
		return "other"
	}
	switch d.Options[idx].Kind {
	case "activate":
		return "tap"
	case "play_land":
		return "land"
	case "cast":
		return "cast"
	case "ability":
		return "ability"
	case "pass", "concede":
		return "pass"
	default:
		return "other"
	}
}

// statKey returns the histogram row a decision counts into. KPriority is
// broken down by the branch taken and KChoose by the sub-kind (Option.Kind
// of its first option -- x, exile, sacrifice, discard, name, type, number,
// yes/no); every other kind maps to one row named by the kind itself.
func statKey(d *decision.Decision, in decision.Intent) string {
	switch d.Kind {
	case decision.KPriority:
		return "priority/" + priorityBranch(d, in)
	case decision.KChoose:
		if len(d.Options) > 0 {
			return "choose/" + d.Options[0].Kind
		}
		return "choose"
	default:
		return string(d.Kind)
	}
}

type statRow struct {
	key     string
	count   int64
	sumOpts int64
	single  int64
	first   int64
}

// write prints the histogram table to out. Rows are emitted in sorted key
// order, never map-iteration order, so the report is byte-deterministic for
// identical inputs. It snapshots the tallies under the lock and prints after
// (called only once every game has finished, so no goroutine is writing, but
// the snapshot keeps the shape safe regardless).
func (c *decisionStats) write(out io.Writer) {
	if c == nil {
		return
	}
	c.mu.Lock()
	games := c.games
	rows := make([]statRow, 0, len(c.rows))
	for k, st := range c.rows {
		rows = append(rows, statRow{key: k, count: st.count, sumOpts: st.sumOpts, single: st.single, first: st.first})
	}
	c.mu.Unlock()
	if games == 0 || len(rows) == 0 {
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].key < rows[j].key })

	fmt.Fprintf(out, "\ndecision stats (%d games):\n", games)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "kind\tcount\tmean/game\tmean opts\tsingleton%\tfirst-option%")
	for _, r := range rows {
		meanGame := float64(r.count) / float64(games)
		meanOpts := float64(r.sumOpts) / float64(r.count)
		singlePct := float64(r.single) / float64(r.count) * 100
		firstPct := float64(r.first) / float64(r.count) * 100
		fmt.Fprintf(tw, "%s\t%d\t%.2f\t%.2f\t%.1f%%\t%.1f%%\n",
			r.key, r.count, meanGame, meanOpts, singlePct, firstPct)
	}
	tw.Flush()
}
