package gorgebe

// respelled maps an mtgish type word to gorge's spelling. A word not listed is
// printed unchanged. Forge stores multiword types as separate tokens.
var respelled = map[string]string{
	"AssemblyWorker": "Assembly-Worker",
	"Ctan":           "C'tan",
	"MindlessOne":    "Mindless-One",
	"Shiar":          "Shi'ar",
	"PowerPlant":     "Power-Plant",
	"Urzas":          "Urza's",
	"TimeLord":       "Time Lord",
}

// Respell returns the gorge spelling of an mtgish type word.
func Respell(w string) string {
	if s, ok := respelled[w]; ok {
		return s
	}
	return w
}
