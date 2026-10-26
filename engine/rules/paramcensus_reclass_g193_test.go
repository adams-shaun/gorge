package rules

import (
	"sort"
	"testing"
)

// cr-g193: census-only reclassification. Seven stack-description,
// dialog-title and AI-hint keys are in ignoredParamKeys because every Forge
// reader is text or AI only (see the entries' cites). The ignore is
// key-global, so it is only sound while no gorge primitive reads them.

// TestIgnoredDescAndAIHintKeysAreUnreadInGorge fails when the census scan
// records a read of any of the seven keys, in any bucket, by any function in
// engine/rules or engine/effects. A real reader must drop the key from
// ignoredParamKeys instead of hiding behind it.
func TestIgnoredDescAndAIHintKeysAreUnreadInGorge(t *testing.T) {
	t.Parallel()
	keys := []string{"ChoiceDesc", "FromDesc", "ChoiceTitleAppend", "ChangeNumDesc", "AIMinTgts", "AITgtOwnCards", "AIXMax"}
	for _, k := range keys {
		if ignoredParamKeys[k] == "" {
			t.Errorf("%s is not in ignoredParamKeys -- the reclassification is gone", k)
		}
	}
	s := scanPackages(t)
	// Precondition: the scan sees reads at all, and sees a known ignored
	// key's absence the same way (so an empty result is not a blind scan).
	total := 0
	for _, fi := range s.fns {
		for _, set := range fi.reads {
			total += len(set)
		}
	}
	if total == 0 {
		t.Fatal("scan recorded no reads -- the absence check below is vacuous")
	}
	var found []string
	for name, fi := range s.fns {
		for b, set := range fi.reads {
			for _, k := range keys {
				if set[k] {
					found = append(found, name+" reads "+k+" (bucket "+b.String()+")")
				}
			}
		}
		for b, set := range fi.keyRds {
			for _, k := range keys {
				if set[k] {
					found = append(found, name+" key-reads "+k+" (bucket "+b.String()+")")
				}
			}
		}
		for param, set := range fi.mapIdxRds {
			for _, k := range keys {
				if set[k] {
					found = append(found, name+" indexes "+param+"["+k+"]")
				}
			}
		}
	}
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("gorge reads an ignored key: %s -- remove it from ignoredParamKeys and classify it", f)
	}
}
