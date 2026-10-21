package cards

// Runtime copy-basis setters (CR 707.2/707.9b). events.Apply synthesises a
// copy's characteristics -- a face-down permanent's copiable values, a copy
// exception's final values -- as a value copy of an existing Face whose
// printed fields it then edits. Most derived fields self-heal (their caches
// are guarded by the identity of the slice they were derived over, so an
// edited slice reads the slow path), but the parsed P/T is read unguarded,
// so a P/T edit goes through SetPT. Neither setter runs derive(): derive
// records slice identities as uintptrs, which would make a fold-built face
// differ between a live game and its log-only replay under
// reflect.DeepEqual / FaceEqual.

// SetPT overwrites PT and re-parses Power(), Toughness() and
// CharacteristicDefining() from it, touching no other field.
func (f *Face) SetPT(pt string) {
	f.PT = pt
	f.power, f.toughness, f.characteristicDefining = parsePT(pt)
}

// SetManaCost overwrites ManaCost and re-derives Cmc()/PrintedManaValue()
// from it (CR 707.9b "it has no mana cost" is SetManaCost("")), touching no
// other field.
func (f *Face) SetManaCost(mc string) {
	f.ManaCost = mc
	f.cmc = cmcFromManaCost(mc)
	f.cmcSrc, f.cmcBound = mc, true
}

// copyColoursSet marks Face.copyColours as present; bits 0-4 are the
// ColourWhite..ColourGreen set.
const copyColoursSet uint8 = 1 << 7

// SetCopyColours records a copy exception's final colour set (CR 707.9b;
// ColourWhite..ColourGreen bits, 0 = colourless). effects.ColorMaskOf
// returns it ahead of every other colour source of the face -- mana cost,
// colour indicator, Devoid and a colour-setting characteristic-defining
// ability (CR 707.9d).
func (f *Face) SetCopyColours(mask uint8) { f.copyColours = copyColoursSet | mask&0x1f }

// CopyColours reports the colour set SetCopyColours recorded, if any.
func (f *Face) CopyColours() (uint8, bool) {
	return f.copyColours &^ copyColoursSet, f.copyColours&copyColoursSet != 0
}
