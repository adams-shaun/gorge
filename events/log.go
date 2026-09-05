package events

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

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

	chain     [sha256.Size]byte
	buf       []byte
	started   bool
	noHashSet bool
}

func NewLog(seed uint64) *Log {
	l := &Log{Seed: seed, buf: make([]byte, 0, 128)}
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

	l.Events = append(l.Events, e)
	if l.NoHash {
		return e
	}
	l.buf = e.Append(l.buf[:0])
	h := sha256.New()
	h.Write(l.chain[:])
	h.Write(l.buf)
	h.Sum(l.chain[:0])
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

// Clone returns an independent copy of l: the same Seed, NoHash and chain
// state, and fresh copies of Events and Intents (down to each event's IDs
// and Pairs and each intent's Choices). Appending to either log afterwards
// leaves the other untouched, and the copy's Head continues from exactly
// where l's was. rules.Engine.Clone uses it to snapshot a match.
func (l *Log) Clone() *Log {
	c := *l
	c.Events = make([]Event, len(l.Events))
	for i, e := range l.Events {
		e.IDs = append([]state.ObjID(nil), e.IDs...)
		e.Pairs = append([][2]state.ObjID(nil), e.Pairs...)
		c.Events[i] = e
	}
	c.Intents = make([]decision.Intent, len(l.Intents))
	for i, in := range l.Intents {
		in.Choices = append([]int(nil), in.Choices...)
		c.Intents[i] = in
	}
	c.buf = make([]byte, 0, cap(l.buf))
	return &c
}
