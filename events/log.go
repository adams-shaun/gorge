package events

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/adams-shaun/gorge/decision"
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
	NoHash bool `json:"-"`

	chain [sha256.Size]byte
	buf   []byte
}

func NewLog(seed uint64) *Log { return &Log{Seed: seed, buf: make([]byte, 0, 128)} }

// Append assigns the next sequence number, folds the event into the chain and
// stores it. It returns the stored event so callers see the assigned Seq.
func (l *Log) Append(e Event) Event {
	e.Seq = uint64(len(l.Events))
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
	var cur [sha256.Size]byte
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
