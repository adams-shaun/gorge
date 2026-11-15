package decision

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// HideName replaces name with shown in d's prompt and option labels, because the
// seat asked can't see object hidden, which holds that name. Text about a card
// holding the same name in the open keeps it: open says whether an id is one, and
// a label about that card reads wrong otherwise, unless the option names hidden
// as what it attacks. An option that names no object at all carries a
// label-valued choice (a ChooseType's type word, a name pick): its label is a
// vocabulary word, never the face-down card's identity, so hiding it would
// narrow the ask's legal answers (and, on a fixed vocabulary, leak that a
// face-down card of that name exists).
func (d *Decision) HideName(hidden state.ObjID, name, shown string, open func(state.ObjID) bool) {
	if !open(d.Source) {
		d.Prompt = strings.ReplaceAll(d.Prompt, name, shown)
	}
	for i := range d.Options {
		if o := &d.Options[i]; o.Attacker == hidden || (o.Obj != 0 && !open(o.Obj)) {
			o.Label = strings.ReplaceAll(o.Label, name, shown)
		}
	}
}
