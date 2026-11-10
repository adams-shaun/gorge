package oracle

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
)

// RunConfig bounds one lockstep game.
type RunConfig struct {
	Config
	MaxDecisions int   // canonical decisions; 0 = 20000
	MaxTurns     int32 // 0 = 100; a capped game is a stall
	// Check, when set, runs at every decision both sides agree on (after
	// the projection and option comparisons) and may report a divergence
	// (FeatCheck: the network inputs).
	Check func(ref *GorgeSide, k Side) *Divergence `json:"-"`
	// Finish, when set, runs once at the end of every game a gorge side
	// took part in (diverged or not, after the panic recover), with that
	// gorge side (CoverageOf walks its finished log).
	Finish func(g *GorgeSide) `json:"-"`
}

func (rc RunConfig) caps() (int, int32) {
	n, t := rc.MaxDecisions, rc.MaxTurns
	if n <= 0 {
		n = 20000
	}
	if t <= 0 {
		t = 100
	}
	return n, t
}

// Result is one lockstep game.
type Result struct {
	Decisions int     // canonical decisions both sides posed
	Actions   []Canon // the canonical actions applied, in order
	Over      bool
	Winner    int8
	Stall     string // "decisions" or "turns" when a cap ended the game; KernelStallPrefix+reason when the kernel side stopped (ProjectionStall: the instances outgrew the projection); GorgeTriggerCapStall when gorge's trigger bound cut the game
	// StallUnmodelled: the kernel stopped because gorge does something here it does
	// not model (dirac.StallClassOf), not because a capacity ran out.
	StallUnmodelled bool
	Turns           int32
	Div             *Divergence
	Stats           Stats // the oracle side's
}

// Clean reports whether the game counts as evidence: no divergence and no
// unmodelled stall. A game that reached -max-turns or -max-decisions agreed for its
// whole length, and one the kernel stopped at a declared capacity agreed up to that
// point, where gorge has no such limit, so both stay clean. A verdict still counts
// the capacity stall as its own outcome.
func (r *Result) Clean() bool {
	return r.Div == nil && !r.StallUnmodelled
}

// GorgeTriggerCapStall is Result.Stall for a game gorge's hang guard cut short: a trigger
// reached rules' maxTriggerFires and stopped matching, which the kernel does not count,
// so the two disagree from there on. The bound is an engine limit, not a rule, so the
// game is compared up to the decision where the difference appeared and no further. It
// is a declared limit, counted and printed like a capacity stall, and the game is Clean.
const GorgeTriggerCapStall = "gorge trigger cap"

// KernelStallPrefix starts Result.Stall when the kernel side ended the
// game at its fixed capacity ("kernel stack", "kernel board", ...).
const KernelStallPrefix = "kernel "

// ProjectionStall is Result.Stall for a game whose card instances outgrew the
// projection (MaxInst, the canonical Ref space): the instances past it would be
// compared on neither side, so the game ends there. It is a declared capacity,
// counted and reported like the kernel's, never a pass and never a divergence.
const ProjectionStall = KernelStallPrefix + "projection"

// capacityStaller is a kernel Side that can end a game at its capacity.
type capacityStaller interface {
	CapacityStall() string // "" unless the game was stopped at capacity
}

// unmodelledStaller is a capacityStaller that can tell a rule gap from a capacity.
type unmodelledStaller interface {
	StallIsUnmodelled() bool
}

// noteKernelStall records s's stall in res and reports whether there was one.
func noteKernelStall(s Side, res *Result) bool {
	cs, ok := s.(capacityStaller)
	if !ok {
		return false
	}
	why := cs.CapacityStall()
	if why == "" {
		return false
	}
	res.Stall = KernelStallPrefix + why
	if u, ok := s.(unmodelledStaller); ok {
		res.StallUnmodelled = u.StallIsUnmodelled()
	}
	return true
}

