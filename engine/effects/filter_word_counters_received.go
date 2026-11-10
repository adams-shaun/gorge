package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// CounterAdd is one positive object-counter placement of the current turn, as
// the engine's per-turn counter ledger holds it: who put the counters on
// (Adder), which object received them (Obj, the id the object keeps through a
// zone change), the counter kind and the amount. The ledger is the same one
// Count$CountersAddedThisTurn reads; this is its id-keyed view for filters.
type CounterAdd struct {
	Adder  state.PlayerID
	Obj    state.ObjID
	Kind   string
	Amount int32
}

// counterAddsHost is the optional Host role that publishes the ledger. It is
// an assertion target, not a Host method, so a host without a ledger (a
// frozen snapshot, a test double) reads as "no counters were put" and the
// word fails closed.
type counterAddsHost interface {
	CounterAddsThisTurn() []CounterAdd
}

// counterAddsFor returns the ledger for a spec that reads it, nil for every
// other spec (the hot count path pays one substring test and nothing else).
// For a spec that names the word the result is never nil: a host without the
// role publishes an empty, bound ledger. nil means "not bound" to
// contextPredicateBound, which makes both spellings fail closed.
func counterAddsFor(h Host, spec string) []CounterAdd {
	if !strings.Contains(spec, countersReceivedWord) {
		return nil
	}
	if ch, ok := h.(counterAddsHost); ok {
		if adds := ch.CounterAddsThisTurn(); adds != nil {
			return adds
		}
	}
	return []CounterAdd{}
}

const countersReceivedWord = "countersReceivedThisTurn_"

// countersReceivedShape is the one shape of Forge's
// countersReceivedThisTurn_<cmp><N>_<kind>_<player> this build reads: any
// numeric comparator, a literal N, kind Any, player You. The kind-specific
// spelling (Kid Loki's GE1_P1P1_You, a continuous-effect scope) and the
// non-You players stay wordUnknown and fail closed by name: a static ability
// is matched without the ledger bound, so reading them there would be
// silently false rather than loud.
type countersReceivedShape struct {
	op CmpOp
	n  int
}

// countersReceivedParse parses the text after the word's prefix.
func countersReceivedParse(rest string) (countersReceivedShape, bool) {
	cmpN, tail, ok := strings.Cut(rest, "_")
	if !ok || len(cmpN) < 3 || tail != "Any_You" {
		return countersReceivedShape{}, false
	}
	op := CmpOpOf(cmpN[:2])
	n, err := strconv.Atoi(cmpN[2:])
	if op == CmpNone || err != nil {
		return countersReceivedShape{}, false
	}
	return countersReceivedShape{op: op, n: n}, true
}

// countersReceivedWordKind classifies p; ok is false when p is not this word.
func countersReceivedWordKind(p string) (kind wordKind, key string, ok bool) {
	rest, has := strings.CutPrefix(p, countersReceivedWord)
	if !has {
		return wordUnknown, "", false
	}
	if _, good := countersReceivedParse(rest); !good {
		return wordUnknown, "", true
	}
	return wordCountersReceived, rest, true
}

// countersReceivedMatches evaluates the word: the total counters of any kind
// the evaluating player (sc.You) put on the candidate this turn, compared
// with N. The object is matched by id and the adder by player.
func countersReceivedMatches(o *state.Object, sc SpecContext, key string) bool {
	shape, ok := countersReceivedParse(key)
	if !ok || o == nil {
		return false
	}
	var total int32
	for _, add := range sc.CounterAdds {
		if add.Obj == o.ID && add.Adder == sc.You {
			total += add.Amount
		}
	}
	return shape.op.Apply(int(total), shape.n)
}
