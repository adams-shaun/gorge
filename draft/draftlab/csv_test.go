package draftlab

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/deck"
)

// csvLabNames is the FDN game_data name universe the CSV tests share: the
// sheet universe united with the basics, sorted unique.
func csvLabNames(t *testing.T, l *Lab) []string {
	t.Helper()
	names, err := l.CardNames("FDN", setdata.Collation(""))
	if err != nil {
		t.Fatalf("CardNames: %v", err)
	}
	names = append(append([]string(nil), names...), BasicNames[:]...)
	sort.Strings(names)
	return uniqueSorted(names)
}

// uniqueSorted drops adjacent duplicates from a sorted slice.
func uniqueSorted(s []string) []string {
	out := s[:0]
	for i, n := range s {
		if i == 0 || n != s[i-1] {
			out = append(out, n)
		}
	}
	return out
}

// findMonoCard names the first universe card whose colour identity is
// exactly the one WUBRG letter bit and which is not a land.
func findMonoCard(t *testing.T, l *Lab, names []string, bit uint8) string {
	t.Helper()
	for _, n := range names {
		c, ok := l.reg.Lookup(n)
		if !ok || (len(c.Faces) > 0 && c.Faces[0].IsLand()) {
			continue
		}
		if c.ColourIdentity() == 1<<bit {
			return n
		}
	}
	t.Fatalf("no mono-%s card in the universe", "WUBRG"[bit:bit+1])
	return ""
}

// repeat builds a main-deck list: n copies of one name plus the basics
// (lands, which the colour classifier skips).
func repeat(name string, n int) []string {
	out := make([]string, 0, n+5)
	for i := 0; i < n; i++ {
		out = append(out, name)
	}
	return append(out, BasicNames[:]...)
}

