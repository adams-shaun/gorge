package policynet

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// LabelExtras is the pn12 label-record extension (schema 3): enough raw
// state to re-encode one label under every feature set and action encoding
// of the pn12 grid without regenerating the corpus. cmd/searchteacher writes
// it under -label-extras; it is the "extras" object of the record.
//
// The Diag* fields are HIDDEN INFORMATION read off the real engine at the
// decision (the opponent's hand, the next cards of both libraries). They
// exist only for the diagnostic feature sets (FeaturesMZOppHand /
// FeaturesMZOracle): never encoded by a checkpointable set, never read by a
// seat.
type LabelExtras struct {
	DiagOppHand       []view.CardView `json:"diag_opp_hand"`
	DiagOppLibraryTop []string        `json:"diag_opp_library_top"`
	DiagOwnLibraryTop []string        `json:"diag_own_library_top"`
	// FollowTargets is, for each offered option whose submission (on a clone
	// of the engine) leads the same seat into a target decision, that target
	// decision's options and the default bot's pick on the clone: the legal
	// (card, target) pairs of the joint action encoding.
	FollowTargets []FollowTarget `json:"follow_targets,omitempty"`
	// ChosenTarget is the target decision the seat actually answered in the
	// game right after this decision (nil when the answer led to none): the
	// target the teacher's chosen cast went on to take.
	ChosenTarget *FollowTarget `json:"chosen_target,omitempty"`
}

// FollowTarget is one follow-up target decision: Option is the index (in the
// label's Options) of the option that led to it, Targets the target
// decision's full option list, Min/Max its choice bounds and Choices the
// answer (the bot's on a clone, or the in-game answer for ChosenTarget).
type FollowTarget struct {
	Option  int               `json:"option"`
	Targets []decision.Option `json:"targets"`
	Min     int               `json:"min"`
	Max     int               `json:"max"`
	Choices []int             `json:"choices"`
}

// LoadOptions selects how LoadWith encodes a corpus. The zero value is Load's
// encoding exactly (FeaturesV1, the split action encoding, every record).
type LoadOptions struct {
	Features FeatureSet
	// Joint expands every priority option that leads to a target decision
	// into one option per legal target (Example.JointCard maps them back).
	Joint bool
	// Keep, when non-nil, filters records by (pair, game index) before they
	// are decoded any further; a record it rejects is not counted as loaded.
	Keep func(pair string, gameIndex int) bool
}

// leanRecord is labelRecord without the board snapshot (the encoders never
// read it) plus the pn12 extras.
type leanRecord struct {
	RecordType    string            `json:"record_type"`
	SchemaVersion int               `json:"schema_version"`
	Pair          string            `json:"pair"`
	GameIndex     int               `json:"game_index"`
	Seed          uint64            `json:"seed"`
	Sequence      uint64            `json:"decision_sequence"`
	Seat          state.PlayerID    `json:"seat"`
	Kind          decision.Kind     `json:"kind"`
	Turn          int32             `json:"turn"`
	View          json.RawMessage   `json:"view"`
	Options       []decision.Option `json:"options"`
	Candidates    []labelCandidate  `json:"candidates"`
	TeacherChoice int               `json:"teacher_choice"`
	BotIndex      int               `json:"bot_index"`
	Margin        float64           `json:"margin"`
	Outcome       float64           `json:"outcome"`
	OutcomeKnown  bool              `json:"outcome_known"`
	Extras        *LabelExtras      `json:"extras"`
}

