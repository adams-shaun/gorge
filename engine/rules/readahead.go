package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// kw:Read ahead (CR 702.155): "As this Saga enters, choose a number between
// one and this Saga's final chapter number. This Saga enters with the chosen
// number of lore counters on it." (702.155b / 714.3b), and "chapter abilities
// of this Saga can't trigger the turn it entered the battlefield unless it
// has exactly the number of lore counters on it specified in the chapter
// symbol of that ability" (702.155a).
//
// The choice half is the Unleash precedent: faceETBChoice poses a KChoose at
// the entry boundary, resumeETBEntry logs it as a Choose "readahead" event
// (Amount = the chosen chapter), the Choose fold records it on the object and
// events.EntryCounterGrantsForFace turns it into the entry's LORE grant. The
// trigger half is checkChapterTriggers' entry-turn filter (engine/rules/saga.go).

// readAheadOptions are the final-chapter-many answers of the as-enters read
// ahead choice, ascending: option i enters with chapter i+1's lore count.
var readAheadLabels = [...]string{"Chapter I", "Chapter II", "Chapter III", "Chapter IV", "Chapter V", "Chapter VI", "Chapter VII", "Chapter VIII"}

func readAheadOptions(id state.ObjID, p state.PlayerID, n int) []decision.Option {
	out := make([]decision.Option, 0, n)
	for i := 1; i <= n; i++ {
		label := "Chapter"
		if i <= len(readAheadLabels) {
			label = readAheadLabels[i-1]
		}
		out = append(out, decision.Option{Index: len(out), Kind: "readahead",
			Label: label, Amount: i, Obj: id, Player: p})
	}
	return out
}

func init() {
	effects.RegisterNonAPI("kw:Read ahead")
}
