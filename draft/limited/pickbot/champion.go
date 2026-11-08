package pickbot

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
)

// The champion persona is a heuristic drafter whose knobs and card values
// were tuned in self-play on FDN: a colour-fit weight of 8, a signal weight
// of 2, a colour bias toward green and away from black and red, and a table
// of card values fitted from simulated games that both the drafter and the
// Best23 builder read in place of the 17Lands ratings. It is selectable by
// name and never the default. The table is licensed CC BY 4.0 and carries
// its attribution, source and measured result (with caveats) in the file's
// own fields. The persona is tuned for FDN only.
const (
	// ChampionName is the registry name of the persona.
	ChampionName = "champion"
	// ChampionSet is the only set the persona is tuned for.
	ChampionSet = "FDN"
)

//go:embed values/FDN-champion.json
var championFile []byte

const championKnobs = "cfm=8;bias=G3/B-2/R-2;sig=2"

// ChampionTuning is the champion drafter's tuning, a fresh value per call.
func ChampionTuning() Tuning {
	t, err := ParseResearchKnobs(championKnobs)
	if err != nil {
		panic("pickbot: champion knobs: " + err.Error())
	}
	return t
}

// ChampionCard is one card's entry in the value table: dv is the fitted
// win-rate-like value, n the number of samples behind it.
type ChampionCard struct {
	DV float64 `json:"dv"`
	N  int     `json:"n"`
}

// ChampionValues is the embedded value table.
type ChampionValues struct {
	Set         string                  `json:"set"`
	Persona     string                  `json:"persona"`
	Metric      string                  `json:"metric"`
	License     string                  `json:"license"`
	Attribution string                  `json:"attribution"`
	Source      string                  `json:"source"`
	Provenance  string                  `json:"provenance"`
	Cards       map[string]ChampionCard `json:"cards"`
}

const championMinCards = 200

// ParseChampionValues decodes and validates a value table. Every failure
// names the offending key or card.
func ParseChampionValues(b []byte) (ChampionValues, error) {
	var v ChampionValues
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return ChampionValues{}, fmt.Errorf("pickbot: champion values: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return ChampionValues{}, fmt.Errorf("pickbot: champion values: trailing data after the table")
	}
	bad := func(format string, a ...any) (ChampionValues, error) {
		return ChampionValues{}, fmt.Errorf("pickbot: champion values: "+format, a...)
	}
	if v.Set != ChampionSet {
		return bad("set is %q, want %q", v.Set, ChampionSet)
	}
	if v.Persona != ChampionName {
		return bad("persona is %q, want %q", v.Persona, ChampionName)
	}
	if v.Metric != "dv" {
		return bad("metric is %q, want %q", v.Metric, "dv")
	}
	if !strings.Contains(v.License, "CC BY 4.0") {
		return bad("license %q does not name CC BY 4.0", v.License)
	}
	if !strings.Contains(v.Attribution, "17Lands") || !strings.Contains(v.Attribution, "https://www.17lands.com") {
		return bad("attribution %q must credit 17Lands and https://www.17lands.com", v.Attribution)
	}
	if v.Source == "" {
		return bad("source is empty")
	}
	if v.Provenance == "" {
		return bad("provenance is empty")
	}
	if len(v.Cards) < championMinCards {
		return bad("cards has %d entries, want at least %d", len(v.Cards), championMinCards)
	}
	names := make([]string, 0, len(v.Cards))
	for n := range v.Cards {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := v.Cards[n]
		if _, ok := rated[ChampionSet].Cards[n]; !ok {
			return bad("card %q is not in the FDN card list", n)
		}
		if math.IsNaN(c.DV) || math.IsInf(c.DV, 0) || c.DV <= 0 || c.DV >= 1 {
			return bad("card %q: dv %v is not strictly between 0 and 1", n, c.DV)
		}
		if c.N < 1 {
			return bad("card %q: n %d is below 1", n, c.N)
		}
	}
	return v, nil
}

// Ratings builds the override the persona reads: the dv values rescaled
// onto the FDN 17Lands mean and spread, a missing card taking the mean.
func (v ChampionValues) Ratings() (*RatingsOverride, error) {
	m := make(map[string]map[string]float64, len(v.Cards))
	for n, c := range v.Cards {
		m[n] = map[string]float64{"dv": c.DV}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return ParseRatingsOverride(b, "values/FDN-champion.json", "dv", ChampionSet)
}

var (
	championOnce    sync.Once
	championValues  ChampionValues
	championRatings *RatingsOverride
)

func championLoad() {
	championOnce.Do(func() {
		v, err := ParseChampionValues(championFile)
		if err != nil {
			panic(err.Error())
		}
		r, err := v.Ratings()
		if err != nil {
			panic("pickbot: champion values: " + err.Error())
		}
		championValues, championRatings = v, r
	})
}

// Champion returns the parsed value table (shared; do not modify).
func Champion() ChampionValues {
	championLoad()
	return championValues
}

// ChampionRatings returns the shared read-only ratings override.
func ChampionRatings() *RatingsOverride {
	championLoad()
	return championRatings
}

// ChampionFile returns a copy of the embedded table bytes.
func ChampionFile() []byte { return append([]byte(nil), championFile...) }

// PersonaSet is the one set a persona is tuned for, "" for any set.
func PersonaSet(name string) string {
	if name == ChampionName {
		return ChampionSet
	}
	return ""
}

// CheckPersonaSet refuses a persona named for a set it is not tuned for.
func CheckPersonaSet(name, set string) error {
	if name == ChampionName && set != ChampionSet {
		return fmt.Errorf("persona %q is tuned for %s only, not %s", name, ChampionSet, set)
	}
	return nil
}

func init() {
	Register(ChampionName,
		func(l CardLookup) Drafter {
			return &Knobbed{Lookup: l, Tuning: ChampionTuning(), Ratings: ChampionRatings()}
		},
		func(l CardLookup) Builder { return &Best23{Lookup: l, Ratings: ChampionRatings()} })
}
