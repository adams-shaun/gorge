package protocol

import "encoding/json"

// PairedBotStatus is GET /api/tables/{t}/paired-bot: whether a seat at this
// table may ask a bot to evaluate its decisions. It carries configuration
// only, so every seat can read it and see that a table allows it.
type PairedBotStatus struct {
	Offered bool `json:"offered"`
	// Reason says, in plain words, why this table does not offer what the
	// server does. A server without the option has none.
	Reason string `json:"reason,omitempty"`
	// Policy names the bot.
	Policy string `json:"policy,omitempty"`
	// Shared marks a table with more than one human that allows it: any of
	// those players may be asking.
	Shared bool `json:"shared,omitempty"`
}

// PairedBotChoice is one paired bot a new table may choose
// (Capabilities.PairedBots).
type PairedBotChoice struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	// Formats are the construction formats it plays; empty means all.
	Formats []string `json:"formats,omitempty"`
}

// The states of an Advice.
const (
	// AdviceReady: Explain holds the bot's evaluation.
	AdviceReady = "ready"
	// AdviceNone: the bot does not evaluate this kind of decision.
	AdviceNone = "none"
	// AdviceRefused: the bot cannot evaluate this decision from what the seat
	// can see; Reason says why.
	AdviceRefused = "refused"
)

// Advice is GET /api/tables/{t}/matches/{k}/advice: one bot's evaluation of
// the decision pending on the requesting seat. It is computed from what that
// seat can see, is never replay input and is never hashed.
type Advice struct {
	Table  string `json:"table"`
	Match  int    `json:"match"`
	AskSeq uint64 `json:"ask_seq"`
	Seat   uint8  `json:"seat"`
	Policy string `json:"policy"`
	State  string `json:"state"`
	// Reason is plain words: why a none or refused has no evaluation, or what
	// a player should know about a ready one, such as a search that ran out of
	// time.
	Reason string `json:"reason,omitempty"`
	// Explain is a bots/explain.Note: the same shape as
	// ReviewDecision.Explain, with Choices indexing the decision's options.
	Explain json.RawMessage `json:"explain,omitempty"`
}