// LoadWith is Load under LoadOptions. With the zero LoadOptions it produces
// the same examples Load does. A diagnostic feature set or the joint
// encoding on a record with no extras simply finds nothing extra to encode
// (a schema 2 record carries none).
func LoadWith(path string, lo LoadOptions) ([]Example, Stats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Stats{}, fmt.Errorf("opening label corpus: %w", err)
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var stream io.Reader = r
	if magic, err := r.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, Stats{}, fmt.Errorf("reading gzipped label corpus: %w", err)
		}
		defer zr.Close()
		stream = zr
	}
	dec := json.NewDecoder(stream)
	var out []Example
	var stats Stats
	for {
		var rec leanRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return nil, stats, fmt.Errorf("label corpus record %d: %w", stats.Records+1, err)
		}
		stats.Records++
		if rec.RecordType != LabelRecordType {
			return nil, stats, fmt.Errorf("label corpus record %d: wrong record_type %q, want %q", stats.Records, rec.RecordType, LabelRecordType)
		}
		if !LabelSchemaAccepted(rec.SchemaVersion) {
			return nil, stats, fmt.Errorf("label corpus record %d: unsupported schema_version %d, want one of %v", stats.Records, rec.SchemaVersion, AcceptedLabelSchemaVersions)
		}
		if lo.Keep != nil && !lo.Keep(rec.Pair, rec.GameIndex) {
			continue
		}
		if len(rec.Candidates) == 0 {
			stats.Skipped++
			continue
		}
		if len(rec.View) == 0 || bytes.Equal(bytes.TrimSpace(rec.View), []byte("null")) {
			return nil, stats, fmt.Errorf("label corpus record %d: missing view", stats.Records)
		}
		var v view.View
		if err := json.Unmarshal(rec.View, &v); err != nil {
			return nil, stats, fmt.Errorf("label corpus record %d: decoding view: %w", stats.Records, err)
		}
		lr := labelRecord{Options: rec.Options, Candidates: rec.Candidates, TeacherChoice: rec.TeacherChoice, BotIndex: rec.BotIndex}
		targets := optionTargets(&lr)
		picks := botPicks(&lr)
		var diag *Diag
		if rec.Extras != nil {
			diag = &Diag{OppHand: rec.Extras.DiagOppHand, OppLibraryTop: rec.Extras.DiagOppLibraryTop, OwnLibraryTop: rec.Extras.DiagOwnLibraryTop}
		}
		ex := Example{
			Pair: rec.Pair, GameIndex: rec.GameIndex, Seed: rec.Seed, Sequence: rec.Sequence,
			Seat: rec.Seat, Kind: rec.Kind, Turn: rec.Turn, Margin: rec.Margin,
			TeacherChoice: rec.TeacherChoice, BotIndex: rec.BotIndex,
			State: EncodeStateWith(lo.Features, v, rec.Seat, diag),
		}
		if rec.SchemaVersion >= 2 && rec.OutcomeKnown {
			ex.Outcome, ex.HasOutcome = rec.Outcome, true
			stats.WithOutcome++
		}
		if rec.TeacherChoice >= 0 && rec.TeacherChoice < len(rec.Candidates) {
			ex.TeacherValue, ex.HasTeacherValue = rec.Candidates[rec.TeacherChoice].Value, true
		}
		base := make([]Option, len(rec.Options))
		for i := range rec.Options {
			base[i] = EncodeOptionWith(lo.Features, v, rec.Seat, rec.Kind, rec.Options[i], i, len(rec.Options))
			base[i].Target = targets[i]
			base[i].BotPick = picks[i]
		}
		if lo.Joint && rec.Kind == decision.KPriority && rec.Extras != nil && len(rec.Extras.FollowTargets) > 0 {
			ex.Options, ex.JointCard = expandJoint(v, &rec, base)
		} else {
			ex.Options = base
		}
		out = append(out, ex)
	}
	return out, stats, nil
}

// playedOption is the option the seat actually played at this decision: the
// teacher's chosen candidate (the bot's own when the teacher kept it), when
// it is a single option.
func playedOption(rec *leanRecord) int {
	if rec.TeacherChoice < 0 || rec.TeacherChoice >= len(rec.Candidates) {
		return -1
	}
	if c := rec.Candidates[rec.TeacherChoice].Choices; len(c) == 1 {
		return c[0]
	}
	return -1
}

// sameTarget is target-option identity across two offers of the same target
// decision (the clone's and the game's): kind, object and player.
func sameTarget(a, b decision.Option) bool {
	return a.Kind == b.Kind && a.Obj == b.Obj && a.Player == b.Player && a.Label == b.Label
}

