package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// searchSeatFlags parses -search-seats, -search and -search-sims. It returns
// a one-line reason when the flags cannot be used with this run: search seats
// play pool runs (-kernel a registered pool or -synth) with gorge leading.
// notPool: the run is a -repro, -replay or -v2 run.
func searchSeatFlags(list, kind string, sims, seats int, kernel, lead string, notPool bool) (oracle.SearchSeatConfig, string) {
	var cfg oracle.SearchSeatConfig
	switch {
	case list == "":
		return cfg, "-search and -search-sims need -search-seats"
	case kernel != "fdn":
		return cfg, fmt.Sprintf("-search-seats plays pool runs (-kernel a registered pool, or -synth), not -kernel %s", kernel)
	case lead != "gorge":
		return cfg, "-search-seats needs -lead gorge (the policy answers gorge's pending decision)"
	case notPool:
		return cfg, "-search-seats plays pool runs, not -repro, -replay or -v2"
	}
	switch kind {
	case "paranoid":
	case "maxn":
		cfg.MaxN = true
	default:
		return cfg, fmt.Sprintf("-search %q: want paranoid or maxn", kind)
	}
	if sims < 1 {
		return cfg, fmt.Sprintf("-search-sims %d: want at least 1", sims)
	}
	cfg.Sims = sims
	for _, f := range strings.Split(list, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 0 || n >= seats {
			return cfg, fmt.Sprintf("-search-seats %q: want seat numbers from 0 to %d, comma-separated", list, seats-1)
		}
		cfg.Seats |= 1 << n
	}
	return cfg, ""
}

// searchSeatsHeader is the run header's line for the search seats.
func searchSeatsHeader(cfg oracle.SearchSeatConfig) string {
	var seats []string
	for s := 0; s < oracle.MaxSeats; s++ {
		if cfg.Seats>>s&1 != 0 {
			seats = append(seats, strconv.Itoa(s))
		}
	}
	kind := "paranoid"
	if cfg.MaxN {
		kind = "maxn"
	}
	return fmt.Sprintf("search seats: %s %s sims=%d", strings.Join(seats, ","), kind, cfg.Sims)
}
