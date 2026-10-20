package agent

type PolicyAudit struct {
	Schema                           string `json:"schema"`
	Policy                           string `json:"policy"`
	BotName                          string `json:"bot_name"`
	Version                          string `json:"version"`
	GameID                           string `json:"game_id"`
	Seat                             string `json:"seat"`
	GameStarted                      bool   `json:"game_started"`
	GameOverReceived                 bool   `json:"game_over_received"`
	NativeDecisions                  int    `json:"native_decisions"`
	ForcedNatives, FallbackNatives   int
	ForcedSubsteps, FallbackSubsteps int
}

// Audit contains only public session labels and aggregate adapter work.
// It neither reads an engine nor serializes the records' underlying objects.
func (s *Server) Audit() PolicyAudit {
	a := PolicyAudit{Schema: "spellbench-gorge-policy-audit/v1", Policy: s.policy,
		BotName: s.identity.Name, Version: Version(), GameID: s.gameID, Seat: s.auditSeat,
		GameStarted: s.bot != nil, GameOverReceived: s.gameOverReceived,
		NativeDecisions: len(s.records)}
	for _, r := range s.records {
		if r.Forced > 0 {
			a.ForcedNatives++
		}
		if r.Fallbacks > 0 {
			a.FallbackNatives++
		}
		a.ForcedSubsteps += r.Forced
		a.FallbackSubsteps += r.Fallbacks
	}
	return a
}