// expandJoint builds the joint (card, target) option list. An option with a
// follow-up target decision becomes one option per legal target; every other
// option is kept as is. A joint option is Preferred (BotPick) when its card
// is and its target is in that card's target answer: the in-game answer for
// the card the seat actually played, the default bot's clone answer for any
// other card. The card-level target (Labelled, Value) is shared by every
// expansion of one card.
func expandJoint(v view.View, rec *leanRecord, base []Option) ([]Option, []int) {
	follow := make([]*FollowTarget, len(base))
	for i := range rec.Extras.FollowTargets {
		ft := &rec.Extras.FollowTargets[i]
		if ft.Option >= 0 && ft.Option < len(follow) && len(ft.Targets) > 0 {
			follow[ft.Option] = ft
		}
	}
	played := playedOption(rec)
	idx := cardIndex(v, rec.Seat)
	var opts []Option
	var card []int
	for i, o := range base {
		ft := follow[i]
		if ft == nil {
			opts = append(opts, o)
			card = append(card, i)
			continue
		}
		chosen := make([]bool, len(ft.Targets))
		marked := false
		if i == played && rec.Extras.ChosenTarget != nil {
			for _, c := range rec.Extras.ChosenTarget.Choices {
				if c < 0 || c >= len(rec.Extras.ChosenTarget.Targets) {
					continue
				}
				want := rec.Extras.ChosenTarget.Targets[c]
				for k, t := range ft.Targets {
					if sameTarget(t, want) {
						chosen[k], marked = true, true
					}
				}
			}
		}
		if !marked {
			for _, c := range ft.Choices {
				if c >= 0 && c < len(chosen) {
					chosen[c], marked = true, true
				}
			}
		}
		if !marked {
			chosen[0] = true
		}
		src := ""
		if ref, ok := idx[rec.Options[i].Obj]; ok && rec.Options[i].Obj != 0 {
			src = ref.cv.Name
		}
		for k, t := range ft.Targets {
			jo := o
			jo.Hashed = append(append([]Feature(nil), o.Hashed...), jointTargetFeatures(src, t, idx, rec.Seat)...)
			jo.Target.Preferred = o.Target.Preferred && chosen[k]
			jo.BotPick = o.BotPick && chosen[k]
			opts = append(opts, jo)
			card = append(card, i)
		}
	}
	return opts, card
}

// jointTargetFeatures are the hashed rows a joint option adds for its target
// (appended after the card option's own hashed ids, in this fixed order).
func jointTargetFeatures(src string, t decision.Option, idx map[state.ObjID]cardRef, seat state.PlayerID) []Feature {
	var out []Feature
	push := func(s string) { out = append(out, Feature{Row: hashID(s), Value: 1}) }
	push("jt|tkind|" + t.Kind)
	if p, ok := t.Obj.PlayerRef(); ok || t.Kind == "player" {
		if !ok {
			p = t.Player
		}
		side := "opp"
		if p == seat {
			side = "me"
		}
		push("jt|player|" + side)
		push("jt|" + src + "|player|" + side)
		return out
	}
	ref, ok := idx[t.Obj]
	if !ok {
		push("jt|tgt|unknown")
		return out
	}
	cv := ref.cv
	side := "theirs"
	if ref.mine {
		side = "mine"
	}
	push("jt|tgt|" + cv.Name)
	push("jt|" + src + "\x1f" + cv.Name)
	push("jt|tgt|" + side)
	push("jt|" + src + "|" + side)
	for _, w := range strings.Fields(cv.Types) {
		push("jt|tgt|" + side + "|type|" + w)
	}
	if hasTypeWord(cv.Types, "Creature") {
		pt := "p" + mzBucket(cv.Power, 9) + "|t" + mzBucket(cv.Toughness-cv.Damage, 9)
		push("jt|tgt|" + side + "|cr|" + pt)
		push("jt|" + src + "|" + side + "|cr|" + pt)
	}
	if cv.Tapped {
		push("jt|tgt|tapped")
	}
	if cv.Attacking {
		push("jt|tgt|attacking")
	}
	for _, k := range cv.Keywords {
		push("jt|tgt|kw|" + strings.ToLower(k))
	}
	return out
}
