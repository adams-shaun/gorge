package pickbot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// PairNames names the ten colour pairs in Tuning.PairBias order.
var PairNames = [10]string{"WU", "WB", "WR", "WG", "UB", "UR", "UG", "BR", "BG", "RG"}

// Filled returns t with every zero pre-existing knob taken from
// CommittedTuning (the new drafter and builder knobs mean "off" at zero and
// stay as they are). Filled(Tuning{}) is CommittedTuning, so the zero knob
// set drafts like Committed and builds like Best23.
func (t Tuning) Filled() Tuning {
	d := CommittedTuning
	fillF := func(v *float64, def float64) {
		if *v == 0 {
			*v = def
		}
	}
	fillI := func(v *int, def int) {
		if *v == 0 {
			*v = def
		}
	}
	fillF(&t.RatingCentre, d.RatingCentre)
	fillF(&t.RatingScale, d.RatingScale)
	fillF(&t.HeuristicCentre, d.HeuristicCentre)
	fillF(&t.HeuristicScale, d.HeuristicScale)
	fillF(&t.ColourFitMax, d.ColourFitMax)
	fillI(&t.ColourFitRamp, d.ColourFitRamp)
	fillF(&t.ColourlessFit, d.ColourlessFit)
	fillI(&t.DeckSize, d.DeckSize)
	if t.MainSize == 0 && t.Lands > 0 {
		t.MainSize = t.DeckSize - t.Lands
	}
	fillI(&t.MainSize, d.MainSize)
	fillI(&t.MinCreatures, d.MinCreatures)
	fillI(&t.HighValueMV, d.HighValueMV)
	fillI(&t.MaxHighValue, d.MaxHighValue)
	fillF(&t.ShortPairPenalty, d.ShortPairPenalty)
	return t
}

// Validate refuses a filled knob set the drafter or builder cannot honour,
// naming the offending key.
func (t Tuning) Validate() error {
	f := t.Filled()
	switch {
	case f.DeckSize != 40:
		return fmt.Errorf("deck_size %d: the builder fills to 40", f.DeckSize)
	case f.MainSize < 1 || f.MainSize >= f.DeckSize:
		return fmt.Errorf("main_size %d: want 1..%d", f.MainSize, f.DeckSize-1)
	case f.Lands != 0 && f.Lands != f.DeckSize-f.MainSize:
		return fmt.Errorf("lands %d: main_size %d leaves %d", f.Lands, f.MainSize, f.DeckSize-f.MainSize)
	case f.MinCreatures < 0 || f.MinCreatures > f.MainSize:
		return fmt.Errorf("min_creatures %d: want 0..%d", f.MinCreatures, f.MainSize)
	case f.CreatureMax < 0 || f.CreatureMax > 0 && f.CreatureMax < f.MinCreatures:
		return fmt.Errorf("creature_max %d: want 0 (no cap) or at least min_creatures %d", f.CreatureMax, f.MinCreatures)
	case f.MaxHighValue < 0 || f.HighValueMV < 1:
		return fmt.Errorf("max_high_value %d / high_value_mv %d: want >= 0 / >= 1", f.MaxHighValue, f.HighValueMV)
	case f.ColourFitRamp < 0:
		return fmt.Errorf("colour_fit_ramp %d: want >= 0", f.ColourFitRamp)
	case f.Need < 0:
		return fmt.Errorf("need %v: want >= 0", f.Need)
	case f.SignalWeight < 0 || f.SignalStart < 0:
		return fmt.Errorf("signal_weight %v / signal_start %d: want >= 0", f.SignalWeight, f.SignalStart)
	}
	floor := 0
	for b := range f.CurveMin {
		if f.CurveMin[b] < 0 || f.CurveMax[b] < 0 || f.CurveMax[b] > 0 && f.CurveMin[b] > f.CurveMax[b] {
			return fmt.Errorf("curve_min/curve_max bucket %d: %d > %d", b, f.CurveMin[b], f.CurveMax[b])
		}
		floor += f.CurveMin[b]
	}
	if floor > f.MainSize {
		return fmt.Errorf("curve_min sums to %d, over main_size %d", floor, f.MainSize)
	}
	return nil
}

// ParseTuning reads a knob file: one JSON object of Tuning keys. An unknown
// key is refused with its name, and so is trailing data or a knob set
// Validate refuses. The result is what the file says (zero fields stay
// zero), so json.Marshal of it reads back identical.
func ParseTuning(b []byte) (Tuning, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var t Tuning
	if err := dec.Decode(&t); err != nil {
		return Tuning{}, fmt.Errorf("pickbot: knobs: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Tuning{}, errors.New("pickbot: knobs: trailing data after the object")
	}
	if err := t.Validate(); err != nil {
		return Tuning{}, fmt.Errorf("pickbot: knobs: %w", err)
	}
	return t, nil
}

// Hash is the first 12 hex digits of the sha256 of the filled knob set's
// canonical JSON (json.Marshal: fixed field order), so two files that mean
// the same knobs share it. Agent refs carry it as knobs@<hash>.
func (t Tuning) Hash() string {
	b, err := json.Marshal(t.Filled())
	if err != nil {
		panic(err) // a struct of numbers always marshals
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:12]
}

// NewTuned builds the drafter/builder pair one knob set names: Knobbed
// drafting and Best23 building, both under t. NewTuned(Tuning{}, l) drafts
// exactly as Committed{Lookup: l} and builds exactly as Best23{Lookup: l}.
func NewTuned(t Tuning, lookup CardLookup) (Drafter, Builder) {
	return &Knobbed{Lookup: lookup, Tuning: t}, &Best23{Lookup: lookup, Tuning: t.Filled()}
}

// ParseResearchKnobs reads the draft-research harness's knob string
// ("cfm=8;need=4;sig=2;deny=1;bias=G+3/B-2;pbias=BG+1") onto a Tuning. An
// unknown key or an unparsable value is an error naming it.
func ParseResearchKnobs(s string) (Tuning, error) {
	var t Tuning
	for _, kv := range strings.Split(s, ";") {
		key, val, _ := strings.Cut(kv, "=")
		num := func() (float64, error) {
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return 0, fmt.Errorf("drafter knob %q: %w", key, err)
			}
			return f, nil
		}
		var err error
		switch key {
		case "":
		case "sig":
			t.SignalWeight, err = num()
		case "deny":
			t.Deny, err = num()
		case "cfm":
			t.ColourFitMax, err = num()
		case "need":
			t.Need, err = num()
		case "bias":
			err = parseBiasList(val, 1, func(name string) int { return strings.Index("WUBRG", name) }, t.Bias[:])
		case "pbias":
			err = parseBiasList(val, 2, func(name string) int {
				for i, p := range PairNames {
					if p == name {
						return i
					}
				}
				return -1
			}, t.PairBias[:])
		default:
			err = fmt.Errorf("drafter knob %q", key)
		}
		if err != nil {
			return Tuning{}, err
		}
	}
	return t, t.Validate()
}

// parseBiasList reads "G+3/B-2": each item is an n-letter name index finds
// followed by a number, stored in dst[index(name)].
func parseBiasList(val string, n int, index func(string) int, dst []float64) error {
	for _, item := range strings.Split(val, "/") {
		if len(item) <= n {
			return fmt.Errorf("bias item %q", item)
		}
		i := index(item[:n])
		x, err := strconv.ParseFloat(item[n:], 64)
		if i < 0 || err != nil {
			return fmt.Errorf("bias item %q", item)
		}
		dst[i] = x
	}
	return nil
}
