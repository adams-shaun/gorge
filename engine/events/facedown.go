package events

import (
	"strconv"
	"strings"
)

// FaceDownEntryCounter is the MoveZone Counter value that marks a card
// entering the battlefield face down (CR 708.5) through a generic
// ChangeZone FaceDown$ True (Yedora, Grave Gardener's "return it ... face
// down. It's a Forest land."), optionally carrying the face-down set type
// and power/toughness the card text names (FaceDownSetType$,
// FaceDownPower$, FaceDownToughness$). A MoveZone whose To is the
// battlefield and whose Counter names this marker folds Object.FaceDown
// before the move and re-asserts it after, so entry grants (loyalty/lore)
// and later projections see the face-down state. It is deliberately NOT the
// manifest marker: ManifestEntryCounter is, because only the manifest
// procedure grants the CR 701.40b mana-cost turn-up.
const FaceDownEntryCounter = "entered_face_down"

// ManifestEntryCounter is the MoveZone Counter value that marks a card
// entering the battlefield face down through the manifest procedure
// (CR 701.40). It is distinct from FaceDownEntryCounter because a generic
// ChangeZone FaceDown$ True entry shares that bare marker and does not grant
// the manifest turn-face-up (CR 701.40b), and from CloakEntryCounter because
// a cloak's turn-up is CR 701.58b. It carries no payload.
const ManifestEntryCounter = "entered_manifested"

// FaceDownEntryCounterFor builds the battlefield face-down marker for a move
// that carries a folded set type and/or P/T. The empty set type and absent
// P/T produce the bare FaceDownEntryCounter (the generic face-down entry).
//
// The payload rides the Counter field (no Event struct change -- it is frozen
// and hash-chained) with a deterministic, replay-parsed grammar:
//
//	entered_face_down
//	entered_face_down;settype=Land & Forest
//	entered_face_down;settype=Artifact & Creature & Cyberman;pt=2/2
//
// The set type is the raw Forge " & "-joined value; the P/T pair is present
// only when a FaceDownPower$/FaceDownToughness$ pair resolved.
func FaceDownEntryCounterFor(setType string, power, toughness int32, hasPT bool) string {
	setType = strings.TrimSpace(setType)
	if setType == "" && !hasPT {
		return FaceDownEntryCounter
	}
	var b strings.Builder
	b.WriteString(FaceDownEntryCounter)
	if setType != "" {
		b.WriteString(";settype=")
		b.WriteString(setType)
	}
	if hasPT {
		b.WriteString(";pt=")
		b.WriteString(strconv.FormatInt(int64(power), 10))
		b.WriteByte('/')
		b.WriteString(strconv.FormatInt(int64(toughness), 10))
	}
	return b.String()
}

// FaceDownEntryFields parses a Counter value built by FaceDownEntryCounterFor.
// ok reports whether the value is a battlefield face-down marker at all; the
// lenient prefix match means a future rider appended to the same Counter still
// folds FaceDown here, while every other MoveZone Counter (the exile markers,
// WithCounters riders) is untouched. hasPT reports whether a P/T pair rode the
// payload; the set type is returned trimmed and may be empty.
func FaceDownEntryFields(counter string) (setType string, power, toughness int32, hasPT, ok bool) {
	if counter != FaceDownEntryCounter && !strings.HasPrefix(counter, FaceDownEntryCounter+";") {
		return "", 0, 0, false, false
	}
	ok = true
	rest := strings.TrimPrefix(counter, FaceDownEntryCounter)
	for part := range strings.SplitSeq(rest, ";") {
		part = strings.TrimSpace(part)
		switch {
		case part == "":
			continue
		case strings.HasPrefix(part, "settype="):
			setType = strings.TrimSpace(strings.TrimPrefix(part, "settype="))
		case strings.HasPrefix(part, "pt="):
			pt := strings.TrimPrefix(part, "pt=")
			p, t, found := strings.Cut(pt, "/")
			if !found {
				continue
			}
			pv, err1 := strconv.Atoi(strings.TrimSpace(p))
			tv, err2 := strconv.Atoi(strings.TrimSpace(t))
			if err1 != nil || err2 != nil {
				continue
			}
			power, toughness, hasPT = int32(pv), int32(tv), true
		}
	}
	return setType, power, toughness, hasPT, true
}

// CloakEntryCounter is the MoveZone Counter value that marks a card entering
// the battlefield face down via Cloak (CR 708.5's cloak variant: a 2/2 with
// ward {2}). It is the cloak battlefield face-down entry marker; unlike
// FaceDownEntryCounter it carries no payload grammar, so it is compared
// exactly rather than by prefix.
const CloakEntryCounter = "entered_cloaked"

// UnearthEntryCounter is the MoveZone Counter value that marks a card
// entering the battlefield through its K:Unearth ability (CR 702.84a). It is
// stamped by engine/effects/zone.go's applyFaceDownMarker from the expansion's
// Unearth$ True parameter and read once by rules' entry hook
// (engine/rules/unearth.go), which grants the creature haste and registers the
// end-step exile promise. It is deliberately NOT a face-down marker:
// IsFaceDownEntry returns false for it, and no corpus Unearth line combines
// the keyword with FaceDown$/ExileFaceDown$.
const UnearthEntryCounter = "entered_unearthed"

// IsFaceDownEntry reports whether a MoveZone Counter value is ANY of the
// battlefield face-down entry markers -- the generic FaceDown$ marker
// FaceDownEntryFields parses (bare or payload-bearing), the manifest marker,
// or the cloak marker. It is the one predicate every consumer that only needs
// "did this entry put the card onto the battlefield face down?" must use, so
// the fold in Apply and the rules-side guards that must skip a face-down
// entry cannot disagree about which markers count. Callers that need the
// folded set type or P/T still call FaceDownEntryFields; a manifest or cloak
// entry carries neither.
func IsFaceDownEntry(counter string) bool {
	if counter == ManifestEntryCounter || counter == CloakEntryCounter {
		return true
	}
	_, _, _, _, ok := FaceDownEntryFields(counter)
	return ok
}
