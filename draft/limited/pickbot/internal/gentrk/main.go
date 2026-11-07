// Command gentrk regenerates draft/limited/pickbot/ratings/TRK.json, the
// TRK draft-bot rating file, from the published simulated card-strength
// list docs/site/research/card-strength/TRK.csv (list trk-nonet-11k). It is
// a build-time tool only: pickbot reads nothing but its embedded files.
//
//	go run ./internal/gentrk
//
// (wired as the //go:generate directive in the parent package; run from
// draft/limited/pickbot, where the default paths resolve). Each CSV row
// becomes one card: gih_wr is the list's prior_gih_wr (the ridge ranks
// mapped onto FDN's 17Lands GIH WR distribution), gih_games its
// games_in_hand. The output is deterministic (sorted keys, fixed indent),
// so a regenerate is byte-identical unless the list was re-published.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
)

// list is the card-strength list the CSV must carry, and the only one the
// rating file takes rows from.
const list = "trk-nonet-11k"

type card struct {
	GihWr    float64 `json:"gih_wr"`
	GihGames int     `json:"gih_games"`
}

// file mirrors pickbot's rating-file schema, field for field.
type file struct {
	Set         string          `json:"set"`
	Metric      string          `json:"metric"`
	Source      string          `json:"source"`
	License     string          `json:"license"`
	Attribution string          `json:"attribution"`
	Cards       map[string]card `json:"cards"`
}

// Generate turns the card-strength CSV into the rating file's bytes. It is
// main's testable body.
func Generate(raw []byte) ([]byte, error) {
	rows, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv: no card rows")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	for _, h := range []string{"list", "card", "games_in_hand", "prior_gih_wr"} {
		if _, ok := col[h]; !ok {
			return nil, fmt.Errorf("csv: no %q column", h)
		}
	}
	out := file{
		Set:         "TRK",
		Metric:      "sim_prior_gih_wr",
		Source:      "gorge simulated card-strength list " + list + " (docs/site/research/card-strength/TRK.md), prior_gih_wr column: ridge in-hand values mapped onto the 17Lands GIH WR scale by quantile",
		License:     "Apache-2.0",
		Attribution: "Scale: 17Lands FDN games-in-hand win rates (Data from 17Lands, https://www.17lands.com, CC BY 4.0)",
		Cards:       map[string]card{},
	}
	for i, r := range rows[1:] {
		line := i + 2
		if r[col["list"]] != list {
			return nil, fmt.Errorf("csv line %d: list %q, want %q", line, r[col["list"]], list)
		}
		name := r[col["card"]]
		if _, dup := out.Cards[name]; dup {
			return nil, fmt.Errorf("csv line %d: card %q repeats", line, name)
		}
		wr, err := strconv.ParseFloat(r[col["prior_gih_wr"]], 64)
		if err != nil {
			return nil, fmt.Errorf("csv line %d: prior_gih_wr: %w", line, err)
		}
		games, err := strconv.Atoi(r[col["games_in_hand"]])
		if err != nil {
			return nil, fmt.Errorf("csv line %d: games_in_hand: %w", line, err)
		}
		out.Cards[name] = card{GihWr: wr, GihGames: games}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func main() {
	in := flag.String("csv", "../../../docs/site/research/card-strength/TRK.csv", "the published TRK card-strength CSV")
	dst := flag.String("out", "ratings/TRK.json", "the rating file to write")
	flag.Parse()
	raw, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gentrk:", err)
		os.Exit(1)
	}
	b, err := Generate(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gentrk:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*dst, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gentrk:", err)
		os.Exit(1)
	}
}