// TestDraftDataCSVPerSeat: draftlab.DraftDataCSV over a finished 4-seat
// draft's log writes one row per pick with one draft_id per drafter and
// the gorge_pod event type, header included, all through the log bytes.
func TestDraftDataCSVPerSeat(t *testing.T) {
	l := testLab(t)
	s := resolveEventAgents(t, l, testEventSpec(42))
	s.Structure = NoGames
	res, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	picks := countEntries(t, res.Log, event.KindPicked)
	if picks != 4*3*14 {
		t.Fatalf("%d picks in the log, want %d", picks, 4*3*14)
	}
	names, err := l.CardNames("FDN", setdata.Collation(""))
	if err != nil {
		t.Fatalf("CardNames: %v", err)
	}
	var buf bytes.Buffer
	if err := DraftDataCSV(&buf, res.Log, names, true); err != nil {
		t.Fatalf("DraftDataCSV: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 1+picks {
		t.Fatalf("CSV has %d lines, want header plus %d picks", len(lines), picks)
	}
	cw := csv.NewReader(strings.NewReader(buf.String()))
	rows, err := cw.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 1+picks {
		t.Fatalf("CSV has %d records, want header plus %d", len(rows), picks)
	}
	if got := rows[0][1]; got != "event_type" {
		t.Errorf("header cell 1 is %q, want event_type", got)
	}
	if got := rows[0][2]; got != "draft_id" {
		t.Errorf("header cell 2 is %q, want draft_id", got)
	}
	counts := map[string]int{}
	for i, row := range rows[1:] {
		if row[1] != "gorge_pod" {
			t.Fatalf("row %d event_type %q, want gorge_pod", i, row[1])
		}
		want := fmt.Sprintf("42-s%d", i%4)
		if row[2] != want {
			t.Fatalf("row %d draft_id %q, want %q", i, row[2], want)
		}
		counts[row[2]]++
	}
	if len(counts) != 4 {
		t.Fatalf("%d distinct draft ids, want 4", len(counts))
	}
	for id, n := range counts {
		if n != 3*14 {
			t.Errorf("draft id %s has %d rows, want %d", id, n, 3*14)
		}
	}
}

// TestGameDataCSVShape writes handcrafted GameResults through GameDataCSV
// and checks the fixed prefix, the colour classification, the boolean
// spelling, the draw skip and the per-card count columns.
func TestGameDataCSVShape(t *testing.T) {
	l := testLab(t)
	names := csvLabNames(t, l)
	w := findMonoCard(t, l, names, 0)
	u := findMonoCard(t, l, names, 1)
	b := findMonoCard(t, l, names, 2)

	// Player 0's deck: 6 W, 6 U and 6 B non-land cards -> main WUB, no
	// splash. Player 1's deck: 7 W only -> main W.
	main0 := append(append(repeat(w, 6), repeat(u, 6)...), repeat(b, 6)...)
	main1 := repeat(w, 7)
	games := []GameResult{
		{
			Seed: 1, Round: 2, Game: 3,
			Players:     [2]string{"bot@1", "bot@1"},
			DraftIDs:    [2]string{"42-s0", "42-s1"},
			Main:        [2][]string{main0, main1},
			Sideboard:   [2][]string{{u}, nil},
			OpeningHand: [2][]string{{w}, {w}},
			Drawn:       [2][]string{nil, {b}},
			Winner:      0, Starter: 0, Turns: 12, Mulligans: [2]int{1, 2},
		},
		{
			Seed: 2, Round: 2, Game: 4,
			Players: [2]string{"bot@1", "bot@1"},
			Main:    [2][]string{main0, main1},
			Winner:  -1, Starter: 1, Turns: 3, // a draw: no rows
		},
		{
			Seed: 3, Round: 2, Game: 5,
			Players:  [2]string{"bot@1", "bot@1"},
			DraftIDs: [2]string{"42-s0", "42-s1"},
			Main:     [2][]string{main0, main1},
			Winner:   1, Starter: 1, Turns: 5, Mulligans: [2]int{0, 0},
		},
	}
	var buf bytes.Buffer
	if err := l.GameDataCSV(&buf, games, "FDN", "gorge_lab", names, true); err != nil {
		t.Fatalf("GameDataCSV: %v", err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 1+4 {
		t.Fatalf("%d records, want header plus two rows per finished game", len(rows))
	}
	hdr := rows[0]
	if got, want := len(hdr), 18+4*len(names); got != want {
		t.Fatalf("header has %d cells, want %d", got, want)
	}
	for i, want := range gameDataColumns {
		if hdr[i] != want {
			t.Errorf("header cell %d is %q, want %q", i, hdr[i], want)
		}
	}
	// Each card group follows the universe in its own order.
	for gi, prefix := range []string{"deck_", "sideboard_", "opening_hand_", "drawn_"} {
		for j, n := range names {
			if got := hdr[18+gi*len(names)+j]; got != prefix+n {
				t.Errorf("header cell %d is %q, want %q", 18+gi*len(names)+j, got, prefix+n)
			}
		}
	}

	checkRow := func(row []string, p int, g GameResult, opp string) {
		t.Helper()
		want := []string{"FDN", "gorge_lab", g.DraftIDs[p], "", "", "1",
			"2", fmt.Sprint(g.Game), "", "",
			"", "", boolCell(g.Starter == p),
			fmt.Sprint(g.Mulligans[p]), fmt.Sprint(g.Mulligans[1-p]),
			opp, fmt.Sprint(g.Turns), boolCell(g.Winner == p),
		}
		want[10], want[11] = "WUB", "" // player 0's deck
		if p == 1 {
			want[10], want[11] = "W", ""
		}
		for i, w := range want {
			if row[i] != w {
				t.Errorf("game %d row %d cell %d is %q, want %q", g.Seed, p, i, row[i], w)
			}
		}
		// The deck_ columns sum to the main-deck size, and each count
		// group matches its list's size.
		sum := func(offset int, cards []string) int {
			total := 0
			for _, v := range row[offset : offset+len(names)] {
				n := 0
				fmt.Sscanf(v, "%d", &n)
				total += n
			}
			if total != len(cards) {
				t.Errorf("game %d row %d: group at %d counts %d cards, want %d", g.Seed, p, offset, total, len(cards))
			}
			return total
		}
		sum(18, g.Main[p])
		sum(18+len(names), g.Sideboard[p])
		sum(18+2*len(names), g.OpeningHand[p])
		sum(18+3*len(names), g.Drawn[p])
	}
	checkRow(rows[1], 0, games[0], "W")
	checkRow(rows[2], 1, games[0], "WUB")
	checkRow(rows[3], 0, games[2], "W")
	checkRow(rows[4], 1, games[2], "WUB")

	// A card outside the universe is an error naming it, before anything
	// is written.
	bad := games
	bad[0].Main[0] = append(append([]string(nil), main0...), "Definitely Not A Card")
	var sink bytes.Buffer
	err = l.GameDataCSV(&sink, bad, "FDN", "gorge_lab", names, true)
	if err == nil {
		t.Fatalf("GameDataCSV accepted a card outside the universe")
	}
	if !strings.Contains(err.Error(), "Definitely Not A Card") {
		t.Errorf("error %q does not name the card", err)
	}
	if sink.Len() != 0 {
		t.Errorf("the failed export wrote %d bytes", sink.Len())
	}
}

// TestWriteDecksJSON round-trips the decks file: one array, input order,
// six fixed keys per record, build null for a list deck.
func TestWriteDecksJSON(t *testing.T) {
	decks := []Deck{
		{
			Name: "deck one", Set: "TST", Source: "build:best23@1", DraftID: "7-s0",
			File:  mustFile(t, "deck one", "TST", map[string]int{"Plains": 2}),
			Build: &build.Deck{Main: []string{"Plains", "Plains"}, Basics: [5]int{2, 0, 0, 0, 0}},
		},
		{
			Name: "list deck", Source: "list",
			File: mustFile(t, "list deck", "", map[string]int{"Island": 1}),
		},
	}
	var buf bytes.Buffer
	if err := WriteDecksJSON(&buf, decks); err != nil {
		t.Fatalf("WriteDecksJSON: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "]\n") {
		t.Errorf("file does not end in a newline")
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("%d records, want 2", len(got))
	}
	first := got[0]
	for k, want := range map[string]any{
		"name": "deck one", "set": "TST", "source": "build:best23@1", "draft_id": "7-s0",
	} {
		if first[k] != want {
			t.Errorf("record 0 %s is %v, want %v", k, first[k], want)
		}
	}
	file, ok := first["file"].(map[string]any)
	if !ok {
		t.Fatalf("record 0 file is %T", first["file"])
	}
	cards, ok := file["cards"].([]any)
	if !ok || len(cards) != 1 {
		t.Fatalf("record 0 file cards are %v", file["cards"])
	}
	entry := cards[0].(map[string]any)
	if entry["name"] != "Plains" || entry["count"] != float64(2) {
		t.Errorf("record 0 file card is %v, want Plains x2", entry)
	}
	if first["build"] == nil {
		t.Errorf("record 0 build is null, want the built deck")
	}
	second := got[1]
	if second["build"] != nil {
		t.Errorf("record 1 build is %v, want null", second["build"])
	}
	if second["set"] != "" {
		t.Errorf("record 1 set is %v, want the empty string", second["set"])
	}
}

// mustFile turns a name→count map into a deck file for the JSON test.
func mustFile(t *testing.T, name, set string, cards map[string]int) deck.File {
	t.Helper()
	_ = set
	f := deck.File{Name: name, Format: "limited"}
	for _, n := range sortedKeys(cards) {
		f.Cards = append(f.Cards, deck.Entry{Name: n, Count: cards[n]})
	}
	return f
}

// sortedKeys lists a map's keys sorted, so the file is deterministic.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
