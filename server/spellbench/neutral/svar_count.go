package neutral

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// svarValue evaluates the subset of Forge's SVar value grammar the neutral
// world can read from an observation alone: a Count$ over cards in a zone the
// observation carries, or Count$CardCounters.<KIND> read from the source
// permanent, with an arithmetic suffix. A body outside that subset returns
// ok=false, and the caller keeps the unknown-is-not-zero behaviour. It exists
// because the target-effect projection needs a dynamic damage amount (Feast
// of Flesh's NumDmg$ X is "1 plus the number of cards named Feast of Flesh in
// all graveyards", Shrine of Burning Rage's is its charge counters) that the
// v2 decision carries nowhere. src is the ability's source, or nil when the
// caller has none; a counter read needs a battlefield permanent.
func (s *Session) svarValue(o *protocol.Observation, src *protocol.ObjectRef, body string) (int32, bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return 0, false
	}
	expr, op := body, ""
	if i := strings.LastIndex(body, "/"); i >= 0 {
		if suffix := body[i+1:]; countOpSuffix(suffix) {
			expr, op = body[:i], suffix
		}
	}
	n, ok := s.countExpr(o, src, strings.TrimSpace(expr))
	if !ok {
		return 0, false
	}
	return effects.ApplyCountOp(n, op), true
}

func countOpSuffix(op string) bool {
	for _, head := range []string{"Plus.", "Minus.", "Times.", "DivideEvenlyDown.", "DivideEvenlyUp."} {
		if strings.HasPrefix(op, head) {
			return true
		}
	}
	return false
}

func (s *Session) countExpr(o *protocol.Observation, src *protocol.ObjectRef, expr string) (int32, bool) {
	rest, ok := strings.CutPrefix(expr, "Count$")
	if !ok {
		return 0, false
	}
	for _, z := range []struct{ head, zone string }{
		{"ValidGraveyard", "graveyard"},
		{"ValidHand", "hand"},
		{"ValidBattlefield", "battlefield"},
		{"ValidExile", "exile"},
	} {
		if strings.HasPrefix(rest, z.head) {
			spec := strings.TrimSpace(rest[len(z.head):])
			return s.countZoneSpec(o, z.zone, spec)
		}
	}
	// "Count$Valid <spec>" names the battlefield by default in Forge.
	if spec, ok := strings.CutPrefix(rest, "Valid "); ok {
		return s.countZoneSpec(o, "battlefield", strings.TrimSpace(spec))
	}
	// "Count$CardCounters.<KIND>" reads the source permanent's counters
	// (engine/effects/count_body_heads.go). The wire carries counters only on
	// a battlefield permanent, so any other source stays unknown.
	if kind, ok := strings.CutPrefix(rest, "CardCounters."); ok {
		return s.cardCounters(o, src, strings.TrimSpace(kind))
	}
	return 0, false
}

// cardCounters counts the source permanent's counters of kind. ALL is
// Forge's wildcard over every kind. A source the observation does not carry
// as a battlefield permanent returns unknown, never a confident zero; a
// permanent that is present and lacks the kind is a known zero. A kind with
// a character outside the counter-name alphabet is an unsupported count
// operator (ALL/Mod.2, AGE/Thrice), not a kind, and stays unknown.
func (s *Session) cardCounters(o *protocol.Observation, src *protocol.ObjectRef, kind string) (int32, bool) {
	if src == nil || kind == "" {
		return 0, false
	}
	for i := 0; i < len(kind); i++ {
		c := kind[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' {
			continue
		}
		return 0, false
	}
	for _, p := range o.Players {
		for _, r := range p.Battlefield {
			if r.ObjectID != src.ObjectID {
				continue
			}
			if r.Permanent == nil {
				return 0, false
			}
			if strings.EqualFold(kind, "ALL") {
				var sum uint32
				for _, n := range r.Permanent.Counters {
					sum += n
				}
				return int32(sum), true
			}
			for name, n := range r.Permanent.Counters {
				if strings.EqualFold(name, kind) {
					return int32(n), true
				}
			}
			return 0, true
		}
	}
	return 0, false
}

func (s *Session) countZoneSpec(o *protocol.Observation, zone, spec string) (int32, bool) {
	if !countSpecResolvable(spec) {
		return 0, false
	}
	return s.countZone(o, zone, spec)
}

// countSpecResolvable is the count evaluator's grammar gate. It reports
// whether every alternative of spec is built only from words the evaluator
// decides; anything else makes the whole count UNKNOWN, never a confident
// zero. That includes the possession predicates (YouCtrl, YouOwn, OppCtrl,
// ...): a Count$Valid body is not the EACH search candidate list the host has
// already filtered, so treating them as always true would count both
// players' zones (or, for a hidden hand, the wrong player's cards). The
// native engine resolves those against the caster's context; the neutral
// world publishes no count rather than invent one.
func countSpecResolvable(spec string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return true
	}
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			return false
		}
		base, rest, _ := strings.Cut(alt, ".")
		if !countBaseResolvable(base) {
			return false
		}
		for _, p := range strings.Split(rest, "+") {
			if p = strings.TrimSpace(p); p != "" && !countPredicateResolvable(p) {
				return false
			}
		}
	}
	return true
}