// Divergence is the first point where the two sides disagree.
type Divergence struct {
	// At is the canonical decision number: Actions[:At] were applied.
	At int `json:"at"`
	// What is "options", "projection", "decision", "terminal", "gorge"
	// (the oracle side failed) or "kernel" (the kernel panicked).
	What string `json:"what"`
	// Field is the first mismatching field ("Cards[12].Zone", "Life[1]",
	// "options", "kind"); Diff lists every mismatching field with both values.
	Field string   `json:"field"`
	Diff  []string `json:"diff,omitempty"`
	// RefOptions and KerOptions are both sides' option lists at At.
	RefOptions []string `json:"ref_options,omitempty"`
	KerOptions []string `json:"ker_options,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	// Lead names the side that led the run ("gorge" or "kernel").
	Lead string `json:"lead,omitempty"`
	// LastSourced is the latest applied action whose source is an object, as
	// Canon.String prints it ("" when none was): Implicates' last resort. A
	// pass has no source, so it reaches back past the passes to the play.
	LastSourced string `json:"last_sourced,omitempty"`
}

func (d *Divergence) String() string {
	return fmt.Sprintf("divergence at decision %d: %s %s %s", d.At, d.What, d.Field, d.Detail)
}

var (
	objectRef = regexp.MustCompile(`#(\d+)`)
	cardField = regexp.MustCompile(`^Cards\[(\d+)\]\.(\w+)`)
)

