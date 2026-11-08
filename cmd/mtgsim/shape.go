package main

import (
	"encoding/json"
	"io"
	"sort"
	"strconv"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// shapeGame accumulates the shape of one game: the intents the drive loop
// submitted, the option count of every decision by Kind, the largest value
// of each table the game reached, and the end-of-game token and commander
// tallies. observe is called once per decision before the seat is asked;
// finish is called once when the game stops, whatever its status.
type shapeGame struct {
	seed           uint64
	decks          []string
	decisions      int
	turns          int
	status         string
	options        map[decision.Kind][]int
	peak           shapePeak
	tokens         int
	tokenEvents    int
	commanderCasts int
}

// shapePeak is one game's largest value for each table a dirac State would
// hold. Each field is the maximum over the game's decisions.
type shapePeak struct {
	Objects     int `json:"objects"`
	LiveObjects int `json:"live_objects"`
	Permanents  int `json:"permanents"`
	Stack       int `json:"stack"`
	Pending     int `json:"pending"`
}

// shapeStat is the mean/p95/max summary of one integer series.
type shapeStat struct {
	Mean float64 `json:"mean"`
	P95  float64 `json:"p95"`
	Max  float64 `json:"max"`
}

// shapePair is the p95/max summary of the per-game peaks for one table.
type shapePair struct {
	P95 float64 `json:"p95"`
	Max float64 `json:"max"`
}

// shapeCount is the option-count summary for one decision Kind: the number
// of decisions, their mean/p95/max option count, and a histogram from each
// option count to how many decisions had it.
type shapeCount struct {
	N    int            `json:"n"`
	Mean float64        `json:"mean"`
	P95  float64        `json:"p95"`
	Max  float64        `json:"max"`
	Hist map[string]int `json:"hist"`
}

// shapeGameRow is the JSON object written per game.
type shapeGameRow struct {
	Row            string                 `json:"row"`
	Kind           string                 `json:"kind"`
	Seed           uint64                 `json:"seed"`
	Decks          []string               `json:"decks"`
	Status         string                 `json:"status"`
	Turns          int                    `json:"turns"`
	Decisions      int                    `json:"decisions"`
	Options        map[string]*shapeCount `json:"options"`
	Peak           shapePeak              `json:"peak"`
	Tokens         int                    `json:"tokens"`
	TokenEvents    int                    `json:"token_events"`
	CommanderCasts int                    `json:"commander_casts"`
}

// shapeAggregateRow is the JSON object written once per run.
type shapeAggregateRow struct {
	Row            string                 `json:"row"`
	Kind           string                 `json:"kind"`
	Games          int                    `json:"games"`
	Statuses       map[string]int         `json:"statuses"`
	Decisions      shapeStat              `json:"decisions"`
	Turns          shapeStat              `json:"turns"`
	Options        map[string]*shapeCount `json:"options"`
	Peak           map[string]*shapePair  `json:"peak"`
	Tokens         shapeStat              `json:"tokens"`
	TokenEvents    shapeStat              `json:"token_events"`
	CommanderCasts shapeStat              `json:"commander_casts"`
}

// observe records one decision: its Kind and option count, and the current
// size of every table. It only reads the engine; it never submits, advances
// or otherwise changes the game.
func (g *shapeGame) observe(e *rules.Engine, d *decision.Decision) {
	if g.options == nil {
		g.options = make(map[decision.Kind][]int)
	}
	g.options[d.Kind] = append(g.options[d.Kind], len(d.Options))

	live := 0
	for i := range e.G.Objs {
		if e.G.Objs[i].Zone != state.ZCeased {
			live++
		}
	}
	permanents := 0
	for p := range e.G.Players {
		permanents += len(e.G.Zone(state.ZBattlefield, state.PlayerID(p)))
	}
	g.peak.Objects = max(g.peak.Objects, len(e.G.Objs))
	g.peak.LiveObjects = max(g.peak.LiveObjects, live)
	g.peak.Permanents = max(g.peak.Permanents, permanents)
	g.peak.Stack = max(g.peak.Stack, len(e.G.Stack))
	g.peak.Pending = max(g.peak.Pending, len(e.PendingTriggers()))
}

// finish records the game's final status and counts. n is the number of
// intents driveMatch submitted, the same number the summary line prints.
func (g *shapeGame) finish(e *rules.Engine, n int, status string) {
	g.decisions = n
	g.turns = int(e.G.Turn)
	g.status = status

	for i := range e.G.Objs {
		if e.G.Objs[i].IsToken {
			g.tokens++
		}
	}
	for i := range e.L.Events {
		switch e.L.Events[i].Kind {
		case events.TokenCreate, events.CardToken, events.CopyToken:
			g.tokenEvents++
		}
	}
	g.commanderCasts = 0
	for p := range e.G.Players {
		for _, c := range e.G.Players[p].CmdCasts {
			g.commanderCasts += int(c)
		}
	}
}

// shapeRun collects the games of one run and writes its game and aggregate
// rows.
type shapeRun struct {
	kind  string
	games []*shapeGame
}

// writeGame writes one per-game JSON line.
func (r *shapeRun) writeGame(w io.Writer, g *shapeGame) error {
	row := shapeGameRow{
		Row:            "game",
		Kind:           r.kind,
		Seed:           g.seed,
		Decks:          g.decks,
		Status:         g.status,
		Turns:          g.turns,
		Decisions:      g.decisions,
		Options:        optionCounts(g.options),
		Peak:           g.peak,
		Tokens:         g.tokens,
		TokenEvents:    g.tokenEvents,
		CommanderCasts: g.commanderCasts,
	}
	return json.NewEncoder(w).Encode(row)
}

// writeAggregate writes the run's single aggregate JSON line.
func (r *shapeRun) writeAggregate(w io.Writer) error {
	row := shapeAggregateRow{
		Row:            "aggregate",
		Kind:           r.kind,
		Games:          len(r.games),
		Statuses:       make(map[string]int),
		Options:        make(map[string]*shapeCount),
		Peak:           make(map[string]*shapePair),
		Decisions:      shapeStat{},
		Turns:          shapeStat{},
		Tokens:         shapeStat{},
		TokenEvents:    shapeStat{},
		CommanderCasts: shapeStat{},
	}
	decisions := make([]int, 0, len(r.games))
	turns := make([]int, 0, len(r.games))
	tokens := make([]int, 0, len(r.games))
	tokenEvents := make([]int, 0, len(r.games))
	commanderCasts := make([]int, 0, len(r.games))
	objects := make([]int, 0, len(r.games))
	liveObjects := make([]int, 0, len(r.games))
	permanents := make([]int, 0, len(r.games))
	stack := make([]int, 0, len(r.games))
	pending := make([]int, 0, len(r.games))
	byKind := make(map[decision.Kind][]int)

	for _, g := range r.games {
		row.Statuses[g.status]++
		decisions = append(decisions, g.decisions)
		turns = append(turns, g.turns)
		tokens = append(tokens, g.tokens)
		tokenEvents = append(tokenEvents, g.tokenEvents)
		commanderCasts = append(commanderCasts, g.commanderCasts)
		objects = append(objects, g.peak.Objects)
		liveObjects = append(liveObjects, g.peak.LiveObjects)
		permanents = append(permanents, g.peak.Permanents)
		stack = append(stack, g.peak.Stack)
		pending = append(pending, g.peak.Pending)
		for kind, n := range g.options {
			byKind[kind] = append(byKind[kind], n...)
		}
	}

	row.Decisions = statOf(decisions)
	row.Turns = statOf(turns)
	row.Tokens = statOf(tokens)
	row.TokenEvents = statOf(tokenEvents)
	row.CommanderCasts = statOf(commanderCasts)
	row.Peak["objects"] = pairOf(objects)
	row.Peak["live_objects"] = pairOf(liveObjects)
	row.Peak["permanents"] = pairOf(permanents)
	row.Peak["stack"] = pairOf(stack)
	row.Peak["pending"] = pairOf(pending)
	for kind, n := range byKind {
		row.Options[string(kind)] = countOf(n)
	}
	return json.NewEncoder(w).Encode(row)
}

// optionCounts summarizes every Kind's option counts for one game.
func optionCounts(byKind map[decision.Kind][]int) map[string]*shapeCount {
	out := make(map[string]*shapeCount, len(byKind))
	for kind, n := range byKind {
		out[string(kind)] = countOf(n)
	}
	return out
}

// countOf summarizes one series of option counts. An empty series writes
// n 0, zero statistics and an empty histogram.
func countOf(xs []int) *shapeCount {
	c := &shapeCount{N: len(xs), Hist: make(map[string]int)}
	if len(xs) == 0 {
		return c
	}
	sum := 0
	m := xs[0]
	for _, x := range xs {
		sum += x
		if x > m {
			m = x
		}
		c.Hist[strconv.Itoa(x)]++
	}
	c.Mean = float64(sum) / float64(len(xs))
	c.P95 = p95Of(xs)
	c.Max = float64(m)
	return c
}

// statOf summarizes one series with mean, nearest-rank p95 and max. An
// empty series writes zeros.
func statOf(xs []int) shapeStat {
	if len(xs) == 0 {
		return shapeStat{}
	}
	sum := 0
	m := xs[0]
	for _, x := range xs {
		sum += x
		if x > m {
			m = x
		}
	}
	return shapeStat{Mean: float64(sum) / float64(len(xs)), P95: p95Of(xs), Max: float64(m)}
}

// pairOf summarizes a per-game peak series with its p95 and max.
func pairOf(xs []int) *shapePair {
	if len(xs) == 0 {
		return &shapePair{}
	}
	m := xs[0]
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return &shapePair{P95: p95Of(xs), Max: float64(m)}
}

// p95Of is the nearest-rank 95th percentile: sort ascending and take index
// ceil(0.95*n)-1. The integer form ceil(95*n/100) keeps it exact for every
// length without a floating-point round trip.
func p95Of(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := append([]int(nil), xs...)
	sort.Ints(sorted)
	i := (95*len(sorted)+99)/100 - 1
	if i < 0 {
		i = 0
	}
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return float64(sorted[i])
}
