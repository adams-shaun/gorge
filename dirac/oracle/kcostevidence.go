package oracle

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
)

// evidencePromptPrefix opens the prompt of gorge's collect-evidence ask
// (rules cast_targets.go evidenceAsk, evidence_cost.go evidenceStage): "Exile
// evidence with total mana value N to cast X" / "... for X".
const evidencePromptPrefix = "Exile evidence with total mana value "

// evidenceTotalMet reports whether chosen reaches the collect-evidence
// threshold N of an "evidence" KChoose. Gorge's Decision carries no floor for
// it (Min is the greedy card count, not a total): the settle rejects a short
// answer and poses the ask again (evidenceAsk / answerEvidenceStage), so such
// an answer is never a whole one. Any other decision is unaffected.
func evidenceTotalMet(d *decision.Decision, chosen []int) bool {
	if d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "evidence" ||
		!strings.HasPrefix(d.Prompt, evidencePromptPrefix) {
		return true
	}
	rest := d.Prompt[len(evidencePromptPrefix):]
	end := strings.IndexByte(rest, ' ')
	if end < 0 {
		return true
	}
	need, err := strconv.Atoi(rest[:end])
	if err != nil {
		return true
	}
	tot := 0
	for _, c := range chosen {
		tot += optByIndex(d, c).Value
	}
	return tot >= need
}
