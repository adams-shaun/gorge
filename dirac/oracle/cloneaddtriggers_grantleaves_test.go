package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// A copy's inherited AddTriggers$ trigger outlives its granting card: the
// fold put the trigger line into the copy's copiable values (CR 707.9b), so
// the trigger is on the copy's face whatever became of the card it was
// copied from. Reachable: Copycrook enters as a copy of a Phantasmal Image
// copy; the Image is sacrificed by its own granted trigger; the Copycrook
// copy then becomes the target of a spell. The projection must still mark
// the stack ability as the copy's granted (latent) trigger, so it must
// decide the trigger's provenance from the matchup's cards, never from the
// battlefield.
func TestCloneAddTriggerGrantLeaves(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	crook, ok := reg.Lookup("Copycrook")
	if !ok {
		t.Fatal("Copycrook not in the corpus")
	}
	image, ok := reg.Lookup("Phantasmal Image")
	if !ok {
		t.Fatal("Phantasmal Image not in the corpus")
	}
	visionary, ok := reg.Lookup("Elvish Visionary")
	if !ok {
		t.Fatal("Elvish Visionary not in the corpus")
	}

	// The trigger line the Image's Clone AddTriggers$ names.
	trig, ok := cloneAddTriggersLine(image.Faces[0])
	if !ok {
		t.Fatal("Phantasmal Image's face has no AddTriggers$ trigger line")
	}
	fold := *crook.Faces[0]
	fold.Triggers = append([]cards.Trigger(nil), fold.Triggers...)
	fold.Triggers = append(fold.Triggers, trig)
	copyOfImage := &state.Object{Card: crook, CopyFace: &fold}
	i := len(fold.Triggers) - 1
	if i < 0 {
		t.Fatal("no folded trigger on the face")
	}
	// The matchup's cards, the granting Image nowhere on any battlefield:
	// the provenance must come from them alone.
	m := &Matchup{reg: reg, Table: []string{"Copycrook", "Elvish Visionary", "Grizzly Bears", "Phantasmal Image"}}
	if !copyAddTriggerOf(m, copyOfImage, i) {
		t.Fatalf("the Image's BecomesTarget trigger folded into a Copycrook copy's face is not recognised with the granting Image off the battlefield (trigger %d, params %v)", i, trig.Params)
	}

	// A trigger that is no AddTriggers$ line of any card of the matchup is
	// the copy's printed card's, or a folded printed trigger: never granted.
	visFold := *crook.Faces[0]
	visFold.Triggers = append([]cards.Trigger(nil), visFold.Triggers...)
	visFold.Triggers = append(visFold.Triggers, visionary.Faces[0].Triggers[0])
	copyOfVisionary := &state.Object{Card: crook, CopyFace: &visFold}
	if copyAddTriggerOf(m, copyOfVisionary, len(visFold.Triggers)-1) {
		t.Fatal("Elvish Visionary's printed ETB trigger on a Copycrook copy's face is marked as an AddTriggers$ grant")
	}

	// The printed face itself is never a granted trigger.
	printed := &state.Object{Card: crook}
	if copyAddTriggerOf(m, printed, 0) {
		t.Fatal("Copycrook's printed face is treated as a copy face")
	}
}

// cloneAddTriggersLine returns one trigger the face's Clone AddTriggers$
// names, the line gorge folds into a copy's face (CR 707.9b).
func cloneAddTriggersLine(f *cards.Face) (cards.Trigger, bool) {
	for _, raw := range f.SVars {
		if !strings.Contains(raw, "AddTriggers$") {
			continue
		}
		for _, n := range strings.Split(parseCloneParam(raw, "AddTriggers"), ",") {
			if tr, ok := cards.ParseTriggerLine(f.SVars[strings.TrimSpace(n)]); ok {
				return tr, true
			}
		}
	}
	return cards.Trigger{}, false
}
