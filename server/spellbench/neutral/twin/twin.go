// Package twin pairs two gorge agents behind one v2 seat. The view agent reads
// gorge's own view from x_gorge_view_v1; the neutral agent rebuilds the same
// decision from the bare observation and candidates, as it would for any other
// engine. Both answer every choose request, the view agent's pick is played
// (the game stays on the native trajectory, teacher-forced) and each
// disagreement is counted by the decision's kind and handed to OnDisagree.
package twin

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// Floated counts the decisions left out of the comparison: the view agent
// floated mana by hand (activate_mana_ability), which an engine_autopay
// neutral seat never does.
const Floated = "(gorge floated mana)"

// ChosenColour counts the decisions left out of the comparison: a permanent
// in play makes mana of a colour chosen as it entered. The observation does
// not carry that choice (permanent.chosen is null), so the neutral planner
// cannot see what the source makes. It is only counted when the Twin was
// given a card registry (SetRegistry).
const ChosenColour = "(chosen-colour source in play)"

// Disagreement is one decision the two agents answered differently.
type Disagreement struct {
	Kind string // the decision kind it is counted under
	// Index counts the choose requests the Twin has seen, from 0, in the order
	// the host posed them: the same game replays to the same Index.
	Index  int
	Seat   string
	Native int // the view agent's candidate_id
	V2     int // the neutral agent's candidate_id, -1 when it answered an error
	// NativeSem and V2Sem are the two picked candidates' semantics.
	NativeSem, V2Sem json.RawMessage
	V2Error          string
	// Request is the v2 choose request both agents were asked.
	Request json.RawMessage
}

// Twin tallies one or more games. It is not safe for concurrent use: a game
// is played by one goroutine.
type Twin struct {
	Agree, Total map[string]int
	Examples     map[string][]string
	// OnDisagree, when non-nil, is called for every disagreement.
	OnDisagree func(Disagreement)
	// reg, when set, recognises sources whose mana depends on a colour chosen
	// as they entered (see ChosenColour).
	reg *cards.Registry
	n   int
}

func New() *Twin {
	return &Twin{Agree: map[string]int{}, Total: map[string]int{}, Examples: map[string][]string{}}
}

// SetRegistry gives the Twin the card registry it needs to recognise a source
// whose mana depends on a colour chosen as it entered; without one it does not
// skip those decisions.
func (tw *Twin) SetRegistry(reg *cards.Registry) { tw.reg = reg }

// Seat binds one seat's pair of agents to the tally. It is a minihost.Link.
type Seat struct {
	tw            *Twin
	view, neutral *agent.Server
}

func (tw *Twin) Seat(view, neutral *agent.Server) *Seat {
	return &Seat{tw: tw, view: view, neutral: neutral}
}

type answer struct {
	ResponseType string `json:"response_type"`
	Selection    struct {
		CandidateID int `json:"candidate_id"`
	} `json:"selection"`
	Error struct {
		Code, Message string
	} `json:"error"`
}

func (s *Seat) Round(req []byte) ([]byte, error) {
	tw := s.tw
	a, _ := s.view.Round(req)
	b, _ := s.neutral.Round(req)
	var q struct {
		RequestType string                `json:"request_type"`
		Decision    protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(req, &q); err != nil || q.RequestType != "choose" {
		if q.RequestType == "game_start" && !strings.Contains(string(b), `"ack"`) {
			return nil, fmt.Errorf("neutral game_start: %s", b)
		}
		return a, nil
	}
	idx := tw.n
	tw.n++
	cands := q.Decision.Candidates
	kind := cands[0].Semantic.Kind
	if q.Decision.Context.Kind == "priority" {
		kind = "priority"
	}
	if len(cands) == 1 {
		kind += "(single)"
	}
	tw.Total[kind]++
	if tw.chosenColour(&q.Decision) {
		tw.Total[kind]--
		tw.Total[ChosenColour]++
		return a, nil
	}
	var ca, cb answer
	json.Unmarshal(a, &ca)
	json.Unmarshal(b, &cb)
	if ca.Selection.CandidateID < 0 || ca.Selection.CandidateID >= len(cands) {
		return a, nil // the host rejects the view agent's pick; nothing to compare
	}
	picked := cands[ca.Selection.CandidateID].Semantic
	if picked.Kind == "activate_mana_ability" {
		tw.Total[kind]--
		tw.Total[Floated]++
		return a, nil
	}
	v2 := cb.Selection.CandidateID
	if cb.ResponseType != "choice" {
		v2 = -1
	}
	if v2 == ca.Selection.CandidateID ||
		v2 >= 0 && v2 < len(cands) && sameCard(picked, cands[v2].Semantic) {
		tw.Agree[kind]++
		return a, nil
	}
	semNative, _ := json.Marshal(picked)
	semV2 := []byte("null")
	if v2 >= 0 && v2 < len(cands) {
		semV2, _ = json.Marshal(cands[v2].Semantic)
	}
	if len(tw.Examples[kind]) < 14 {
		tw.Examples[kind] = append(tw.Examples[kind], fmt.Sprintf("gorge %s\n      neutral %s", semNative, semV2))
	}
	if tw.OnDisagree != nil {
		d := Disagreement{Kind: kind, Index: idx, Seat: q.Decision.ActingSeat, Native: ca.Selection.CandidateID, V2: v2,
			NativeSem: semNative, V2Sem: semV2, Request: slices.Clone(req)}
		if cb.ResponseType != "choice" {
			d.V2Error = fmt.Sprintf("%s %s: %s", cb.ResponseType, cb.Error.Code, cb.Error.Message)
		}
		tw.OnDisagree(d)
	}
	return a, nil
}

// sameCard reports whether two candidates play the same card: the engine
// hides the order of interchangeable copies in a hand, so which of them a
// seat plays first is not a decision.
func sameCard(x, y protocol.Semantic) bool {
	if x.Kind != y.Kind || x.Kind != "cast_spell" && x.Kind != "play_land" {
		return false
	}
	anon := func(s protocol.Semantic) string {
		raw, _ := json.Marshal(s)
		var m map[string]any
		json.Unmarshal(raw, &m)
		if src, ok := m["source"].(map[string]any); ok {
			delete(src, "object_id")
		}
		out, _ := json.Marshal(m)
		return string(out)
	}
	return anon(x) == anon(y)
}

// chosenColour reports whether a permanent whose mana depends on a colour
// chosen as it entered is in play.
func (tw *Twin) chosenColour(sd *protocol.SeatDecision) bool {
	if tw.reg == nil {
		return false
	}
	for _, p := range sd.Observation.Players {
		for _, r := range p.Battlefield {
			if r.CardName == nil {
				continue
			}
			c, ok := tw.reg.Lookup(*r.CardName)
			if !ok {
				continue
			}
			for _, f := range c.Faces {
				for _, a := range f.Abilities {
					if a.API == "Mana" && strings.Contains(a.Params["Produced"], "Chosen") {
						return true
					}
				}
			}
		}
	}
	return false
}
