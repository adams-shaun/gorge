package draftlab

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/deck"
)

// BasicNames are the five basics in the fixed W U B R G order build
// spells them: the names a game_data name universe must carry besides the
// set's sheet universe, because built decks play them.
var BasicNames = build.BasicNames

// DraftDataCSV writes an event log's picks as 17Lands draft_data, one row
// per pick, with one draft_id per drafter ("<seed>-s<seat>") and the
// event_type "gorge_pod" (a 4-seat pod) or "gorge_pod8" (8). The log is
// replayed event-for-event, so the export depends on nothing but the log
// bytes. A sealed event carries no picks, so it writes no rows (the header
// only when header is true); the column shape is event.DraftDataCSVWith's.
func DraftDataCSV(w io.Writer, eventLog []byte, names []string, header bool) error {
	ev, err := event.ReplayLog(eventLog)
	if err != nil {
		return fmt.Errorf("draftlab: DraftDataCSV: %w", err)
	}
	eventType := "gorge_pod"
	if ev.NumSeats() == 8 {
		eventType = "gorge_pod8"
	}
	return event.DraftDataCSVWith(ev, w, names, event.DraftDataOptions{
		EventType:      eventType,
		PerSeatDraftID: true,
		Header:         header,
	})
}

// gameDataColumns is the fixed 18-column prefix of a 17Lands game_data
// row; the per-card deck_/sideboard_/opening_hand_/drawn_ columns follow,
// in that group order, over the sorted name universe.
var gameDataColumns = []string{
	"expansion", "event_type", "draft_id", "draft_time", "game_time",
	"build_index", "match_number", "game_number", "rank", "opp_rank",
	"main_colors", "splash_colors", "on_play", "num_mulligans",
	"opp_num_mulligans", "opp_colors", "num_turns", "won",
}

