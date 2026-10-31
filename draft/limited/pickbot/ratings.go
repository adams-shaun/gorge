package pickbot

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/adams-shaun/gorge/draft/limited"
)

// The embedded 17Lands rating files, one JSON per set code. FDN.json is CC
// BY 4.0 and carries its attribution in the file; never add a file here
// that is not licensed for redistribution.
//
//go:embed ratings/*.json
var ratingFiles embed.FS

// ratedCard is one card's entry in a rating file.
type ratedCard struct {
	GihWr    float64 `json:"gih_wr"`
	GihGames int     `json:"gih_games"`
}

type ratedSet struct {
	Set         string               `json:"set"`
	Metric      string               `json:"metric"`
	Source      string               `json:"source"`
	License     string               `json:"license"`
	Attribution string               `json:"attribution"`
	Cards       map[string]ratedCard `json:"cards"`
}

// rated is every embedded rating file by set code, parsed once at init.
var rated = map[string]ratedSet{}

func init() {
	names, err := ratingFiles.ReadDir("ratings")
	if err != nil {
		panic(fmt.Sprintf("pickbot: rating files unreadable: %v", err))
	}
	for _, n := range names {
		code := n.Name()
		code = code[:len(code)-len(".json")]
		b, err := ratingFiles.ReadFile("ratings/" + n.Name())
		if err != nil {
			panic(fmt.Sprintf("pickbot: rating file %s unreadable: %v", n.Name(), err))
		}
		var s ratedSet
		if err := json.Unmarshal(b, &s); err != nil {
			panic(fmt.Sprintf("pickbot: rating file %s malformed: %v", n.Name(), err))
		}
		rated[code] = s
	}
}

// RatedSet reports whether an embedded rating file covers the set.
func RatedSet(set string) bool {
	_, ok := rated[set]
	return ok
}

// RawRating looks up one card's raw rating: the set's metric value (a win
// rate for the embedded gih_wr files) when a rating file covers the set,
// and no value when it does not — the heuristic path is the caller's.
func RawRating(set, name string) (ratedCard, bool) {
	s, ok := rated[set]
	if !ok {
		return ratedCard{}, false
	}
	e, ok := s.Cards[name]
	return e, ok
}

// Attribution is the rating file's attribution line for a set, for the UI
// to show wherever a 17Lands number is displayed. Empty when the set is
// unrated.
func Attribution(set string) string {
	return rated[set].Attribution
}

// Rating is one card's raw rating: the rating file's metric for a rated
// set, the heuristic's points for any other. Card facts come through
// lookup, which is only consulted on the heuristic path.
func Rating(set string, ref limited.CardRef, lookup CardLookup) float64 {
	if e, ok := RawRating(set, ref.Name); ok {
		return e.GihWr
	}
	return Heuristic(lookup(ref.Name), ref.Rarity)
}
