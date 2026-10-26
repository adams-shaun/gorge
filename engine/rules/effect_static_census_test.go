package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// This file is the census behind AGENTS.md's Known-approximations row for the
// Effect-delivered StaticAbilities$/Triggers$ registration (engine/effects/effect.go).
// It does NOT keep a hand list of supported modes: the registered static modes
// are read straight out of the switch in engine/effects/effect.go (its `case
// cards.Static...` labels) plus the one mode effectUnsupportedStatic admits
// (IgnoreHexproof), and the registered trigger modes are read from the live
// trigger registry (trigmatch.Lookup), which is the same table
// Engine.TriggerModeSupported consults. The corpus walk then reports every
// mode reached through an Effect in the pinned corpus and whether the code
// registers it or emits the unimplemented Note.

var staticCaseRE = regexp.MustCompile(`cards\.Static([A-Z][A-Za-z]+)`)

// effectRegisteredStaticModes parses the Effect switch's own case labels.
func effectRegisteredStaticModes(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "effects", "effect.go"))
	if err != nil {
		t.Fatalf("read engine/effects/effect.go: %v", err)
	}
	modes := map[string]bool{}
	for _, m := range staticCaseRE.FindAllStringSubmatch(string(src), -1) {
		if m[1] == "ModeOf" { // cards.StaticModeOf, not a mode label
			continue
		}
		modes[m[1]] = true
	}
	// IgnoreHexproof is not a switch label: effectUnsupportedStatic registers
	// it from the default arm. Detect that read rather than assume it.
	ih, err := os.ReadFile(filepath.Join("..", "effects", "effect_ignorehexproof.go"))
	if err != nil {
		t.Fatalf("read engine/effects/effect_ignorehexproof.go: %v", err)
	}
	if strings.Contains(string(ih), "mode == ModeIgnoreHexproof") {
		modes["IgnoreHexproof"] = true
	}
	return modes
}

// splitEffectTokens splits a comma/whitespace separated token list.
func splitEffectTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}

// effectAbilityField returns the tokens a `Field$` parameter carries on one
// ability line, or nil when the line has no such parameter. Ability
// parameters are `|`-separated, so a whole-field match cannot confuse
// `Triggers$` with `AddTriggers$`.
func effectAbilityField(line, field string) []string {
	for _, part := range strings.Split(line, "|") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, field+"$") {
			return splitEffectTokens(strings.TrimPrefix(part, field+"$"))
		}
	}
	return nil
}

