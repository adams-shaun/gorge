package host

import "fmt"

// The provenance ledger is HD-2 of the human-divergence design
// (docs/design/2026-10-10-human-divergence-design.md §5.2): one
// byte per accepted intent, saying whether the answer was a deliberate human
// choice. The match log alone cannot tell — a caretaker substitution logs
// byte-identically to a human answer (D3), and the client's autopilot submits
// through the same intent route. Two bits per intent:
//
//   - bit 0 (provCaretaker): the seat's caretaker answered — the HumanSeat's
//     caretakerFires counter moved across the await.
//   - bit 1 (provAuto): the client submitted the answer with ?auto=1, the
//     autopilot flag the intent handler forwards into SubmitIntentFrom.
//
// The ledger is always built (it costs one append per intent in the Submit's
// already-exclusive section and allocates only on slice growth), so the table
// config stays out of the hot loop and a later match can be scanned without
// having had to opt in at intent time. It is host-side metadata of the same
// kind as m.snaps: never replay input, never an event, never part of
// decision.Intent — a match played with and without the auto bit replays to
// the identical chain head. A refused Submit appends nothing, so the ledger's
// length always equals the match's intent count, and an undo truncates it
// beside the log (rewindToLastIntent).
//
// The only reader is Provenance; ShadowScan reads the
// same slice under the match lock to exclude caretaker and auto intents from
// the divergence scan.

const (
	// provCaretaker marks an intent the human seat's caretaker produced.
	provCaretaker uint8 = 1 << 0
	// provAuto marks an intent the client's autopilot submitted (?auto=1).
	provAuto uint8 = 1 << 1
)

// ledgerAppend records the provenance bits of the intent the match just
// accepted: after afterSubmit has counted it, m.intents-1 is its index and
// len(m.ledger) is too. Called inside attempt's exclusive m.locked section,
// on the match goroutine.
func (m *match) ledgerAppend(caretaker, auto bool) {
	var b uint8
	if caretaker {
		b |= provCaretaker
	}
	if auto {
		b |= provAuto
	}
	m.ledger = append(m.ledger, b)
}

// ledgerTruncate drops every ledger entry at or past intent n — the tail an
// undo rewound away. Called on the match goroutine under m.mu, beside the
// bounds/turnStarts/snaps truncation in rewindToLastIntent, so the ledger
// always describes exactly the intents the log holds.
func (m *match) ledgerTruncate(n int) {
	if len(m.ledger) > n {
		m.ledger = m.ledger[:n]
	}
}

// Provenance returns a copy of match k of table id's provenance ledger,
// indexed by intent (see the package ledger doc). It is the read path HD-3's
// ShadowScan and the ledger tests use; the returned slice is a copy, so a
// caller can never mutate the match's own bytes. A finished non-perpetual
// match is read through the table's history tail once run() has retired it
// (t.cur nil) — the same current-or-last match a feedback snapshot picks —
// so the ledger stays readable after the match ends.
func (r *Registry) Provenance(id TableID, k int) ([]uint8, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	t.mu.RLock()
	m := t.cur
	if m == nil && len(t.history) > 0 && t.history[len(t.history)-1].k == k {
		m = t.history[len(t.history)-1]
	}
	t.mu.RUnlock()
	if m == nil || m.k != k {
		return nil, fmt.Errorf("host: table %s: match %d is not the current or last match", id, k)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]uint8(nil), m.ledger...), nil
}
