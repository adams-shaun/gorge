package oracle

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/canon"
)

// ActionLabel names the vocabulary word a recorded pick chose: the creature
// type, land type, card name or colour behind Repro.Actions[At]. The canonical
// int of such a pick is the word's index in a vocabulary that grows (#4601
// shifted the creature-type list and silently detached the Herald's Horn
// repro's pick from its subject), so the repro carries the word beside it and
// ScriptPolicy checks the two agree.
type ActionLabel struct {
	At    int    `json:"at"`
	Label string `json:"label"`
}

// labelledKinds are the choose kinds a pick's label is recorded for.
var labelledKinds = map[string]bool{"type": true, "creature_type": true, "land_type": true, "name": true, "color": true}

// chooseKindOf is the choose kind of canonical action a, "" for any other
// action.
func chooseKindOf(a Canon) string {
	if a.Kind() < ActChoose {
		return ""
	}
	if i := int(a.Kind() - ActChoose); i < len(chooseKinds) {
		return chooseKinds[i]
	}
	return ""
}

// labelMatches reports whether option o (a choose of kind) names label: the
// vocabulary value the canonical form packs for it (optionAction).
func labelMatches(o Canon, kind, label string) bool {
	switch kind {
	case "type", "creature_type", "land_type":
		h := labelValue(label)
		return o.Tgt() == Ref(h>>8) && o.Mode() == uint8(h)
	case "name":
		h := canon.NameValue(label)
		return o.Tgt() == Ref(h>>8) && o.Mode() == uint8(h)
	case "color":
		c := strings.Index(manaSyms, label)
		return len(label) == 1 && c >= 0 && int(o.Mode()) == c
	}
	return false
}

// noteLabel records the label of the vocabulary pick a is, in the decision the
// side poses, before Apply answers it. The label is only kept when
// labelMatches reads it back off a, so a replay can check it.
func (g *GorgeSide) noteLabel(a Canon) {
	at := g.applyN
	g.applyN++
	fr := g.fr
	kind := chooseKindOf(a)
	if fr == nil || fr.d == nil || !labelledKinds[kind] {
		return
	}
	for i, o := range fr.cur.Options {
		if o != a || i >= len(fr.curAt) || fr.curAt[i] < 0 || fr.curAt[i] >= len(fr.d.Options) {
			continue
		}
		opt := &fr.d.Options[fr.curAt[i]]
		label := opt.Label
		if kind == "color" {
			label = opt.ManaSymbol
		}
		if label != "" && labelMatches(a, kind, label) {
			g.Labels = append(g.Labels, ActionLabel{At: at, Label: label})
		}
		return
	}
}

// labelAt is the label recorded for the action at index i.
func (s *ScriptPolicy) labelAt(i int) (string, bool) {
	for s.labelNext < len(s.Labels) && s.Labels[s.labelNext].At < i {
		s.labelNext++
	}
	if s.labelNext < len(s.Labels) && s.Labels[s.labelNext].At == i {
		return s.Labels[s.labelNext].Label, true
	}
	return "", false
}

// pickLabelled resolves the labelled action a (index i) against the offered
// options. It serves a when a is offered and names the label; false (a skip)
// when neither a nor the label is offered; and fails the replay when the label
// names a different option than a does, or a is offered and the label is not,
// both a vocabulary change under a stale int. Remap serves the label's option
// instead of failing the first.
func (s *ScriptPolicy) pickLabelled(c *PickCtx, i int, a Canon, label string) (Canon, bool) {
	kind := chooseKindOf(a)
	var want Canon
	found, offered := false, false
	for _, o := range c.D.Options {
		offered = offered || o == a
		if o.Kind() == a.Kind() && labelMatches(o, kind, label) && (!found || o == a) {
			want, found = o, true
		}
	}
	switch {
	case found && want == a:
		return a, true
	case found && s.Remap:
		return want, true
	case found:
		panic(fmt.Sprintf("replay action %d: pick %v is stale: its label %q is option %v at decision %d (kfuzz -remap rewrites the ints from the labels); offered: %s",
			i, a, label, want, c.Seq, offeredList(c.D.Options)))
	case offered:
		panic(fmt.Sprintf("replay action %d: label %q does not name pick %v at decision %d; offered: %s", i, label, a, c.Seq, offeredList(c.D.Options)))
	}
	return 0, false
}

// offeredList renders options for a replay failure, capped.
func offeredList(opts []Canon) string {
	const max = 24
	var b strings.Builder
	for i, o := range opts {
		if i == max {
			fmt.Fprintf(&b, " … %d more", len(opts)-max)
			break
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(o.String())
	}
	return "[" + b.String() + "]"
}

// skipAction counts action i (a) as skipped, or fails the replay when the
// script does not allow skips.
func (s *ScriptPolicy) skipAction(c *PickCtx, i int, a Canon, label string, labelled bool) {
	if !s.AllowSkips {
		l := ""
		if labelled {
			l = fmt.Sprintf(" (label %q)", label)
		}
		panic(fmt.Sprintf("replay action %d: %v%s is not offered at decision %d and the repro does not set allow_skips; offered: %s", i, a, l, c.Seq, offeredList(c.D.Options)))
	}
	s.Skipped++
	if s.Skipped == 1 {
		s.firstSkipAt = c.Seq
	}
}
