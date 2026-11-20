package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// kgenPassThroughExtra admits keyword heads beyond the automatic pass-through
// (lower_kwpassthrough.go): a head gorge's rules read by name
// (cards.KeywordExpansionOnly false, its entry in engine/cards
// keywordRulesReads) whose kernel replication was proven per head, with the
// file:line the kernel replicates and the lockstep repro that exercises the
// read. Empty at T05: every expansion-only head rides the automatic
// pass-through, and no rules-read head has a reviewed admission yet.
var kgenPassThroughExtra = map[string]string{}

// passThroughKeyword is lowerKeywords' last case before the default: it
// consumes a printed K: node whose head's ENTIRE rules meaning is the
// expansion its registered expander minted (cards.KeywordExpansionOnly), and
// whose expansion lines are on the face -- the keyword lowers nothing itself
// and the expansion's own abilities, triggers, statics and replacements lower
// like any printed line. It returns false (the node keeps refusing) when
//   - the head is one of simpleKeywords or an arm above this case: those win
//     first, so their lowerings and their LIR stay byte-identical (the
//     additive rule; the simpleKeywords check is the design's explicit guard);
//   - the head is not expansion-only and carries no reviewed admission in
//     kgenPassThroughExtra (the default stays a refusal);
//   - the face carries no expansion line of k, so a printed keyword whose
//     expander produced nothing would drop silently.
//
// The keyword gets no kop bit: a filter word over it (with<Head>, gorge's
// hasKeyword) still refuses at the filter compiler, as it does today; T13b
// ports those words.
func (lc *lowerCtx) passThroughKeyword(node, k, head string) bool {
	_ = node // the node stays unused: the pass-through consumes it without a refusal
	if simpleKeywords[head] {
		return false
	}
	if !cards.KeywordExpansionOnly(head) && kgenPassThroughExtra[head] == "" {
		return false
	}
	return lc.hasKeywordLine(k) || lc.hasKeywordTrigger(k) ||
		lc.hasKeywordStatic(k) || lc.hasKeywordRepl(k)
}

// hasKeywordTrigger reports whether the face has the trigger a keyword line
// expanded into: its KeywordLine$ is the line (engine/cards addKeywordTrigger)
// or, on an expansion that tags only the head, its Keyword$ is the head.
func (lc *lowerCtx) hasKeywordTrigger(k string) bool {
	head := cards.KeywordHead(k)
	for _, t := range lc.face.Triggers {
		if t.Params["KeywordLine"] == k || t.Params["Keyword"] == head {
			return true
		}
	}
	return false
}

// hasKeywordStatic reports whether the face has the static a keyword line
// expanded into (its KeywordLine$ is the line; kwUndaunted's ReduceCost).
func (lc *lowerCtx) hasKeywordStatic(k string) bool {
	for _, s := range lc.face.Statics {
		if s.Params["KeywordLine"] == k {
			return true
		}
	}
	return false
}

// hasKeywordRepl reports whether the face has the replacement a keyword line
// expanded into (its KeywordLine$ is the line; kwAmplify's, kwDevour's Moved).
func (lc *lowerCtx) hasKeywordRepl(k string) bool {
	for _, r := range lc.face.Repls {
		if r.Params["KeywordLine"] == k {
			return true
		}
	}
	return false
}
