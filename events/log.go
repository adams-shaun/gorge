package events

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Log is an append-only event log plus the intents that produced it, with a
// rolling hash chain over the events. Two logs with the same Head describe the
// same match; a replay that diverges says so in one comparison.
type Log struct {
	Seed    uint64            `json:"seed"`
	Events  []Event           `json:"events"`
	Intents []decision.Intent `json:"intents"`

	// NoHash disables chaining. Benchmarks use it to price the audit trail;
	// production never sets it.
	// Must be set before the first Append and never changed after.
	NoHash bool `json:"-"`

	chain    [sha256.Size]byte
	buf      []byte
	headHash hash.Hash
	started  bool
	// noHashSet records NoHash's value on the first Append to pin it from
	// changing (see the check in Append).
	noHashSet bool
}

func NewLog(seed uint64) *Log {
	l := &Log{Seed: seed, buf: make([]byte, 0, 128), headHash: sha256.New()}
	// Seed the chain with the seed value
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], seed)
	h := sha256.New()
	h.Write(b[:])
	h.Sum(l.chain[:0])
	return l
}

// Append assigns the next sequence number, folds the event into the chain and
// stores it. It returns the stored event so callers see the assigned Seq.
func (l *Log) Append(e Event) Event {
	// Check NoHash immutability: must not change after the log is started
	if l.started {
		if l.noHashSet != l.NoHash {
			panic("events: NoHash changed after the log was started")
		}
	} else {
		l.started = true
		l.noHashSet = l.NoHash
	}

	e.Seq = uint64(len(l.Events))

	// Copy IDs and Pairs so a caller mutating its own slice afterwards cannot
	// retroactively rewrite a logged event and desync Head from HeadAt.
	// This also normalises a non-nil empty slice to nil. That is deliberate:
	// the encoding writes a length prefix only, so nil and empty are already
	// indistinguishable on the wire, and collapsing them keeps the in-memory
	// log canonical with what the chain actually hashed.
	e.IDs = append([]state.ObjID(nil), e.IDs...)
	e.Pairs = append([][2]state.ObjID(nil), e.Pairs...)

	l.Events = growEvents(l.Events, len(l.Events)+1)
	l.Events[len(l.Events)-1] = e
	if l.NoHash {
		return e
	}
	l.buf = e.Append(l.buf[:0])
	// Reuse one digest instead of sha256.New() per event — a fresh hasher per
	// append was a measurable per-event allocation on a long log. Reset
	// restores the initial (empty) state, so the fold into the chain is
	// byte-identical to a fresh hasher's: sha256(chain || encode(e)).
	l.headHash.Reset()
	l.headHash.Write(l.chain[:])
	l.headHash.Write(l.buf)
	l.headHash.Sum(l.chain[:0])
	return e
}

// Head is the chain head over every event so far.
func (l *Log) Head() string {
	if l.NoHash {
		return ""
	}
	return hex.EncodeToString(l.chain[:8])
}

// HeadAt recomputes the chain over the first n events, which is what makes
// "playback to N" verifiable against a full log.
func (l *Log) HeadAt(n int) string {
	if l.NoHash {
		return ""
	}
	// Start with the seeded chain state
	var cur [sha256.Size]byte
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], l.Seed)
	h := sha256.New()
	h.Write(b[:])
	h.Sum(cur[:0])

	var buf []byte
	for i := 0; i < n && i < len(l.Events); i++ {
		buf = l.Events[i].Append(buf[:0])
		h := sha256.New()
		h.Write(cur[:])
		h.Write(buf)
		h.Sum(cur[:0])
	}
	return hex.EncodeToString(cur[:8])
}

// Clone returns a log that continues the same chain from exactly where l's
// was: the same Seed, NoHash and chain state. Events and Intents are shared
// by backing array, not copied — a stored Event or Intent is append-only,
// hash-chained history: once Append has folded it into the chain, mutating
// it (or its IDs/Pairs/Choices) in place would desync Head from HeadAt, so
// the engine never does. Because nothing writes to the shared region, both
// logs can read it freely. The full-slice expressions below are the point:
// each fixes cap == len, so the first append on EITHER side allocates a
// fresh array and the two logs diverge cleanly, never writing into the
// other's storage (pinned by TestLogCloneAppendsDiverge). buf is a scratch
// buffer reused only by Append, so a clone starts from nil and gets its own
// backing on first use — sharing it could race two append paths. Sharing
// here is what keeps memory-profile hot: ViewAt clones per time-travel query
// and Engine.Clone per turn-start snapshot, and a deep copy of an ever-
// growing log was ~17 GB of allocated space in the host test.
func (l *Log) Clone() *Log {
	c := *l
	c.Events = l.Events[:len(l.Events):len(l.Events)]
	c.Intents = l.Intents[:len(l.Intents):len(l.Intents)]
	c.buf = nil
	// headHash is mutable scratch Append reuses; two appended-to logs must not
	// share it, exactly as they must not share buf. A clone gets its own.
	c.headHash = sha256.New()
	return &c
}

// growEvents grows s's backing array by a fixed geometric factor (doubling)
// instead of relying on the built-in append growth, which tapers to 1.25x
// for slices past 1024 elements. A match log that grows to a few tens of
// thousands of events is the common case in ./host, and the runtime default
// reallocates (and copies) on the order of 5x its final length there; a
// constant doubling factor keeps the total realloc at ~2x final length for a
// slice of any size. It returns s with len == need, whether in place or in a
// freshly-allocated array, so the ordinary no-realloc Append path costs
// nothing. Clone safety is untouched: Clone has already fixed a clone's
// cap == len, so the first append on either side arrives here with no spare
// capacity, allocates a fresh array, and the two logs diverge cleanly
// (pinned by TestLogCloneAppendsDiverge).
func growEvents(s []Event, need int) []Event {
	if need <= cap(s) {
		return s[:need]
	}
	newCap := cap(s)
	if newCap < 16 {
		newCap = 16
	}
	for newCap < need {
		newCap *= 2
	}
	out := make([]Event, need, newCap)
	copy(out, s)
	return out
}
