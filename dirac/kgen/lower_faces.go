package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Multi-face cards. A multi-face card lowers
// its front face as the card itself and each other face as an extra pool
// entry (a face key, carried beside the tokens so a matchup that holds the
// card holds its faces); CardDef.Faces links them. Only the layouts the
// kernel follows exactly are accepted; every other one stays refused.

// faceKeyPrefix prefixes a face entry's pool name: "face:<card>#<index>".
const faceKeyPrefix = "face:"

func faceKey(card string, i int) string {
	return faceKeyPrefix + card + "#" + strconv.Itoa(i)
}

// parseFaceKey splits a face key into its card name and face index.
func parseFaceKey(k string) (string, int, bool) {
	rest, ok := strings.CutPrefix(k, faceKeyPrefix)
	if !ok {
		return "", 0, false
	}
	i := strings.LastIndexByte(rest, '#')
	if i < 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(rest[i+1:])
	if err != nil {
		return "", 0, false
	}
	return rest[:i], n, true
}

// faceLayout reports the kernel's layout for a multi-face card, FaceNone
// when it is refused. token says c is a token script, not a pool card.
//
// Modal (CR 712.4d) is accepted for every back face: a land back is
// played from the hand as gorge's "modal_land" (rules legal.go
// modalLandBack, a FlipFace to 1 before the land play) and reset on
// leaving the battlefield (events apply_zone.go); a nonland back
// (gorge) is cast from the hand as gorge's "modal_spell" (rules
// legal.go modalSpellBack) and reset on leaving the stack for any zone but
// the battlefield (CR 712.8a) as well as on leaving the battlefield.
//
// Transforming double-faced (CR 712) is accepted for cards and tokens: a
// SetState Mode$ Transform flips it in place (CR 701.27a), a ChangeZone
// Transformed$ True return enters its back face, and a trigger Mode$
// Transformed follows the flip. A card any of whose faces is a Battle
// (CR 310) is refused by refuseBattle (lower_battle.go) unless battlesAccepted.
func faceLayout(c *cards.Card, token bool) uint8 {
	if len(c.Faces) != 2 || c.Faces[0] == nil || c.Faces[1] == nil || c.Faces[1].Name == "" {
		return kop.FaceNone
	}
	switch c.AlternateMode {
	case cards.AlternateModal:
		return kop.FaceModal
	case cards.AlternateDoubleFaced:
		return kop.FaceTransform
	}
	m := castableFaceLayout(c) // lower_faces.go
	if m == kop.FaceOmen && token {
		return kop.FaceNone // no token script is an Omen (refuse)
	}
	return m
}

// meldHalf reports the half of a meld pair that carries no meld-result
// face (AlternateMode:Meld with one face: Midnight Scavengers, whose
// partner Graf Rats holds the result): gorge plays it as a single-faced
// card (state.MeldResultFace is false; only the Meld effect, which the
// refused partner carries, reads IsMeldCard), so it lowers as one. CR 712.4c.
func meldHalf(c *cards.Card) bool {
	return c.AlternateMode == "Meld" && len(c.Faces) == 1 && c.Faces[0] != nil
}

// lowerFaces sets up the front face of an accepted multi-face card and
// names its other faces; false refuses the card.
func (lc *lowerCtx) lowerFaces(c *cards.Card) bool {
	if meldHalf(c) {
		lc.face = c.Faces[0]
		return true
	}
	m := faceLayout(c, lc.out.Token)
	if m == kop.FaceNone {
		if c.AlternateMode == "Omen" {
			switch {
			case lc.out.Token:
				lc.refuseNode("card", "Omen layout on a token script")
			case len(c.Faces) == 2 && c.Faces[0] != nil && c.Faces[0].IsLand():
				lc.refuseNode("card", "Omen card with a land front")
			default:
				lc.refuseNode("card", "Omen face 1 is not an instant or sorcery of type Omen")
			}
		}
		if c.AlternateMode == "Prepare" && len(c.Faces) == 2 && c.Faces[0] != nil && c.Faces[1] != nil {
			lc.refuseNode("card", "Prepare face 1 is not an instant or sorcery")
		}
		if c.AlternateMode == cards.AlternateSplit && len(c.Faces) == 2 && (c.Faces[0].IsRoom() || c.Faces[1].IsRoom()) {
			lc.refuseNode("card", "split card with one Room face") // lower_rooms.go
		}
		return false
	}
	if lc.refuseBattle(c) { // lower_battle.go
		return false
	}
	lc.face = c.Faces[0]
	out := lc.out
	out.FaceMode = m
	out.Faces = faceNames(out.Name, c, m)
	out.RoomSplit = roomAllLines // a plain door: every line is door Face's (lower_rooms.go)
	if m == kop.FaceRoom {
		lc.refuseRoomDoors(c)
	}
	out.Tokens = append(out.Tokens, out.Faces[1:]...)
	lc.faceTokens() // lower_faces.go
	return true
}

// faceNames is the pool names of card name's faces (CardDef.Faces): the
// front, a face key per other face and, for a Room, its state defs.
func faceNames(name string, c *cards.Card, m uint8) []string {
	out := []string{name}
	for j := 1; j < len(c.Faces); j++ {
		out = append(out, faceKey(name, j))
	}
	if m == kop.FaceRoom {
		out = append(out, roomKeys(name)...) // lower_rooms.go
	}
	return out
}