// Implicates names the cards a divergence points at, in order of suspicion and
// without repeats: the cards named in the options only one side offers; else
// the card of Diff's first Cards[] field, passing over Pos, which names a victim
// of a library reorder rather than its cause; else the source of the latest
// sourced action. Tokens and copies name no card, so the result can be empty.
func (d *Divergence) Implicates(m *Matchup) []string {
	if m == nil {
		return nil
	}
	if d.What == "options" {
		var out []string
		seen := map[string]bool{}
		for _, o := range symmetricDiff(d.RefOptions, d.KerOptions) {
			if n := firstObjectCard(m, o); n != "" && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	for _, l := range d.Diff {
		f := cardField.FindStringSubmatch(l)
		if f == nil || f[2] == "Pos" {
			continue
		}
		o, _ := strconv.Atoi(f[1])
		if n := m.InstanceCard(o); n != "" {
			return []string{n}
		}
		break // the first card field is a token's: nothing further is the cause
	}
	if n := firstObjectCard(m, d.LastSourced); n != "" {
		return []string{n}
	}
	return nil
}

// RecentSources is the cards of the game's latest sourced actions, newest first and
// without repeats, at most n. A stall has no differing options to name, and a
// divergence on a token or copy names no card, so this is what points at a cause there.
func (r *Result) RecentSources(m *Matchup, n int) []string {
	if m == nil {
		return nil
	}
	var out []string
	for i := len(r.Actions) - 1; i >= 0 && len(out) < n; i-- {
		if src := r.Actions[i].Src(); src != RefNone && src < RefStack {
			if c := firstObjectCard(m, r.Actions[i].String()); c != "" && !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	return out
}

// firstObjectCard is the card of the first object an action string names.
func firstObjectCard(m *Matchup, action string) string {
	r := objectRef.FindStringSubmatch(action)
	if r == nil {
		return ""
	}
	o, _ := strconv.Atoi(r[1])
	return m.InstanceCard(o)
}

// symmetricDiff is the options on exactly one list, a's first, each in list order.
func symmetricDiff(a, b []string) []string {
	var out []string
	for _, x := range [][2][]string{{a, b}, {b, a}} {
		for _, o := range x[0] {
			if !slices.Contains(x[1], o) {
				out = append(out, o)
			}
		}
	}
	return out
}

// fieldClass is a divergence's comparable class: What plus the field with
// its indices stripped ("projection Cards[].Zone").
func (d *Divergence) fieldClass() string {
	var b strings.Builder
	skip := false
	for _, r := range d.Field {
		switch {
		case r == '[':
			skip = true
			b.WriteRune(r)
		case r == ']':
			skip = false
			b.WriteRune(r)
		case !skip:
			b.WriteRune(r)
		}
	}
	return d.What + " " + b.String()
}

// InstanceCard names game object o, the index a Canon's #n and a projection's
// Cards[n] share: the two decks in order, card by card, then the two sideboards. A token or copy is past
// them and names no card ("").
func (m *Matchup) InstanceCard(o int) string {
	if i := m.instanceTable(o); i >= 0 && i < len(m.Table) {
		return m.Table[i]
	}
	return ""
}

// instanceTable is the table index of card instance o: the two decks in
// order, then the two sideboards; -1 for a token or copy, past them.
func (m *Matchup) instanceTable(o int) int {
	if o < 0 {
		return -1
	}
	for s := 0; s < m.NSeat(); s++ {
		d := m.seatIx(s)
		if o < len(d) {
			return int(d[o])
		}
		o -= len(d)
	}
	for s := range m.Side {
		if o < len(m.Side[s]) {
			return int(m.Side[s][o])
		}
		o -= len(m.Side[s])
	}
	return -1
}

// Lockstep plays one game of m on seed with gorge (the oracle) leading and k
// (a kernel through KernelSide, or another GorgeSide) following, under pol:
// at every canonical decision both option lists must be equal; after every
// action the leader committed, both projections must be equal at the next
// decision; at the end both terminal states must agree. It is LockstepLed
// with a fresh gorge side as the leader.
func Lockstep(m *Matchup, seed uint64, k Side, pol Policy, rc RunConfig) Result {
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		return Result{Winner: -1, Div: &Divergence{What: "gorge", Field: "setup", Detail: err.Error(), Lead: "gorge"}}
	}
	return LockstepLed(m, seed, lead, k, pol, rc)
}

// LockstepLed plays one game of m on seed with lead and follow in lockstep
// under pol. The leader poses the canonical decision the policy answers
// (PickCtx.D and PickCtx.Seq); the follower mirrors the action. PickCtx.G is
// the gorge side only when gorge leads. Either side can lead: a gorge-led
// run is byte-identical to Lockstep's.
func LockstepLed(m *Matchup, seed uint64, lead, follow Side, pol Policy, rc RunConfig) (res Result) {
	maxN, maxT := rc.caps()
	res.Winner = -1
	// Result.Stats is the gorge side's, whichever role it plays.
	var gorge *GorgeSide
	if g, ok := lead.(*GorgeSide); ok {
		gorge = g
	} else if g, ok := follow.(*GorgeSide); ok {
		gorge = g
	}
	if gorge != nil {
		// Consulted only once a difference is found, so the transition loop pays nothing.
		defer func() {
			if res.Div != nil && gorge.E != nil && gorge.E.TriggerCapReached() {
				res.Div, res.Stall = nil, GorgeTriggerCapStall
			}
		}()
	}
	defer func() {
		if res.Div != nil {
			res.Div.Lead = sideName(lead)
			res.Div.LastSourced = lastSourced(res.Actions)
		}
	}()
	if gorge != nil {
		defer func() { res.Stats = gorge.Stats }()
		if rc.Finish != nil {
			defer rc.Finish(gorge)
		}
	}
	var inSide Side
	defer func() {
		if r := recover(); r != nil {
			what := "policy"
			if inSide != nil {
				what = sideName(inSide)
			}
			res.Div = &Divergence{At: res.Decisions, What: what, Field: "panic", Detail: fmt.Sprintf("%v\n%s", r, debug.Stack())}
		}
	}()
	var pr, pk Projection
	needCmp := true
	for {
		inSide = lead
		dl := lead.Step()
		inSide = nil
		if e := lead.Err(); e != nil {
			res.Div = &Divergence{At: res.Decisions, What: sideName(lead), Field: "step", Detail: e.Error()}
			return res
		}
		inSide = follow
		df := follow.Step()
		inSide = nil
		// The kernel side stopped (State.StallReason): a stall, not a divergence.
		// A capacity is a declared limit; an unmodelled reason is a rule gap that
		// Result.StallUnmodelled carries to whoever is counting.
		if df.Kind == DNone && noteKernelStall(follow, &res) {
			res.Turns = oracleTurn(lead, follow)
			return res
		}
		if dl.Kind == DNone && noteKernelStall(lead, &res) {
			res.Turns = oracleTurn(lead, follow)
			return res
		}
		// ref is the gorge side, ker the kernel side (the DiffProjection
		// labels and the RefOptions/KerOptions fields keep that meaning in
		// either direction); dr/dk are the same decisions in that order.
		ref, ker, leadIsRef := splitSides(lead, follow)
		dr, dk := dl, df
		if !leadIsRef {
			dr, dk = df, dl
		}
		// gorge keeps asking while the kernel's game is already over (a
		// player lost, gorge removing the loser's objects before it ends
		// the game): the boards are not compared; the extra ask is gorge's.
		// This is keyed on the engines, not their roles: in a kernel-led run
		// the kernel leader is over while the gorge follower still asks.
		if dk.Kind == DNone && dr.Kind != DNone {
			inSide = ker
			ko, kw := ker.Terminal()
			inSide = ref
			ro, rw := ref.Decided()
			inSide = nil
			if ko && ro && rw == kw {
				res.Over, res.Winner, res.Turns = ro, rw, oracleTurn(lead, follow)
				res.Div = &Divergence{At: res.Decisions, What: "terminal", Field: sideName(ref) + " asks after the loss",
					Detail: fmt.Sprintf("%v decision for p%d after the game was decided, winner %d", dr.Kind, dr.Player, rw)}
				return res
			}
		}
		// The gorge side's commit boundaries are the comparison cadence in
		// either direction: a partial multi-pick answer moves the kernel's
		// state at once but gorge only commits (and marks dirty) at Done or
		// Max picks, so the kernel leader's Dirty would compare mid-answer.
		if needCmp || ref.Dirty() {
			inSide = ref
			ref.Project(&pr)
			inSide = ker
			ker.Project(&pk)
			inSide = nil
			if gorge != nil && gorge.id.err != "" {
				res.Stall, res.Turns = ProjectionStall, oracleTurn(lead, follow)
				return res
			}
			if gorge != nil && gorge.CharErr != "" {
				res.Div = &Divergence{At: res.Decisions, What: "projection", Field: "chars-map", Detail: gorge.CharErr}
				return res
			}
			// The priority holder is compared through the decision's player
			// below; outside a priority decision (declarations, choices) the
			// engines' stale holder values are not game state.
			pr.Priority, pk.Priority = 0, 0
			if pr.Over && pk.Over && pr.Winner == pk.Winner && pr.Life == pk.Life {
				pr = pk // local patch: a finished game compares by result only
			}
			if pr != pk {
				diff := DiffProjection(&pr, &pk)
				if os.Getenv("KFEAT_DEBUG") != "" {
					fmt.Printf("  at the projection divergence: ref %v p%d %v, kernel %v p%d %v\n", dr.Kind, dr.Player, actionStrings(dr.Options), dk.Kind, dk.Player, actionStrings(dk.Options))
				}
				res.Div = &Divergence{At: res.Decisions, What: "projection", Field: firstField(diff), Diff: diff}
				return res
			}
		}
		ro, ko := dr.Options, dk.Options
		if dr.Kind != dk.Kind || (dr.Kind != DNone && dr.Player != dk.Player) {
			res.Div = &Divergence{At: res.Decisions, What: "decision", Field: "kind",
				Detail:     fmt.Sprintf("ref %v p%d, kernel %v p%d", dr.Kind, dr.Player, dk.Kind, dk.Player),
				RefOptions: actionStrings(ro), KerOptions: actionStrings(ko)}
			return res
		}
		if !equalActions(ro, ko) {
			if os.Getenv("KFEAT_DEBUG") != "" {
				if g, ok := ref.(*GorgeSide); ok {
					optionsDebug(g)
				}
			}
			res.Div = &Divergence{At: res.Decisions, What: "options", Field: "options",
				Detail: fmt.Sprintf("%v decision for p%d", dr.Kind, dr.Player), RefOptions: actionStrings(ro), KerOptions: actionStrings(ko)}
			return res
		}
		if dr.Kind != DNone && rc.Check != nil {
			if g, ok := ref.(*GorgeSide); ok {
				inSide = ref
				if d := rc.Check(g, ker); d != nil {
					inSide = nil
					d.At = res.Decisions
					res.Div = d
					return res
				}
				inSide = nil
			}
		}
		if dr.Kind == DNone {
			inSide = ref
			ro, rw := ref.Terminal()
			inSide = ker
			ko, kw := ker.Terminal()
			inSide = nil
			res.Over, res.Winner, res.Turns = ro, rw, oracleTurn(lead, follow)
			if ro != ko || rw != kw {
				res.Div = &Divergence{At: res.Decisions, What: "terminal", Field: "terminal",
					Detail: fmt.Sprintf("ref over=%v winner=%d, kernel over=%v winner=%d", ro, rw, ko, kw)}
			}
			return res
		}
		if res.Decisions >= maxN {
			res.Stall, res.Turns = "decisions", oracleTurn(lead, follow)
			return res
		}
		if lead.Turn() >= int(maxT) {
			res.Stall, res.Turns = "turns", oracleTurn(lead, follow)
			return res
		}
		// The policy answers the leader's decision; G is the gorge side only
		// when gorge leads (HeuristicPolicy needs it).
		var pg *GorgeSide
		if leadIsRef {
			pg, _ = lead.(*GorgeSide)
		}
		a := pol.Pick(&PickCtx{D: &dl, G: pg, Seq: res.Decisions})
		res.Actions = append(res.Actions, a)
		res.Decisions++
		inSide = lead
		lead.Apply(a)
		inSide = nil
		if e := lead.Err(); e != nil {
			res.Div = &Divergence{At: res.Decisions - 1, What: sideName(lead), Field: "apply", Detail: e.Error()}
			return res
		}
		inSide = follow
		follow.Apply(a)
		inSide = nil
		if e := follow.Err(); e != nil {
			res.Div = &Divergence{At: res.Decisions - 1, What: sideName(follow), Field: "apply", Detail: e.Error()}
			return res
		}
		needCmp = false
	}
}

// sideName names a side for a divergence: the gorge side, or the kernel.
// A side may name itself (a test wrapper, say) through SideName.
func sideName(s Side) string {
	if n, ok := s.(interface{ SideName() string }); ok {
		return n.SideName()
	}
	if _, ok := s.(*GorgeSide); ok {
		return "gorge"
	}
	return "kernel"
}

// splitSides orders two sides as (gorge, kernel) and reports whether the
// leader is the gorge side. With no gorge side, the leader is the reference.
func splitSides(lead, follow Side) (ref, ker Side, leadIsRef bool) {
	if sideName(lead) == "gorge" {
		return lead, follow, true
	}
	if sideName(follow) == "gorge" {
		return follow, lead, false
	}
	return lead, follow, true
}

// oracleTurn is the gorge side's turn (the reported turn in either
// direction); the leader's turn when neither side is gorge.
func oracleTurn(lead, follow Side) int32 {
	if _, ok := lead.(*GorgeSide); ok {
		return int32(lead.Turn())
	}
	if _, ok := follow.(*GorgeSide); ok {
		return int32(follow.Turn())
	}
	return int32(lead.Turn())
}

func equalActions(a, b []Canon) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// lastSourced prints the latest action whose source is an object, "" if none.
func lastSourced(as []Canon) string {
	for i := len(as) - 1; i >= 0; i-- {
		if src := as[i].Src(); src != RefNone && src < RefStack {
			return as[i].String()
		}
	}
	return ""
}

func actionStrings(as []Canon) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.String()
	}
	return out
}

func firstField(diff []string) string {
	if len(diff) == 0 {
		return ""
	}
	f, _, _ := strings.Cut(diff[0], ":")
	return f
}

// DiffProjection lists every field where a (gorge) and b (kernel) differ,
// as "path: ref=<a> kernel=<b>", in field order.
func DiffProjection(a, b *Projection) []string {
	var out []string
	diffValue("", reflect.ValueOf(*a), reflect.ValueOf(*b), &out)
	return out
}

func diffValue(path string, a, b reflect.Value, out *[]string) {
	switch a.Kind() {
	case reflect.Struct:
		t := a.Type()
		for i := 0; i < a.NumField(); i++ {
			p := t.Field(i).Name
			if path != "" {
				p = path + "." + p
			}
			diffValue(p, a.Field(i), b.Field(i), out)
		}
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			diffValue(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i), out)
		}
	default:
		if a.Interface() != b.Interface() {
			*out = append(*out, fmt.Sprintf("%s: ref=%v kernel=%v", path, a.Interface(), b.Interface()))
		}
	}
}
