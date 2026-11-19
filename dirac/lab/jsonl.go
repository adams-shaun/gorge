package lab

import (
	"encoding/json"
	"io"
	"strconv"
)

// AppendJSON appends r as one JSON line. The fields dplay writes keep its
// names: pair, decks, seed, winner (-1 for no winner), turns, intents (the
// decisions answered), and stall and class for a stalled game and error for
// a failed one. outcome is added: won, drawn, stalled, truncated or failed.
func (r *Result) AppendJSON(b []byte) []byte {
	b = append(b, `{"pair":`...)
	b = strconv.AppendInt(b, int64(r.Pair), 10)
	b = append(b, `,"decks":[`...)
	for i, d := range r.Decks {
		if i > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendQuote(b, d)
	}
	b = append(b, `],"seed":`...)
	b = strconv.AppendUint(b, r.Seed, 10)
	b = append(b, `,"winner":`...)
	b = strconv.AppendInt(b, int64(r.Winner), 10)
	b = append(b, `,"turns":`...)
	b = strconv.AppendInt(b, int64(r.Turns), 10)
	b = append(b, `,"intents":`...)
	b = strconv.AppendInt(b, int64(r.Decisions), 10)
	b = append(b, `,"outcome":`...)
	b = strconv.AppendQuote(b, r.Outcome.String())
	if r.Stall != "" {
		b = append(b, `,"stall":`...)
		b = strconv.AppendQuote(b, r.Stall)
		b = append(b, `,"class":`...)
		b = strconv.AppendQuote(b, r.Class)
	}
	if r.Err != "" {
		q, _ := json.Marshal(r.Err)
		b = append(append(b, `,"error":`...), q...)
	}
	return append(b, "}\n"...)
}

// WriteJSONL writes one JSON line per result.
func WriteJSONL(w io.Writer, rs []Result) error {
	var b []byte
	for i := range rs {
		b = rs[i].AppendJSON(b[:0])
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}
