package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// sacEachValidParts reads one Sacrifice node's SacEachValid$ flag together
// with the SacValid$ list it splits, parsed once per call site: True
// sacrifices one permanent per LISTED TYPE, part by part (Tomb of
// Annihilation's Oubliette room, "Discard a card and sacrifice an artifact, a
// creature, and a land"; the card-folder spellings Catch // Release, Perilous
// Predicament, To the Slaughter and Structural Collapse carry the same
// shape). The list grammar is eachAlternatives's flat " & " split
// (filter.go): no type word or predicate token in this grammar is or
// contains " & ", so the flat split is the top-level split. Only
// SacEachValid$ True takes the split, and only a spec that actually carries
// " & " needs it -- a single-type SacValid$ is one part and is the ordinary
// spec. A list with an empty entry is not a list: it returns false so the
// whole spec keeps its old fail-closed behaviour (the " & " head matches
// nothing) rather than half-matching, exactly like eachAlternatives.
func sacEachValidParts(sa *cards.SA) ([]string, bool) {
	if !isTrue(sa.ParamStr(cards.PKSacEachValid)) {
		return nil, false
	}
	sv := strings.TrimSpace(sa.ParamStr(cards.PKSacValid))
	if !strings.Contains(sv, " & ") {
		return nil, false
	}
	parts := strings.Split(sv, " & ")
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return nil, false
		}
	}
	return parts, true
}

// sacrificeEachValid is the registered Sacrifice handler: a SacEachValid$
// node's sacrifices run one ordinary Sacrifice per listed type, part by part
// ("Discard a card and sacrifice an artifact, a creature, and a land", Tomb
// of Annihilation's Oubliette room; Catch // Release, Perilous Predicament,
// To the Slaughter and Structural Collapse carry the same shape). Each part
// is effSacrifice over that part alone (sacEachValidPartSA below), so every
// spelling the primitive reads -- the ask, the Amount$, the Optional$ --
// reads the part exactly once. The parts resolve in list order: a part's
// players choose and that part's batch leaves the battlefield before the
// next part's pool is taken, and a permanent answering two parts leaves with
// the first part that takes it and is never taken again. The UnlessCost$
// gate ran in Resolve before this body, exactly once for the whole node; the
// plain node falls through to effSacrifice untouched.
func sacrificeEachValid(h Host, c *Ctx, sa *cards.SA) {
	parts, ok := sacEachValidParts(sa)
	if !ok {
		effSacrifice(h, c, sa)
		return
	}
	for i, part := range parts {
		ps := sacEachValidPartSA(sa, part)
		if i > 0 {
			// The earlier part's departures changed the board (an animator
			// that left ends its animation of another permanent), and Resolve
			// published the layer tables once for the whole node: take them
			// again so this part's pool reads the board as it now stands.
			c.Layers = layerTablesFor(h, ps, c)
		}
		effSacrifice(h, c, ps)
	}
}

// sacEachValidPartSA is one part's Sacrifice node: the source node with its
// params copied, SacValid$ the single part and SacEachValid$ gone, so the
// recursion runs the ordinary single-spec sacrifice for it. The copy shares
// every other field with the node it came from (the sub-ability chain, the
// compiled catalog, the bound ParamSet); the fresh params map is no longer
// the set the ParamSet was bound to, so every parameter read falls back to
// the map -- the path every unbound node already takes.
func sacEachValidPartSA(sa *cards.SA, part string) *cards.SA {
	m := make(map[string]string, len(sa.Params))
	for k, v := range sa.Params {
		m[k] = v
	}
	m[cards.PKSacValid.String()] = part
	delete(m, cards.PKSacEachValid.String())
	p := *sa
	p.Params = m
	return &p
}