// countBaseResolvable whitelists the base words the evaluator can apply to a
// printed face. TypeMaskOfName is the catalog type-line vocabulary (card
// types and supertypes); a subtype such as Goblin, a marker such as CARDNAME
// or a filter word such as attacking is not a type and makes the body
// unknown. "non" is Forge's negation prefix.
func countBaseResolvable(base string) bool {
	if neg, ok := strings.CutPrefix(base, "non"); ok {
		return neg != "" && countBaseResolvable(neg)
	}
	switch base {
	case "Any", "Card", "Permanent", "Spell":
		return true
	}
	return cards.TypeMaskOfName(base) != 0
}

// countPredicateResolvable whitelists the predicates the evaluator can decide
// from a printed face. Possession/context predicates are deliberately absent;
// see countSpecResolvable.
func countPredicateResolvable(p string) bool {
	if neg, ok := strings.CutPrefix(p, "non"); ok {
		return neg != "" && countPredicateResolvable(neg)
	}
	if strings.HasPrefix(p, "named") || strings.HasPrefix(p, "notnamed") {
		return true
	}
	switch p {
	case "White", "Blue", "Black", "Red", "Green":
		return true
	}
	return cards.TypeMaskOfName(p) != 0
}

func (s *Session) countZone(o *protocol.Observation, zone, spec string) (int32, bool) {
	var n int32
	for _, p := range o.Players {
		var records []protocol.ObjectRecord
		switch zone {
		case "graveyard":
			records = p.Graveyard
		case "hand":
			records = p.Hand
		case "battlefield":
			records = p.Battlefield
		case "exile":
			records = p.Exile
		}
		for _, r := range records {
			f := s.B.face(r.CardName, false, nil)
			if f == nil && r.Token {
				f = s.B.face(r.CardName, true, r.Characteristics)
			}
			if f == nil {
				// The count would silently drop an object the observation
				// carries; publish no number instead.
				return 0, false
			}
			if countFaceMatches(f, zone, spec) {
				n++
			}
		}
	}
	return n, true
}

// countFaceMatches is the comma-alternative arm of Forge's filter grammar for
// an already-resolvable spec. It reads only printed face facts; the seat
// predicates that would need the caster's context were rejected above.
func countFaceMatches(f *cards.Face, zone, spec string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return true
	}
	for _, alt := range strings.Split(spec, ",") {
		if countAltMatches(f, zone, strings.TrimSpace(alt)) {
			return true
		}
	}
	return false
}

func countAltMatches(f *cards.Face, zone, alt string) bool {
	base, rest, _ := strings.Cut(alt, ".")
	if !countBaseMatches(f, zone, base) {
		return false
	}
	for _, p := range strings.Split(rest, "+") {
		if p = strings.TrimSpace(p); p != "" && !countPredicateMatches(f, p) {
			return false
		}
	}
	return true
}

func countBaseMatches(f *cards.Face, zone, base string) bool {
	if neg, ok := strings.CutPrefix(base, "non"); ok {
		return !countBaseMatches(f, zone, neg)
	}
	switch base {
	case "Any", "Card":
		return true
	case "Permanent":
		// Every object in a battlefield zone is a permanent; a card in any
		// other zone the evaluator reads is not one.
		return zone == "battlefield"
	case "Spell":
		// The evaluator reads no stack zone, so no record is a spell.
		return false
	}
	return f.TypeLineHas(base, cards.TypeWordIDOf(base))
}

func countPredicateMatches(f *cards.Face, p string) bool {
	if neg, ok := strings.CutPrefix(p, "non"); ok {
		return !countPredicateMatches(f, neg)
	}
	if name, ok := strings.CutPrefix(p, "named"); ok {
		return strings.EqualFold(f.Name, name)
	}
	if name, ok := strings.CutPrefix(p, "notnamed"); ok {
		return !strings.EqualFold(f.Name, name)
	}
	switch p {
	case "White":
		return countPrintedColours(f)&cards.ColourWhite != 0
	case "Blue":
		return countPrintedColours(f)&cards.ColourBlue != 0
	case "Black":
		return countPrintedColours(f)&cards.ColourBlack != 0
	case "Red":
		return countPrintedColours(f)&cards.ColourRed != 0
	case "Green":
		return countPrintedColours(f)&cards.ColourGreen != 0
	}
	return f.TypeLineHas(p, cards.TypeWordIDOf(p))
}

// countPrintedColours is the face's characteristic colour as printed (CR
// 105.2): the mana cost's pips plus a colour indicator. It is not
// ColourIdentity, which also folds rules-text mana symbols and would make a
// colourless card with an off-colour activation match "Red" here.
func countPrintedColours(f *cards.Face) uint8 {
	m := cards.ManaCostColours(f.ManaCost)
	for _, part := range strings.Split(f.Colors, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "white":
			m |= cards.ColourWhite
		case "blue":
			m |= cards.ColourBlue
		case "black":
			m |= cards.ColourBlack
		case "red":
			m |= cards.ColourRed
		case "green":
			m |= cards.ColourGreen
		}
	}
	return m
}
