package neutral

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// svarValue evaluates the subset of Forge's SVar value grammar the neutral
// world can read from an observation alone: a Count$ over cards in a zone the
// observation carries, with an arithmetic suffix. A body outside that subset
// returns ok=false, and the caller keeps the unknown-is-not-zero behaviour.
// It exists because the target-effect projection needs a dynamic damage
// amount (Feast of Flesh's NumDmg$ X is "1 plus the number of cards named
// Feast of Flesh in all graveyards") that the v2 decision carries nowhere.
func (s *Session) svarValue(o *protocol.Observation, body string) (int32, bool) {
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
	n, ok := s.countExpr(o, strings.TrimSpace(expr))
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

func (s *Session) countExpr(o *protocol.Observation, expr string) (int32, bool) {
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
			return s.countZone(o, z.zone, spec), true
		}
	}
	// "Count$Valid <spec>" names the battlefield by default in Forge.
	if spec, ok := strings.CutPrefix(rest, "Valid "); ok {
		return s.countZone(o, "battlefield", strings.TrimSpace(spec)), true
	}
	return 0, false
}

func (s *Session) countZone(o *protocol.Observation, zone, spec string) int32 {
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
			if f := s.B.face(r.CardName, false, nil); f != nil && matchesAnySpec(f, spec) {
				n++
			}
		}
	}
	return n
}

// matchesAnySpec is the comma-alternative arm of Forge's filter grammar.
func matchesAnySpec(f *cards.Face, spec string) bool {
	if strings.TrimSpace(spec) == "" {
		return true
	}
	for _, alt := range strings.Split(spec, ",") {
		if eachFaceMatches(f, strings.TrimSpace(alt)) {
			return true
		}
	}
	return false
}