// GameDataCSV writes games as 17Lands game_data: one row per player per
// FINISHED game (a winner; a draw and a watchdog stall are skipped — the
// manifest counts them), player 0's row first, over the sorted name
// universe names. A card outside names is an error naming it.
//
// The gorge-specific column values, in the fixed prefix's order:
//
//   - expansion is set; event_type is eventType verbatim (the caller's
//     step label, e.g. "gorge_lab", "gorge_pod8", "gorge_gauntlet").
//   - draft_id is the player's GameResult.DraftIDs entry ("" when the deck
//     was not drafted).
//   - draft_time, game_time, rank and opp_rank are EMPTY: the lab has no
//     wall clock (archtest) and no ladder.
//   - build_index is 1 (one deck build per drafter).
//   - match_number is the game's Round, 1 when 0 (an unstructured Play);
//     game_number is Game, 1 when 0 (every lab game is game 1 of its
//     series).
//   - main_colors / splash_colors spell the player's colours in WUBRG
//     order: the colour identity of the player's Main deck's NON-LAND
//     cards (deck-file names resolved against the lab's corpus; a colour
//     is main with at least 4 such cards, splash with 1..3).
//   - opp_colors is the opponent's main and splash colours in WUBRG order
//     (the concatenation of what main_colors and splash_colors would be
//     for their deck).
//   - on_play is True when the player sat in Starter's seat (the engine's
//     CR 103.1 toss decides who is actually on the play), False otherwise;
//     won is True when the player is Winner. Both are 17Lands' capitalised
//     spelling.
//   - num_mulligans / opp_num_mulligans are the player's (opponent's)
//     KMulligan asks answered "mulligan".
//   - num_turns is the game's turn count.
//   - deck_<name>, sideboard_<name>, opening_hand_<name> and
//     drawn_<name> count that name's copies in the player's Main,
//     Sideboard, OpeningHand and Drawn lists.
func (l *Lab) GameDataCSV(w io.Writer, games []GameResult, set, eventType string, names []string, header bool) error {
	cols := append([]string(nil), names...)
	sort.Strings(cols)
	universe := make(map[string]bool, len(cols))
	for _, n := range cols {
		universe[n] = true
	}
	// Validate every list before anything is written: a card outside the
	// universe is an error naming it.
	for gi := range games {
		g := &games[gi]
		for p := 0; p < 2; p++ {
			for _, group := range [4][]string{g.Main[p], g.Sideboard[p], g.OpeningHand[p], g.Drawn[p]} {
				for _, n := range group {
					if !universe[n] {
						return fmt.Errorf("draftlab: GameDataCSV: game %d player %d: card %q is outside the name universe", gi, p, n)
					}
				}
			}
		}
	}
	cw := csv.NewWriter(w)
	if header {
		hdr := append([]string(nil), gameDataColumns...)
		for _, prefix := range []string{"deck_", "sideboard_", "opening_hand_", "drawn_"} {
			for _, n := range cols {
				hdr = append(hdr, prefix+n)
			}
		}
		if err := cw.Write(hdr); err != nil {
			return fmt.Errorf("draftlab: GameDataCSV: %w", err)
		}
	}
	for gi := range games {
		g := &games[gi]
		if g.Winner < 0 {
			continue
		}
		for p := 0; p < 2; p++ {
			mainC, splashC, err := l.colours(g.Main[p])
			if err != nil {
				return fmt.Errorf("draftlab: GameDataCSV: game %d player %d: %w", gi, p, err)
			}
			oppMainC, oppSplashC, err := l.colours(g.Main[1-p])
			if err != nil {
				return fmt.Errorf("draftlab: GameDataCSV: game %d player %d: %w", gi, 1-p, err)
			}
			row := []string{
				set,
				eventType,
				g.DraftIDs[p],
				"", "", // draft_time, game_time: no clock
				"1", // build_index
				strconv.Itoa(atLeastOne(g.Round)),
				strconv.Itoa(atLeastOne(g.Game)),
				"", "", // rank, opp_rank: no ladder
				mainC,
				splashC,
				boolCell(g.Starter == p),
				strconv.Itoa(g.Mulligans[p]),
				strconv.Itoa(g.Mulligans[1-p]),
				oppMainC + oppSplashC,
				strconv.Itoa(g.Turns),
				boolCell(g.Winner == p),
			}
			row = appendCounts(row, g.Main[p], cols)
			row = appendCounts(row, g.Sideboard[p], cols)
			row = appendCounts(row, g.OpeningHand[p], cols)
			row = appendCounts(row, g.Drawn[p], cols)
			if err := cw.Write(row); err != nil {
				return fmt.Errorf("draftlab: GameDataCSV: %w", err)
			}
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("draftlab: GameDataCSV: %w", err)
	}
	return nil
}

// colours classifies a main-deck name list's colour identity: the non-land
// cards' ColourIdentity bits counted per WUBRG colour, a colour with at
// least 4 such cards main, 1..3 splash, spelled in WUBRG order. A name the
// lab's corpus does not carry is an error naming it.
func (l *Lab) colours(names []string) (main, splash string, err error) {
	var counts [5]int
	for _, n := range names {
		c, ok := l.reg.Lookup(n)
		if !ok {
			return "", "", fmt.Errorf("card %q is not in the corpus", n)
		}
		if len(c.Faces) > 0 && c.Faces[0].IsLand() {
			continue
		}
		id := c.ColourIdentity()
		for i := 0; i < 5; i++ {
			if id&(1<<uint(i)) != 0 {
				counts[i]++
			}
		}
	}
	const wubrg = "WUBRG"
	for i := 0; i < 5; i++ {
		switch {
		case counts[i] >= 4:
			main += wubrg[i : i+1]
		case counts[i] >= 1:
			splash += wubrg[i : i+1]
		}
	}
	return main, splash, nil
}

// appendCounts appends one column per col: the count of that name in
// cards.
func appendCounts(row []string, cards []string, cols []string) []string {
	counts := make(map[string]int, len(cards))
	for _, n := range cards {
		counts[n]++
	}
	for _, n := range cols {
		row = append(row, strconv.Itoa(counts[n]))
	}
	return row
}

// atLeastOne lifts a zero round/game number to 1: 17Lands counts matches
// and games from 1, and an unstructured lab game carries neither.
func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// boolCell spells a boolean the way 17Lands game_data does.
func boolCell(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// decksJSONEntry is one WriteDecksJSON array record: the deck's identity
// and provenance, its engine-loadable file and its built deck (null for a
// list deck, which has no pool).
type decksJSONEntry struct {
	Name    string      `json:"name"`
	Set     string      `json:"set"`
	Source  string      `json:"source"`
	DraftID string      `json:"draft_id"`
	File    deck.File   `json:"file"`
	Build   *build.Deck `json:"build"`
}

// WriteDecksJSON writes decks as one JSON array, in input order, of
// {name, set, source, draft_id, file, build} records; build is null for a
// list deck. The array is indented two spaces with a trailing newline, so
// the file diffs cleanly and is byte-stable across runs.
func WriteDecksJSON(w io.Writer, decks []Deck) error {
	entries := make([]decksJSONEntry, len(decks))
	for i, d := range decks {
		entries[i] = decksJSONEntry{
			Name:    d.Name,
			Set:     d.Set,
			Source:  d.Source,
			DraftID: d.DraftID,
			File:    d.File,
			Build:   d.Build,
		}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("draftlab: WriteDecksJSON: %w", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("draftlab: WriteDecksJSON: %w", err)
	}
	return nil
}
