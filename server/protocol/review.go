package protocol

import "encoding/json"

// ReviewDecisionOptionCap bounds the options one ReviewDecision lists. A
// NameCard ask offers every card name in the corpus (~24k); the reviewer
// needs the answer, not the whole universe, so a longer list keeps its
// first ReviewDecisionOptionCap options plus every option the answer or the
// explain note names, and OptionsTotal says how many were offered.
const ReviewDecisionOptionCap = 64

// ReviewDecision is one entry of a review table's decision index, served
// only by `gorged review` at GET /api/tables/{t}/matches/{k}/decisions
// (match reviewer, design D8/T4). Entry i describes the match log's
// Intents[i]: the decision the engine offered at that point and the answer
// the seat gave. It is derived at load from the replay itself -- the
// options and labels are never stored in a record -- so it is exactly what
// the seat was shown.
//
//   - AskSeq and MadeSeq are the seqs of the intent's decision_ask and
//     decision_made events. AskSeq is the cursor the client's Decision step
//     stops on.
//   - Turn is the engine turn number; Round the round the phase bar calls
//     "Turn N".
//   - Options carry the labels the live client renders for the same pending
//     decision (the view projection's own option labels), capped at
//     ReviewDecisionOptionCap; OptionsTotal is the uncapped count.
//   - Chosen is the intent's Choices; Rest its arrange complement order.
//     Answer is a short server-built label of the answer (the chosen
//     options' labels, or the payment action a planned or announced cast
//     selected); Made is the decision_made event's text, verbatim.
//   - Explain is the record's explain note for this intent (bots/explain.Note
//     JSON) passed through untouched, or null when none was recorded.
type ReviewDecision struct {
	Intent       int             `json:"intent"`
	AskSeq       uint64          `json:"ask_seq"`
	MadeSeq      uint64          `json:"made_seq"`
	Turn         int32           `json:"turn"`
	Round        int32           `json:"round"`
	Seat         uint8           `json:"seat"`
	Kind         string          `json:"kind"`
	Prompt       string          `json:"prompt,omitempty"`
	Min          int             `json:"min"`
	Max          int             `json:"max"`
	Options      []ReviewOption  `json:"options"`
	OptionsTotal int             `json:"options_total"`
	Chosen       []int           `json:"chosen"`
	Rest         []int           `json:"rest,omitempty"`
	Answer       string          `json:"answer,omitempty"`
	Made         string          `json:"made"`
	Explain      json.RawMessage `json:"explain"`
}

// ReviewOption is one offered option of a ReviewDecision: its index in the
// decision (the number Chosen and an explain note's choices refer to), the
// label the live client shows for it, and the option kind.
type ReviewOption struct {
	Index int    `json:"index"`
	Label string `json:"label"`
	Kind  string `json:"kind,omitempty"`
}
