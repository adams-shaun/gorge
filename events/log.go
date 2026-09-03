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

	// Copy IDs and Pairs slices to avoid aliasing issues when the caller mutates their slices
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
