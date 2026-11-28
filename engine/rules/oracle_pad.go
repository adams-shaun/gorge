package rules

// The pad-to-four relation: a two-seat scenario replayed with two inert seats
// after its own (p2 and p3) must leave everything the two original seats can
// observe unchanged, unless a rule makes the difference correct (an "each
// opponent" effect, a per-turn trigger: testdata/oracle/seat-sensitive.tsv).
// A metamorphic test, needing no second engine. Every runner hook it adds is
// a no-op when oracleRun.padInert is 0, so a non-padded run is untouched.

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// OraclePadResult is one scenario's outcome under the pad-to-four relation.
type OraclePadResult struct {
	Verdict   string   // "same", "differs", "base-fails", "not-two-seat"
	FirstDiff string   // set when Verdict is "differs"
	BaseFails []string // the two-seat run's own failures
	PadFails  []string // the four-seat run's failures against the scenario's expectations
}

// oraclePadSeats is how many inert seats a padded run adds.
const oraclePadSeats = 2

// RunOracleScenarioPadded plays one scenario as written and padded to four
// seats (padOracleScenario), and compares what the two original seats see.
// fixture marks a generated compliance scenario, as RunOracleAuditScenarioJSON's.
func RunOracleScenarioPadded(reg *cards.Registry, raw []byte, fixture bool) (OraclePadResult, error) {
	sc, err := decodeOracleScenario(raw)
	if err != nil {
		return OraclePadResult{}, err
	}
	sc.xmageFixture = fixture
	if _, err := oracleSeatCount(sc.Players); err != nil {
		return OraclePadResult{}, err
	}
	pad, ok := padOracleScenario(sc)
	if !ok {
		return OraclePadResult{Verdict: "not-two-seat"}, nil
	}
	baseFails, _, base := runOracleScenarioSpare(reg, sc, false, nil)
	if len(baseFails) > 0 {
		return OraclePadResult{Verdict: "base-fails", BaseFails: baseFails}, nil
	}
	padFails, _, padded := runOracleScenarioSpare(reg, pad, false, nil)
	res := OraclePadResult{Verdict: "same", PadFails: padFails}
	res.FirstDiff = oraclePadDiff(base, padded)
	if res.FirstDiff == "" && len(padFails) > 0 {
		res.FirstDiff = "expect: " + padFails[0]
	}
	if res.FirstDiff != "" {
		res.Verdict = "differs"
	}
	return res, nil
}

// padOracleScenario returns sc seated at four: p0 and p1 unchanged, two inert
// seats after them in turn order, and the scenario's turn mapped so the same
// seat is active (padTurn). false when sc seats more than two or its players
// count is invalid (RunOracleScenarioPadded reports that one as an error).
func padOracleScenario(sc oracleScenario) (oracleScenario, bool) {
	if seats, err := oracleSeatCount(sc.Players); err != nil || seats != 2 {
		return oracleScenario{}, false
	}
	turn := 1
	if sc.Turn != nil {
		turn = *sc.Turn
	}
	padded := padTurn(turn)
	sc.Turn = &padded
	sc.Players = 2 + oraclePadSeats
	sc.padInert = oraclePadSeats
	sc.padBaseTurn = turn
	return sc, true
}

// padTurn maps a two-seat turn to the four-seat turn with the same active
// seat: each two-seat round (p0, p1) becomes a four-seat round (p0, p1, p2, p3).
func padTurn(t int) int {
	return 4*((t-1)/2) + (t-1)%2 + 1
}

// inert reports whether p is one of a padded run's inert seats.
func (r *oracleRun) inert(p state.PlayerID) bool {
	return r.padInert > 0 && int(p) >= len(r.e.G.Players)-r.padInert
}

// padDeckTurn is the turn collectSetupDecks sizes seat p's deck for: the
// scenario's own turn for a focus seat of a padded run, turn otherwise.
func padDeckTurn(sc oracleScenario, seats, p, turn int) int {
	if sc.padInert > 0 && p < seats-sc.padInert {
		return sc.padBaseTurn
	}
	return turn
}

// skipInert passes every inert seat's priority, and during an inert seat's
// turn answers every pending decision of every seat by the fallback, until a
// focus seat holds priority in a focus seat's turn (or the game ends).
func (r *oracleRun) skipInert() error {
	if r.padInert == 0 {
		return nil
	}
	e := r.e
	for i := 0; i < 4000; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			return nil
		}
		if !r.inert(e.G.Active) && (d.Kind != decision.KPriority || !r.inert(d.Player)) {
			return nil
		}
		if d.Kind == decision.KPriority {
			idx := pickPass(d)
			if idx < 0 {
				return harnessf("priority with no pass option")
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				return err
			}
			continue
		}
		if err := r.answerFallback(d, "inert"); err != nil {
			return err
		}
	}
	return harnessf("inert seats never gave the game back")
}

