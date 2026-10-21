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