// effectCorpusModes returns, per mode, the set of corpus files whose EFFECT
// ability (an `A:AB$`/`A:SP$` or `SVar:NAME:DB$/AB$/SP$` whose API is
// `Effect`) carries a `StaticAbilities$`/`Triggers$` reference to an SVar
// whose `Mode$` names that mode. Only Effect-delivered grants are counted: an
// Animate, a Clone `AddTriggers$` or a printed `T:Mode$` trigger is a
// different registration path and is not what the AGENTS.md row describes.
func effectCorpusModes(t *testing.T) (static, trigger map[string]map[string]bool) {
	t.Helper()
	root := filepath.Join(testutil.CorpusDirOrSkip(t), "cardsfolder")
	static = map[string]map[string]bool{}
	trigger = map[string]map[string]bool{}
	abilityRE := regexp.MustCompile(`(?m)^(?:SVar:[^:\n]+:)?(?:A:)?(?:DB|AB|SP)\$([^\n]*)`)
	modeRE := regexp.MustCompile(`(?m)^SVar:([^:\n]+):Mode\$([^\n|]*)`)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		txt := string(b)
		modeBySVar := map[string][]string{}
		for _, m := range modeRE.FindAllStringSubmatch(txt, -1) {
			modeBySVar[strings.TrimSpace(m[1])] = splitEffectTokens(m[2])
		}
		add := func(tbl map[string]map[string]bool, refs []string) {
			for _, ref := range refs {
				for _, mode := range modeBySVar[ref] {
					if tbl[mode] == nil {
						tbl[mode] = map[string]bool{}
					}
					tbl[mode][path] = true
				}
			}
		}
		for _, m := range abilityRE.FindAllStringSubmatch(txt, -1) {
			if !strings.HasPrefix(strings.TrimSpace(m[1]), "Effect") {
				continue
			}
			add(static, effectAbilityField(m[1], "StaticAbilities"))
			add(trigger, effectAbilityField(m[1], "Triggers"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return static, trigger
}

// effectRowStillNotes parses the Known-approximations row file for the
// Effect-delivered StaticAbilities$/Triggers$ registration and returns the
// static and trigger modes the row names as still unimplemented. Reading the
// row (rather than a second hand list) is what makes the census a ratchet on
// the row itself: a registration change fails the census, and the only way to
// pass is to change the row.
func effectRowStillNotes(t *testing.T) (statics, triggers []string) {
	t.Helper()
	const rowFile = "docs/agents/known-approximations/effect-modes.md"
	b, err := os.ReadFile(filepath.Join("..", "..", rowFile))
	if err != nil {
		t.Fatalf("read %s: %v", rowFile, err)
	}
	var cell string
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "**Stand-in.** "); ok {
			cell = rest
			break
		}
	}
	if cell == "" {
		t.Fatalf("%s: no **Stand-in.** paragraph -- did the row move or get deleted?", rowFile)
	}
	backtick := regexp.MustCompile("`([A-Za-z][A-Za-z0-9]*)`")
	section := func(start, end string) []string {
		i := strings.Index(cell, start)
		if i < 0 {
			t.Fatalf("effect-modes row has no %q section", strings.TrimSpace(start))
		}
		rest := cell[i+len(start):]
		if end != "" {
			j := strings.Index(rest, end)
			if j < 0 {
				t.Fatalf("effect-modes row %q section has no %q terminator", strings.TrimSpace(start), strings.TrimSpace(end))
			}
			rest = rest[:j]
		}
		var out []string
		for _, m := range backtick.FindAllStringSubmatch(rest, -1) {
			out = append(out, m[1])
		}
		return out
	}
	return section("statics ", "; triggers "), section("triggers ", "")
}

// TestEffectStaticCensus is a report: it logs every mode reached through an
// Effect in the corpus and whether the code registers it, and pins the modes
// that still Note to AGENTS.md's row for the same registration, so a mode
// silently gaining or losing its registration fails here and the row cannot
// drift from the code.
func TestEffectStaticCensus(t *testing.T) {
	staticHandled := effectRegisteredStaticModes(t)
	staticCorpus, triggerCorpus := effectCorpusModes(t)
	rowStatic, rowTrigger := effectRowStillNotes(t)

	names := func(m map[string]map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	var staticNotes, triggerNotes []string
	var b strings.Builder
	fmt.Fprintf(&b, "Effect static modes reached in the corpus (%d):\n", len(staticCorpus))
	for _, mode := range names(staticCorpus) {
		state := "REGISTERED"
		if !staticHandled[mode] {
			state = "NOTES"
			staticNotes = append(staticNotes, mode)
		}
		fmt.Fprintf(&b, "  %-4s %-28s %d files\n", state, mode, len(staticCorpus[mode]))
	}
	fmt.Fprintf(&b, "Effect trigger modes reached in the corpus (%d):\n", len(triggerCorpus))
	for _, mode := range names(triggerCorpus) {
		state := "REGISTERED"
		if trigmatch.Lookup(mode) == nil && mode != "ChangesController" {
			state = "NOTES"
			triggerNotes = append(triggerNotes, mode)
		}
		fmt.Fprintf(&b, "  %-4s %-28s %d files\n", state, mode, len(triggerCorpus[mode]))
	}
	t.Logf("\n%s", b.String())

	// Compare as sets (sorted), so the row's prose order is not load-bearing
	// while a mode silently added or removed still fails.
	set := func(s []string) string {
		c := append([]string(nil), s...)
		sort.Strings(c)
		return strings.Join(c, ",")
	}
	if got, want := set(staticNotes), set(rowStatic); got != want {
		t.Errorf("Effect static modes that still Note differ from docs/agents/known-approximations/effect-modes.md:\n census: %s\n    row: %s", got, want)
	}
	if got, want := set(triggerNotes), set(rowTrigger); got != want {
		t.Errorf("Effect trigger modes that still Note differ from docs/agents/known-approximations/effect-modes.md:\n census: %s\n    row: %s", got, want)
	}
}