// lowerFaceEntry lowers face key k's face as its own pool entry.
func lowerFaceEntry(p *pool, k string) *lCard {
	out := &lCard{Name: k}
	name, i, ok := parseFaceKey(k)
	c := p.byName[name]
	token := false
	if c == nil {
		c, _ = p.reg.Token(name)
		token = c != nil
		out.Token = token
	}
	lc := &lowerCtx{p: p, name: k, card: c, out: out, mode: -1, known: p.knownSubtype}
	if !ok || c == nil || i <= 0 || i >= len(c.Faces) || faceLayout(c, token) == kop.FaceNone {
		lc.refuseNode("card", "face key %q", k)
		return out
	}
	lc.face = c.Faces[i]
	out.printed = lc.face.Name
	out.FaceMode, out.Face = faceLayout(c, token), i
	out.Faces = faceNames(name, c, out.FaceMode)
	out.RoomSplit = roomAllLines
	lc.lowerFace()
	// a face's own tokens reach the matchup through its front's token
	// list (lower_faces.go faceTokens)
	return out
}

// Casting a face: the Adventure and split
// layouts, whose second face is a spell cast on its own (gorge's
// "adventure_alt" and "split_alt", dirac/face_castface.go).

// castableFaceLayout is faceLayout for the layouts that cast a face; FaceNone refuses.
//
//   - Adventure (CR 715): face 1 must be an Instant or Sorcery of type
//     Adventure (gorge rules adventure.go adventureSpellFace); gorge offers
//     nothing for any other shape.
//   - Split (CR 709): neither half a Room (gorge's face_rooms.go owns those)
//     and no Fuse (gorge's "fuse" cast, CR 702.102, is not modelled). A
//     K:Aftermath right half (CR 702.127a) is cast only from the graveyard
//     (gorge's "aftermath", dirac/face_aftermath.go); Aftermath on the left
//     half is no shape gorge's aftermathAlternateFace reads, so it refuses.
//   - Omen (CR 720): face 1 must be an Instant or Sorcery of type Omen
//     (gorge rules adventure.go omenSpellFace) and the front must not be a
//     land (gorge's hand walk offers the Omen face from the land branch
//     too, which the kernel does not test). The card is cast as gorge's
//     "omen_alt" (dirac/face_omen.go).
func castableFaceLayout(c *cards.Card) uint8 {
	f0, f1 := c.Faces[0], c.Faces[1]
	switch c.AlternateMode {
	case "Adventure":
		// A land front (Final Fantasy's towns: Midgar, City of Mako) is
		// accepted too its Adventure is cast from
		// the hand (CR 715.3a, rules legal_walk_hand.go appendAdventureAlt
		// inside the land branch); the resolved card rests in exile with
		// no play from there (gorge offers none: legal_walk_alt.go refuses
		// a land main face as an adventure_recast, CR 715.3b open).
		if f1.IsInstant() || f1.IsSorcery() {
			for _, t := range f1.Types {
				if t == "Adventure" {
					return kop.FaceAdventure
				}
			}
		}
	case "Prepare":
		// CR 722: face 1 is the prepare spell, never cast from the hand
		// (gorge legal.go); its only route is the exile copy a prepared
		// permanent holds. Anything but an instant or sorcery refuses.
		if f1.IsInstant() || f1.IsSorcery() {
			return kop.FacePrepare
		}
	case "Omen":
		// CR 720: face 1 is the Omen spell half. gorge's omenSpellFace
		// accepts exactly this shape; a land front is refused because the
		// kernel's hand walk offers Omen faces only from the nonland branch.
		if f1.IsInstant() || f1.IsSorcery() {
			for _, t := range f1.Types {
				if t == "Omen" && !f0.IsLand() {
					return kop.FaceOmen
				}
			}
		}
	case cards.AlternateSplit:
		if roomFace(f0) && roomFace(f1) && !f0.HasKeyword("Fuse") && !f1.HasKeyword("Fuse") &&
			!f0.HasKeyword("Aftermath") && !f1.HasKeyword("Aftermath") {
			return kop.FaceRoom // lower_rooms.go
		}
		if f0.IsRoom() || f1.IsRoom() || f0.HasKeyword("Fuse") || f1.HasKeyword("Fuse") ||
			f0.HasKeyword("Aftermath") {
			return kop.FaceNone
		}
		return kop.FaceSplit
	}
	return kop.FaceNone
}

// faceTokens adds the tokens every other face of the card being lowered
// creates to its own token list: a face entry rides the front's token
// list and is not walked again, so a matchup that holds the card holds
// its faces' tokens through the front (refused such a face).
func (lc *lowerCtx) faceTokens() {
	out := lc.out
	have := map[string]bool{}
	for _, t := range out.Tokens {
		have[t] = true
	}
	for _, k := range out.Faces[1:] {
		if _, _, ok := parseFaceKey(k); !ok {
			continue // a Room state def: its doors' tokens are the faces' (lower_rooms.go)
		}
		for _, t := range lowerFaceEntry(lc.p, k).Tokens {
			if !have[t] {
				have[t] = true
				out.Tokens = append(out.Tokens, t)
			}
		}
	}
}
