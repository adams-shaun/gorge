package dirac

// NonCreatureSubtype reports whether w names a subtype a matchup table can
// hold as a non-creature subtype (CR 205.3g-205.3k): the words in
// nonCreatureSubtype, dirac/gen.go. kgen keeps a word outside this set and the
// creature types unknown. Cold: only kgen's filter-vocabulary reader calls it.
func NonCreatureSubtype(w string) bool {
	if DebugSubtypeVocabNone {
		return false
	}
	return nonCreatureSubtype[w]
}

// DebugSubtypeVocabNone is a test hook for the mutation:
// true makes NonCreatureSubtype answer false, so a word only the corpus
// vocabulary knows (Room, Lesson, Shrine) falls back to kgen's old refusal.
var DebugSubtypeVocabNone bool
