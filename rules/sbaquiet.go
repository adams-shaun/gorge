package rules

import "fmt"

// Skipping a state-based-action pass loop that provably applies nothing.
//
// checkStateBased runs after nearly every Submit and step, and its pass loop
// rescans the battlefield (derived toughness, attachments, sagas, tokens,
// departed players) every time -- although most of those calls follow
// nothing but priority bookkeeping since the previous call, which found
// nothing to do.
//
// The pass loop is a deterministic function of the game state (e.G), the
// continuous-effect registry and a few engine runtime fields. When one run of
// the loop emits no event at all it changed nothing, so a later run from the
// same inputs emits nothing either. sbaQuiet records the key of such a run:
// the log length after it, continuousVersion and len(e.G.Objs). The next
// call skips the loop when every event appended since is layer-inert
// (DecisionAsk, DecisionMade, Priority -- layercache.go's argument: Apply
// writes nothing for the two markers and only g.Priority/g.Passes for
// Priority, which no SBA reads, directly or through the layer path) and the
// other two inputs are unchanged. That is exactly the layer caches' reuse
// key, so the derived characteristics the loop reads are unchanged too.
//
// The runtime fields the loop reads are excluded rather than keyed:
//
//   - e.pending / e.choosing / e.legendBatch decide only whether a found
//     duplicate-permanent set (CR 704.5j legend / CR 704.5k world) is parked
//     (an emitting ask) or deferred (no emit).
//     A deferral sets sbaUnquiet, so a run that deferred never records.
//   - e.pendingTriggers decides whether a concluded Saga is "busy" (its
//     chapter ability still pending). A busy-deferred Saga sets sbaUnquiet.
//   - the per-call attempt memory (sbaAttempts) is local to one call, and a
//     quiet run attempted nothing that emitted; the departed-player sweep
//     re-runs every call but emits nothing once a player is swept, which is
//     precisely a quiet run.
//
// Only the pass loop is skipped: the legend re-pose guard, expireControl,
// reconcileControlStatics, checkGameOver and the departed-decision release
// around it still run on every call. Clone() leaves sbaQuiet zero (ep 0
// never matches), and a direct e.G write in a test with no event is the one
// input the key cannot see -- sbaQuietVerify, on in the rules test binary,
// runs the loop anyway on every would-be skip and panics if it emits.

type sbaQuietKey struct {
	ep   int
	ver  int
	objs int
}

// sbaQuietVerifyFlag turns verify mode on in a non-test binary:
// go build -ldflags "-X github.com/adams-shaun/gorge/rules.sbaQuietVerifyFlag=1".
var sbaQuietVerifyFlag string

var sbaQuietVerify = sbaQuietVerifyFlag != ""

// sbaQuietNow reports whether the pass loop would provably apply nothing.
func (e *Engine) sbaQuietNow() bool {
	q := e.sbaQuiet
	if q.ep <= 0 || e.legendBatch != nil || q.ver != e.continuousVersion || q.objs != len(e.G.Objs) ||
		!e.layerInertSince(q.ep) {
		return false
	}
	// The player-level losses (CR 704.5a/b, 903.10) are re-read on every
	// call: a per-seat scan, cheap next to the battlefield passes, and it
	// keeps a direct life/poison write (a test's board setup, the one input
	// the log key cannot see) from being skipped past.
	for i := range e.G.Players {
		p := &e.G.Players[i]
		if p.Lost {
			continue
		}
		if p.Life <= 0 || p.Counter("POISON") >= 10 {
			return false
		}
		for _, dmg := range p.CmdDamage {
			if dmg >= 21 {
				return false
			}
		}
	}
	return true
}

// sbaRecordQuiet records the key after a pass loop that started at log
// length ep0 and reached its fixed point; only a run that emitted nothing
// and deferred nothing on a runtime input is quiet.
func (e *Engine) sbaRecordQuiet(ep0 int) {
	if len(e.L.Events) != ep0 || e.sbaUnquiet || e.legendBatch != nil {
		e.sbaQuiet = sbaQuietKey{}
		return
	}
	e.sbaQuiet = sbaQuietKey{ep: ep0, ver: e.continuousVersion, objs: len(e.G.Objs)}
}

func (e *Engine) verifySBAQuiet(ep0 int) {
	if n := len(e.L.Events); n != ep0 {
		panic(fmt.Sprintf("rules: SBA quiet skip at log %d disagrees with a full pass (%d events emitted, first %v)",
			ep0, n-ep0, e.L.Events[ep0].Kind))
	}
	if e.legendBatch != nil {
		panic(fmt.Sprintf("rules: SBA quiet skip at log %d disagrees with a full pass (duplicate-permanent batch parked)", ep0))
	}
}
