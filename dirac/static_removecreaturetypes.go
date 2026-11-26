package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugRmCreatSubKeep is the mutation switch of TestRmcreatsubMutation: the
// RemoveCreatureTypes strip of an override (kop.Override.RemoveCreatureSub)
// is skipped, the add-only reading.
var DebugRmCreatSubKeep bool

// ovrCreatSub reports whether override o drops the creature subtypes and the
// all-creature-types mark: its RemoveCreatureSub flag (RemoveCreatureTypes$
// without RemoveCardTypes$), or CR 205.1a's follow-through, the subtypes
// correlated with the card types its RemoveTypes removes -- a static's
// RemoveType$ Creature takes the God word with the Creature card type (the
// devotion gods' isn't-a-creature; gorge chars/types.go's RemoveTypes
// block). Every reader of the two fields goes through this one helper, as
// ovrAllCreat's readers do.
func ovrCreatSub(o *kop.Override) bool {
	return o.RemoveCreatureSub || o.RemoveTypes&kop.TCreature != 0
}

// artifactSubtypeWords is the CR 205.3g artifact-subtype vocabulary (plus the
// words non-tournament cards print beside the type): stripArtifactSubs reads
// the table's subtype names against it so a RemoveTypes$ Artifact strip takes
// the correlated subtypes with it (CR 205.1a; gorge effects.artifactTypeWords).
var artifactSubtypeWords = map[string]bool{
	"Attraction": true, "Blood": true, "Bobblehead": true, "Book": true, "Clue": true,
	"Communicator": true, "Contraption": true, "Equipment": true, "Food": true,
	"Fortification": true, "Gold": true, "Hat": true, "Heartwood": true,
	"Incubator": true, "Infinity": true, "Junk": true, "Key": true, "Lander": true,
	"Map": true, "Mutagen": true, "Powerstone": true, "Spacecraft": true,
	"Stone": true, "Treasure": true, "Vehicle": true, "Vibranium": true,
}

// ovrArtifactSub reports whether override o drops the artifact subtypes: its
// RemoveTypes names the Artifact card type (CR 205.1a's follow-through, as
// ovrCreatSub does for Creature).
func ovrArtifactSub(o *kop.Override) bool { return o.RemoveTypes&kop.TArtifact != 0 }

// stripArtifactSubs removes names' artifact-subtype words from m. The table's
// subtype vocabulary is read in place: the strip is rare (only an
// override.RemoveTypes naming Artifact), so it costs no fixed genPrep field
// and no layout pin.
func stripArtifactSubs(m kop.SubMask, names []string) kop.SubMask {
	var drop kop.SubMask
	for i, n := range names {
		if artifactSubtypeWords[n] {
			drop = drop.With(i)
		}
	}
	return m.AndNot(drop)
}
