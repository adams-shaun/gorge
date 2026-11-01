package event

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/adams-shaun/gorge/draft/limited"
)

// DraftDataOptions shapes a draft_data export. The zero value is
// DraftDataCSVRows': no header line, the default event type and one
// draft_id.
type DraftDataOptions struct {
	// EventType is the event_type column's value; "" = "gorge_pod".
	EventType string
	// PerSeatDraftID writes draft_id "<seed>-s<seat>" (one per drafter, as
	// 17Lands') instead of "<seed>".
	PerSeatDraftID bool
	// Header writes the header line first.
	Header bool
}

// DraftDataCSV writes the event's draft as one row per pick in 17Lands'
// draft_data shape: the columns expansion, event_type ("gorge_pod"),
// draft_id (the event seed in decimal), pack_number (the draft round),
// pick_number (the seat's pick within the round) and pick (the picked
// card's name), then one pack_card_<name> column per name in cardNames —
// the count of that name in the pack as the seat saw it at the pick — and
// one pool_<name> column — the count in the seat's pool before the pick.
// cardNames is sorted internally, so the column order is deterministic.
// The pack and pool state is rebuilt by replaying the event's own log, so
// the export depends on nothing but the log.
func DraftDataCSV(e *Event, w io.Writer, cardNames []string) error {
	return DraftDataCSVWith(e, w, cardNames, DraftDataOptions{Header: true})
}

// DraftDataCSVRows writes an event's draft_data ROWS with no header line:
// the shape a run appends to an existing -draftlog (a second header line
// mid-file is not a 17Lands draft_data CSV). The column order is the same
// header's. The event must be created.
func DraftDataCSVRows(e *Event, w io.Writer, cardNames []string) error {
	return DraftDataCSVWith(e, w, cardNames, DraftDataOptions{})
}

// DraftDataCSVWith writes a draft_data export shaped by o: Header adds the
// header line, EventType renames the event_type column ("" =
// "gorge_pod"), and PerSeatDraftID writes draft_id "<seed>-s<seat>" — one
// draft_id per drafter, as 17Lands' own draft_data carries — instead of
// "<seed>". The default values reproduce DraftDataCSV (with a header) and
// DraftDataCSVRows (without) byte for byte.
func DraftDataCSVWith(e *Event, w io.Writer, cardNames []string, o DraftDataOptions) error {
	if !e.cooked {
		return fmt.Errorf("event: DraftDataCSVWith: event not created")
	}
	names := append([]string(nil), cardNames...)
	sort.Strings(names)
	eventType := o.EventType
	if eventType == "" {
		eventType = "gorge_pod"
	}
	cw := csv.NewWriter(w)
	if o.Header {
		header := []string{"expansion", "event_type", "draft_id", "pack_number", "pick_number", "pick"}
		for _, n := range names {
			header = append(header, "pack_card_"+n)
		}
		for _, n := range names {
			header = append(header, "pool_"+n)
		}
		if err := cw.Write(header); err != nil {
			return fmt.Errorf("event: DraftDataCSVWith: %w", err)
		}
	}
	if err := draftDataRows(e, cw, names, eventType, o.PerSeatDraftID); err != nil {
		return err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("event: DraftDataCSVWith: %w", err)
	}
	return nil
}

// draftDataRows replays the log onto a throwaway event and writes one
// draft_data row per pick, capturing each pick's pack and pool from the pod
// state just before the pick is applied. eventType is the event_type
// column's value; perSeatID switches draft_id to "<seed>-s<seat>".
func draftDataRows(e *Event, cw *csv.Writer, names []string, eventType string, perSeatID bool) error {
	re := New()
	idToName := map[string]string{}
	entries := e.Log()
	for i := range entries {
		en := entries[i]
		switch en.Kind {
		case KindPackOpened:
			for _, c := range en.Cards {
				idToName[c.ID] = c.Name
			}
		case KindPicked:
			pack, ok := re.Next(en.Seat)
			if !ok {
				return fmt.Errorf("event: DraftDataCSVWith: log entry %d: seat %d has no pack at its pick", i, en.Seat)
			}
			name := idToName[en.ID]
			if name == "" {
				return fmt.Errorf("event: DraftDataCSVWith: log entry %d: picked ID %q is in no logged pack", i, en.ID)
			}
			row := make([]string, 0, 6+2*len(names))
			row = append(row,
				e.set,
				eventType,
				draftID(e.seed, en.Seat, perSeatID),
				strconv.Itoa(en.Round),
				strconv.Itoa(en.PickNo),
				name,
			)
			row = appendRowCounts(row, pack.Pack, names)
			row = appendRowCounts(row, re.Pool(en.Seat), names)
			if err := cw.Write(row); err != nil {
				return fmt.Errorf("event: DraftDataCSVWith: %w", err)
			}
		}
		if err := re.Apply(en); err != nil {
			return fmt.Errorf("event: DraftDataCSVWith: log entry %d does not replay: %w", i, err)
		}
	}
	return nil
}

// draftID spells a row's draft_id: the event seed in decimal, or
// "<seed>-s<seat>" when perSeatID (one draft_id per drafter, as 17Lands').
func draftID(seed uint64, seat int, perSeatID bool) string {
	if !perSeatID {
		return strconv.FormatUint(seed, 10)
	}
	return fmt.Sprintf("%d-s%d", seed, seat)
}

// appendRowCounts appends one column per name: the count of that card name
// in cards.
func appendRowCounts(row []string, cards []limited.CardRef, names []string) []string {
	counts := make(map[string]int, len(cards))
	for _, c := range cards {
		counts[c.Name]++
	}
	for _, n := range names {
		row = append(row, strconv.Itoa(counts[n]))
	}
	return row
}
