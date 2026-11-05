package draftlab

import (
	"fmt"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// KnobsPrefix marks an agent ref that names a pickbot knob file:
// "knobs:<path>" resolves to pickbot.NewTuned under the file's Tuning
// (pickbot.ParseTuning), and the resolved ref is "knobs@<hash>"
// (Tuning.Hash), so a manifest records the knobs' content, not the path.
// An explicit version ("knobs:<path>@<hash>") must equal the file's hash.
const KnobsPrefix = "knobs:"

// knobsName is the name a resolved knob-file ref carries.
const knobsName = "knobs"

// resolveKnobs builds the pair a knobs:<path> ref names, or reports ok
// false when ref is not one.
func (l *Lab) resolveKnobs(role Role, ref AgentRef) (pickbot.Drafter, pickbot.Builder, AgentRef, bool, error) {
	path, ok := strings.CutPrefix(ref.Name, KnobsPrefix)
	if !ok {
		return nil, nil, AgentRef{}, false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, AgentRef{}, true, fmt.Errorf("draftlab: %s %q: %w", role, ref.Name, err)
	}
	t, err := pickbot.ParseTuning(b)
	if err != nil {
		return nil, nil, AgentRef{}, true, fmt.Errorf("draftlab: %s %q: %w", role, ref.Name, err)
	}
	hash := t.Hash()
	if ref.Version != "" && ref.Version != hash {
		return nil, nil, AgentRef{}, true, fmt.Errorf("%w: %s %q hashes to %q, not %q", ErrVersion, role, ref.Name, hash, ref.Version)
	}
	d, bld := pickbot.NewTuned(t, l.lookup())
	return d, bld, AgentRef{Name: knobsName, Version: hash}, true, nil
}
