package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerStep(ir.VerbEmblem, printEmblem) }

// printEmblem prints "you get an emblem with ...": an Effect$ named for the
// source whose statics and triggers live in the command zone for good. Each
// emblem ability is printed by the ability printers into a scratch printer
// and its line kept as an SVar the Effect names.
func printEmblem(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Optional || st.Who != nil || len(st.Abilities) == 0 || p.face == nil || p.face.Name == "" {
		return nil, 0, unprintable("verb:emblem", st.Src)
	}
	n := &node{API: "Effect", Params: []kv{
		{"Name", "Emblem — " + p.face.Name},
		{"Image", "emblem_" + strings.Trim(Stem(p.face.Name), "_")},
		{"Duration", "Permanent"},
	}}
	var statics, trigs []string
	for _, a := range st.Abilities {
		fn := abilityFns[a.Kind]
		if fn == nil || (a.Kind != ir.AbStatic && a.Kind != ir.AbTriggered) {
			return nil, 0, unprintable("emblem:"+a.Kind.String(), a.Src)
		}
		sub := &pr{tokens: p.tokens, face: p.face, svars: p.svars}
		if err := fn(sub, a); err != nil {
			return nil, 0, err
		}
		p.svars = sub.svars
		if len(sub.lines) != 1 {
			return nil, 0, unprintable("emblem:lines", a.Src)
		}
		line := sub.lines[0]
		switch {
		case a.Kind == ir.AbStatic && strings.HasPrefix(line, "S:"):
			line = strings.TrimPrefix(line, "S:")
			if !strings.Contains(line, "AffectedZone$") && strings.Contains(line, "Affected$") {
				line += " | AffectedZone$ Battlefield"
			}
			statics = append(statics, p.addSVar("EmblemStatic", line))
		case a.Kind == ir.AbTriggered && strings.HasPrefix(line, "T:"):
			line = strings.TrimPrefix(line, "T:")
			line = strings.ReplaceAll(line, "TriggerZones$ Battlefield", "TriggerZones$ Command")
			if !strings.Contains(line, "TriggerZones$") {
				line += " | TriggerZones$ Command"
			}
			trigs = append(trigs, p.addSVar("EmblemTrig", line))
		default:
			return nil, 0, unprintable("emblem:"+a.Kind.String(), a.Src)
		}
	}
	if len(statics) > 0 {
		n.set("StaticAbilities", strings.Join(statics, ","))
	}
	if len(trigs) > 0 {
		n.set("Triggers", strings.Join(trigs, ","))
	}
	return n, 1, nil
}

// addSVar adds body as a new SVar named for base and returns the name.
func (p *pr) addSVar(base, body string) string {
	name := p.fresh(base)
	p.svars = append(p.svars, kv{name, body})
	return name
}
