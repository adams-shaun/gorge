package effects

import "strings"

// MayPlayAdventureSpec reads the Affected$ spec of a may-play grant for the
// Adventure demand: Mosswood Dreadknight's and Hildibrand Manderville's
// "cast it from your graveyard as an Adventure" static names
// `Card.IsRemembered+Adventure`. In that position the word is not the type
// test a spell on the stack answers (the graveyard card shows its main face,
// CR 715.4, so a type test never matches) but the Adventure-card predicate
// (wordAdventureCard: the card HAS an Adventure half). The returned spec has
// every bare Adventure token rewritten to AdventureCard; adventure reports
// that every comma alternative carried one, i.e. the grant lets the card be
// cast ONLY as the Adventure (CR 715.3a), never as its main face.
//
// The SpellCast ValidCard$ uses of the same word (Lucky Clover, Lozhan,
// Gorion, ...) read a spell on the stack and keep the type test: this rewrite
// is applied only to a may-play grant's Affected$.
func MayPlayAdventureSpec(spec string) (rewritten string, adventure bool) {
	if !strings.Contains(spec, "Adventure") {
		return spec, false
	}
	var b strings.Builder
	adventure = true
	for _, alt := range strings.Split(spec, ",") {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		found := false
		start := 0
		for i := 0; i <= len(alt); i++ {
			if i < len(alt) && alt[i] != '.' && alt[i] != '+' {
				continue
			}
			tok := alt[start:i]
			if tok == "Adventure" {
				tok = "AdventureCard"
				found = true
			}
			b.WriteString(tok)
			if i < len(alt) {
				b.WriteByte(alt[i])
			}
			start = i + 1
		}
		adventure = adventure && found
	}
	return b.String(), adventure
}