// padUndoFirstDraw undoes the starting player's turn-1 draw of a padded run
// (CR 103.8a, 800.7: only a two-player game skips it), the first time a
// decision is pending in turn 1's draw step: the drawn card goes back on top
// of p0's library through logged events, so hands and libraries match the
// two-seat start. It returns whether the draw is undone.
func (r *oracleRun) padUndoFirstDraw(done bool) bool {
	e := r.e
	if r.padInert == 0 || done || e.G.Turn != 1 || e.G.Step != state.StepDraw || e.Pending() == nil {
		return done
	}
	var id state.ObjID
	for _, h := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(h).DrawnTurn == 1 {
			id = h // the turn-based draw is the only draw before this first decision
		}
	}
	if id == 0 {
		return true
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	order := []state.ObjID{id}
	for _, o := range e.G.Zone(state.ZLibrary, 0) {
		if o != id {
			order = append(order, o)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order, Secret: true})
	r.logf("  [pad] undo p0's turn-1 draw")
	return true
}

// oraclePadDiff returns the first difference the two focus seats can observe
// between the two-seat run and the padded run, or "". Turn, Over, Winner, the
// decisions and the option counts are never compared: four seats offer more
// targets, and the game goes on after a focus seat loses (CR 800.4a).
func oraclePadDiff(base, pad *oracleRun) string {
	if len(base.snaps) != len(pad.snaps) {
		return fmt.Sprintf("snapshots: %d != %d", len(base.snaps), len(pad.snaps))
	}
	for k := range base.snaps {
		if d := oraclePadSnapDiff(base.snaps[k], pad.snaps[k], 2); d != "" {
			return fmt.Sprintf("snap %d %s", k, d)
		}
		if base.snaps[k].Over {
			break
		}
	}
	return ""
}

// oraclePadSnapDiff compares one checkpoint's focus-seat view; focus is the
// number of focus seats.
func oraclePadSnapDiff(b, p OracleSnapshot, focus int) string {
	for _, f := range []struct {
		name string
		b, p any
	}{{"step", b.Step, p.Step}, {"active", b.Active, p.Active}, {"priority", b.Priority, p.Priority}} {
		if !reflect.DeepEqual(f.b, f.p) {
			return fmt.Sprintf("%s: %v != %v", f.name, f.b, f.p)
		}
	}
	for s := 0; s < focus; s++ {
		bp, pp := b.Players[s], p.Players[s]
		bp.Seat, pp.Seat = 0, 0
		if d := oraclePadFieldDiff(fmt.Sprintf("p%d.", s), bp, pp); d != "" {
			return d
		}
	}
	isFocus := func(seat int) bool { return seat < focus }
	var bperms, pperms []OracleSnapPerm
	for _, x := range b.Permanents {
		if isFocus(x.Controller) || isFocus(x.Owner) {
			bperms = append(bperms, x)
		}
	}
	for _, x := range p.Permanents {
		if isFocus(x.Controller) || isFocus(x.Owner) {
			pperms = append(pperms, x)
		}
	}
	for i := 0; i < max(len(bperms), len(pperms)); i++ {
		if i >= len(bperms) || i >= len(pperms) {
			return fmt.Sprintf("permanents: %s != %s", oraclePadPermNames(bperms), oraclePadPermNames(pperms))
		}
		if d := oraclePadFieldDiff(fmt.Sprintf("perm %s.", bperms[i].Ref), bperms[i], pperms[i]); d != "" {
			return d
		}
	}
	for _, x := range p.Permanents {
		if !isFocus(x.Controller) && !isFocus(x.Owner) {
			return fmt.Sprintf("inert: p%d controls %s", x.Controller, x.Name)
		}
	}
	if !reflect.DeepEqual(b.Stack, p.Stack) {
		return fmt.Sprintf("stack: %v != %v", b.Stack, p.Stack)
	}
	return ""
}

// oraclePadFieldDiff compares two structs of one type field by field and
// names the first that differs by its JSON name, prefixed.
func oraclePadFieldDiff(prefix string, b, p any) string {
	bv, pv := reflect.ValueOf(b), reflect.ValueOf(p)
	t := bv.Type()
	for i := 0; i < t.NumField(); i++ {
		bf, pf := bv.Field(i).Interface(), pv.Field(i).Interface()
		if reflect.DeepEqual(bf, pf) {
			continue
		}
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		return fmt.Sprintf("%s%s: %v != %v", prefix, name, bf, pf)
	}
	return ""
}

func oraclePadPermNames(ps []OracleSnapPerm) string {
	names := make([]string, len(ps))
	for i, x := range ps {
		names[i] = x.Ref
	}
	return "[" + strings.Join(names, " ") + "]"
}
