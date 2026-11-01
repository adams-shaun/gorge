package eventhost

import (
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/server/protocol"
)

// Sets lists every draftable set for GET /api/events/sets: the set data
// catalogue's code, name and pack size, plus the rating source the pickbot
// uses for it — "17lands" when an embedded ratings manifest covers the
// set, "heuristic" otherwise.
func (h *Host) Sets() []protocol.EventSetInfo {
	codes := setdata.Codes()
	out := make([]protocol.EventSetInfo, 0, len(codes))
	for _, code := range codes {
		spec, err := setdata.Load(code)
		if err != nil {
			continue
		}
		ratings := "heuristic"
		if pickbot.RatedSet(code) {
			ratings = "17lands"
		}
		out = append(out, protocol.EventSetInfo{
			Code: code, Name: spec.Name, PackSize: spec.PackSize, Ratings: ratings,
		})
	}
	return out
}

// Personas lists the registered pickbot personas for
// GET /api/events/personas. Every registry entry names one drafter/builder
// pair, so both lists are the registry's sorted names.
func (h *Host) Personas() protocol.EventPersonas {
	names := pickbot.Names()
	return protocol.EventPersonas{
		Drafters: append([]string(nil), names...),
		Builders: append([]string(nil), names...),
	}
}
