package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEffectReplDeathRepLowers: the Effect's dies-to-exile replacement (Cry of
// the Carnarium, Flaying Tendrils, Malicious Malfunction: "If a creature would
// die this turn, exile it instead") lowers as one until-end-of-turn emblem of
// the resolving controller carrying one ReplExileOnLeave with the line's
// filter, plus the CreateEmblem instruction. Each compared value differs: the
// cards' other shapes are the parent spell's instructions, and the emblem is
// the only one that exiles.
func TestEffectReplDeathRepLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lowerNamed := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	for _, name := range []string{"Cry of the Carnarium", "Flaying Tendrils", "Malicious Malfunction"} {
		c := lowerNamed(name)
		if got := keclReason(c.Refusals); got != "" {
			t.Errorf("%s: refusals %q, want none", name, got)
		}
		if len(c.Emblems) != 1 {
			t.Fatalf("%s: %d emblems, want 1", name, len(c.Emblems))
		}
		e := c.Emblems[0]
		if !e.UntilEOT {
			t.Errorf("%s: the emblem is not until end of turn", name)
		}
		if len(e.Repls) != 1 || e.Repls[0].Kind != "ExileOnLeave" || e.Repls[0].F == nil {
			t.Fatalf("%s: emblem repls %+v, want one ExileOnLeave with a filter", name, e.Repls)
		}
		if e.Repls[0].Cond.Kind != cNone || e.Repls[0].AnyZone {
			t.Errorf("%s: the emblem's replacement is conditioned or any-zone: %+v", name, e.Repls[0])
		}
		var at int
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "CreateEmblem" {
					at++
					if in.C != 0 {
						t.Errorf("%s: CreateEmblem C %d, want the emblem's index 0", name, in.C)
					}
				}
			}
		}
		if at == 0 {
			t.Errorf("%s: no CreateEmblem instruction", name)
		}
	}

	// Out-of-shape variants keep today's refusal by name: any extra key on the
	// Effect or the line, a Duration$, and a from-anywhere origin are the
	// census classes this slice does not touch.
	const base = "Name:Death Rep Probe\nManaCost:1 B B\nTypes:Sorcery\n" +
		"A:SP$ PumpAll | ValidCards$ Creature | NumAtt$ -2 | NumDef$ -2 | SubAbility$ DBEffect | SpellDescription$ pump\n" +
		"SVar:DBEffect:DB$ Effect | ReplacementEffects$ DeathRep | SpellDescription$ effect\n" +
		"SVar:DeathRep:Event$ Moved | ValidLKI$ Creature | Origin$ Battlefield | Destination$ Graveyard | ReplaceWith$ Exile | Description$ probe\n" +
		"SVar:Exile:DB$ ChangeZone | Defined$ ReplacedCard | Destination$ Exile | Origin$ Battlefield\n"
	if r := deathRepSynth(t, base); len(r) != 0 {
		t.Fatalf("the probe script itself refuses: %q", keclReason(r))
	}
	for _, c := range []struct{ name, from, to, want string }{
		{"Duration", "ReplacementEffects$ DeathRep |", "ReplacementEffects$ DeathRep | Duration$ Permanent |", "Effect replacement Duration$"},
		{"from anywhere", "Origin$ Battlefield | Destination$ Graveyard", "Destination$ Graveyard", "Effect replacement \"DeathRep\""},
		{"both filters", "ValidLKI$ Creature", "ValidLKI$ Creature | ValidCard$ Creature", "Effect replacement \"DeathRep\""},
		{"extra line key", "Description$ probe", "Description$ probe | ReplacementResult$ None", "Effect replacement \"DeathRep\""},
		{"counter body", "Origin$ Battlefield\n", "Origin$ Battlefield | WithCountersType$ VOID\n", "Effect replacement \"DeathRep\""},
		{"rider body", "Destination$ Exile | Origin$ Battlefield\n", "Destination$ Exile | Origin$ Battlefield | SubAbility$ ExSub\nSVar:ExSub:DB$ ChangeZone | Defined$ ReplacedCard | Destination$ Exile | Tapped$ True\n", "Effect replacement \"DeathRep\""},
		{"uncompilable filter", "ValidLKI$ Creature", "ValidLKI$ Creature+FooBar", "Effect replacement \"DeathRep\""},
	} {
		src := strings.Replace(base, c.from, c.to, 1)
		if src == base {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(deathRepSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}

// deathRepSynth lowers one synthetic script the way exileBfSynth does.
func deathRepSynth(t *testing.T, src string) []refusal {
	t.Helper()
	card, diags := cards.ParseBytes("deathrep_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Death Rep Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false).Refusals
}
