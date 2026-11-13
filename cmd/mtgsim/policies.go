package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
)

// usageError is a bad flag value: main exits 2 for it, as flag does for
// an unknown flag.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// seatPolicies is each seat's bot policy, one entry per seat, "" for the
// default heuristic seat. list is -policies split on commas (nil when the
// flag is absent: -a and -b then name seats 0 and 1). An entry of "-" or ""
// keeps that seat's default. A list longer than seats, or a name the bots
// registry does not know, is a usage error.
func seatPolicies(list []string, a, b string, seats int) ([]string, error) {
	if list == nil {
		list = []string{a, b}
		if seats < len(list) {
			list = list[:seats]
		}
	} else if len(list) > seats {
		return nil, usageError{fmt.Sprintf("-policies names %d seats, but -seats is %d", len(list), seats)}
	}
	out := make([]string, seats)
	for i, p := range list {
		p = strings.TrimSpace(p)
		if p == "-" {
			p = ""
		}
		if p != "" {
			if _, ok := bots.Lookup(p); !ok {
				return nil, usageError{fmt.Sprintf("seat %d: unknown bot policy %q (known: %s)", i, p, strings.Join(bots.Names(), ", "))}
			}
		}
		out[i] = p
	}
	return out, nil
}

// policyList is the summary line's policies field: each seat's policy in
// seat order, "-" for a default heuristic seat.
func policyList(policies []string) string {
	out := make([]string, len(policies))
	for i, p := range policies {
		if p == "" {
			p = "-"
		}
		out[i] = p
	}
	return strings.Join(out, ",")
}

// searchTally is one game's search cost per seat, from azmcts.Watch: the
// evidence that a search seat searched rather than playing its wrapped bot.
type searchTally struct {
	prev   func(azmcts.Diag)
	bySeat map[uint64]int // a named seat's bot seed -> its seat
	seats  []searchSeatTally
	on     bool
}

type searchSeatTally struct {
	decisions, searched, sims, completed, nodes, fallback, refused int
}

// watchSearch installs azmcts.Watch for one game. A named policy's seat i
// is built with seed+i (matchSeats), which is the seed an az seat reports
// in its Diag. A game with no named policy installs nothing.
func watchSearch(seed uint64, policies []string) *searchTally {
	t := &searchTally{bySeat: map[uint64]int{}, seats: make([]searchSeatTally, len(policies))}
	for i, p := range policies {
		if p != "" {
			t.bySeat[seed+uint64(i)] = i
		}
	}
	if len(t.bySeat) == 0 {
		return t
	}
	t.prev, t.on = azmcts.Watch, true
	azmcts.Watch = func(d azmcts.Diag) {
		i, ok := t.bySeat[d.Seed]
		if !ok {
			return
		}
		s := &t.seats[i]
		s.decisions++
		if d.Searched {
			s.searched++
		}
		s.sims += d.Stats.Simulations
		s.completed += d.Stats.Completed
		s.nodes += d.Stats.Expanded
		s.fallback += d.Stats.FeedStopped
		s.refused += d.Stats.RootRefused
	}
	return t
}

// stop restores the previous Watch.
func (t *searchTally) stop() {
	if t.on {
		azmcts.Watch, t.on = t.prev, false
	}
}

// print writes one line per seat that reported a searched-kind decision:
// fallback counts the decisions the search was routed around (a stopped
// feed or a refused honest root, azmcts.Stats.FeedStopped), root-refused
// the refused-root share of them.
func (t *searchTally) print(out io.Writer, policies []string) {
	for i, s := range t.seats {
		if s.decisions == 0 {
			continue
		}
		fmt.Fprintf(out, "  search seat %d (%s): decisions=%d searched=%d sims=%d completed=%d nodes=%d fallback=%d root-refused=%d\n",
			i, policies[i], s.decisions, s.searched, s.sims, s.completed, s.nodes, s.fallback, s.refused)
	}
}
