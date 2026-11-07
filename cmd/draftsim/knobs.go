package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// knobsPrefix marks a -drafter or -builder value that names a pickbot knob
// file, "knobs:<path>" or "knobs:<path>@<hash>". It is the same literal as
// draftlab.KnobsPrefix (draft/draftlab/knobs.go); draftsim does not import
// draftlab for one constant.
const knobsPrefix = "knobs:"

// knobFile reads the knob file one flag value names. ok is false when spec
// is not a knob-file value (a registry name). The file is parsed with
// pickbot.ParseTuning, so an unknown key or an invalid knob set is refused
// naming the key; an explicit @<hash> must equal Tuning.Hash. No corpus is
// needed, so the refusal comes before the corpus opens.
func knobFile(spec, flag string) (t pickbot.Tuning, ok bool, err error) {
	rest, ok := strings.CutPrefix(spec, knobsPrefix)
	if !ok {
		return pickbot.Tuning{}, false, nil
	}
	path, want := rest, ""
	// The @<hash> suffix is the text after the last '@' when it holds no
	// path separator, so a directory carrying an '@' stays a path.
	if i := strings.LastIndexByte(rest, '@'); i >= 0 && !strings.ContainsRune(rest[i+1:], '/') {
		path, want = rest[:i], rest[i+1:]
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return pickbot.Tuning{}, true, fmt.Errorf("-%s %q: %w", flag, spec, err)
	}
	t, err = pickbot.ParseTuning(b)
	if err != nil {
		return pickbot.Tuning{}, true, fmt.Errorf("-%s %q: knob file %s: %w", flag, spec, path, err)
	}
	if h := t.Hash(); want != "" && want != h {
		return pickbot.Tuning{}, true, fmt.Errorf("-%s %q: knob file %s hashes to %q, not %q", flag, spec, path, h, want)
	}
	return t, true, nil
}

// personaLabel validates one -drafter/-builder value without the corpus: a
// knob file is parsed (its label is knobs@<hash>), a registry name is
// looked up (its label is empty).
func personaLabel(spec, flag string) (string, error) {
	t, ok, err := knobFile(spec, flag)
	if err != nil {
		return "", err
	}
	if ok {
		return "knobs@" + t.Hash(), nil
	}
	if _, _, ok := pickbot.Lookup(spec); !ok {
		return "", fmt.Errorf("unknown -%s %q (known: %s)", flag, spec, pickbot.Names())
	}
	return "", nil
}

// persona builds the drafter/builder pair one flag value names: the
// registry pair for a persona name, pickbot.NewTuned under the file's
// Tuning for a knob file. The label is personaLabel's.
func persona(spec, flag string, lookup pickbot.CardLookup) (pickbot.Drafter, pickbot.Builder, string, error) {
	t, ok, err := knobFile(spec, flag)
	if err != nil {
		return nil, nil, "", err
	}
	if ok {
		d, b := pickbot.NewTuned(t, lookup)
		return d, b, "knobs@" + t.Hash(), nil
	}
	d, b, ok := pickbot.New(spec, lookup)
	if !ok {
		return nil, nil, "", fmt.Errorf("unknown -%s %q (known: %s)", flag, spec, pickbot.Names())
	}
	return d, b, "", nil
}

// personaHeader is the event header's ", drafter X, builder Y" part: the
// flag values as given, each knob file followed by its knobs@<hash>.
func personaHeader(drafter, dLabel, builder, bLabel string) string {
	part := func(v, label string) string {
		if label == "" {
			return v
		}
		return v + " " + label
	}
	return fmt.Sprintf("drafter %s, builder %s", part(drafter, dLabel), part(builder, bLabel))
}
