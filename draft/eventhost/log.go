package eventhost

import (
	"encoding/json"

	"github.com/adams-shaun/gorge/server/protocol"
)

// EventLog is the event's log as the token's viewer sees it: a seat token
// reads its own seat's facts (its packs, picks and deck), an empty token
// the public spectator view. Entries stay redacted until the event is
// COMPLETE (event.LogFor).
func (h *Host) EventLog(id, token string) (protocol.EventLog, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.EventLog{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, true)
	if terr != nil {
		return protocol.EventLog{}, terr
	}
	out := protocol.EventLog{Event: r.meta.ID, Complete: r.ev.Done()}
	for _, en := range r.ev.LogFor(seat) {
		b, err := json.Marshal(&en)
		if err != nil {
			return protocol.EventLog{}, errUnavailable("eventhost: %s entry: %v", en.Kind, err)
		}
		out.Entries = append(out.Entries, json.RawMessage(b))
	}
	return out, nil
}
