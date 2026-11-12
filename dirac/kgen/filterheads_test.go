package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// the six filter spellings gorge reads with a fixed
// meaning (TargetedPlayerCtrl, ControlledBy TriggeredDefendingPlayer, the
// A;B cost type list, NICKNAME, Targeted.!<Type>, the Spell head of a
// target-gated reduction) and filter-named creature types outside the pool.
// NICKNAME stays refused (TestNicknameSacSelf).

// kfhPool lowers and builds the named cards; mod, when set, edits the pool's
// cards first (a synthetic shape no corpus card carries).
type kfhPool struct {
	cards map[string]*lCard
	built map[string]built
	pt    *poolTables
}

func kfhBuild(t *testing.T, names []string, mod func(p *pool)) kfhPool {
	t.Helper()
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	if mod != nil {
		mod(p)
	}
	all, bs, pt, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	out := kfhPool{cards: map[string]*lCard{}, built: map[string]built{}, pt: pt}
	for i, c := range all {
		out.cards[c.Name] = c
		out.built[c.Name] = bs[i]
	}
	return out
}

// kfhClone deep-copies a card's first face so a test may edit its params.
func kfhClone(c *cards.Card) *cards.Card {
	n := *c
	f := *c.Faces[0]
	f.SVars = map[string]string{}
	for k, v := range c.Faces[0].SVars {
		f.SVars[k] = v
	}
	cp := func(m map[string]string) map[string]string {
		o := map[string]string{}
		for k, v := range m {
			o[k] = v
		}
		return o
	}
	f.Abilities = nil
	for _, a := range c.Faces[0].Abilities {
		x := *a
		x.Params = cp(a.Params)
		f.Abilities = append(f.Abilities, &x)
	}
	f.Statics = nil
	for _, s := range c.Faces[0].Statics {
		x := s
		x.Params = cp(s.Params)
		f.Statics = append(f.Statics, x)
	}
	n.Faces = append([]*cards.Face{&f}, c.Faces[1:]...)
	return &n
}

func (kp kfhPool) accepted(t *testing.T, name string) {
	t.Helper()
	c := kp.cards[name]
	if c == nil {
		t.Fatalf("%s not in the pool", name)
	}
	if len(lowerRefusals(c)) > 0 {
		t.Fatalf("%s refused: %v", name, lowerRefusals(c))
	}
}

func (kp kfhPool) refusedWith(t *testing.T, name, reason string) {
	t.Helper()
	c := kp.cards[name]
	if c == nil {
		t.Fatalf("%s not in the pool", name)
	}
	var got []string
	for _, r := range c.Refusals {
		got = append(got, r.Reason)
	}
	if all := strings.Join(got, " | "); !strings.Contains(all, reason) {
		t.Fatalf("%s refusals %q, want %q", name, all, reason)
	}
}

// filtersWith lists the pool filters with every bit of f2.
func (kp kfhPool) filtersWith(f2 kop.FilterFlags2) []kop.Filter {
	var out []kop.Filter
	for _, f := range kp.pt.filters {
		if f.Flags2&f2 == f2 {
			out = append(out, f)
		}
	}
	return out
}

func TestTargetedPlayerCtrlFilter(t *testing.T) {
	kp := kfhBuild(t, []string{"Villainous Wrath"}, nil)
	kp.accepted(t, "Villainous Wrath")
	fs := kp.filtersWith(kop.F2TgtPlayerCtrl)
	if len(fs) != 1 || fs[0].Ctrl != kop.CtrlAny || fs[0].Types != typeMask(tCreature) || fs[0].Zone != kop.ZoneNone {
		t.Fatalf("TargetedPlayerCtrl filters %+v, want one creature filter with Ctrl CtrlAny on the battlefield", fs)
	}

	kp = kfhBuild(t, []string{"Villainous Wrath"}, func(p *pool) {
		c := kfhClone(p.byName["Villainous Wrath"])
		c.Faces[0].SVars["X"] = "Count$ValidGraveyard Creature.TargetedPlayerCtrl"
		p.byName["Villainous Wrath"] = c
	})
	kp.refusedWith(t, "Villainous Wrath", "TargetedPlayerCtrl off the battlefield")
}

func TestDefendingPlayerCtrlFilter(t *testing.T) {
	carriers := []string{"Spring Splasher", "Thunder Lasso", "Captain America's Shield"}
	for _, n := range carriers {
		kp := kfhBuild(t, []string{n}, nil)
		kp.accepted(t, n)
		fs := kp.filtersWith(kop.F2DefendingCtrl)
		if len(fs) != 1 || fs[0].Ctrl != kop.CtrlAny || fs[0].Types != typeMask(tCreature) {
			t.Fatalf("%s: defending-player filters %+v, want one creature filter with Ctrl CtrlAny", n, fs)
		}
	}
	const why = "ControlledBy TriggeredDefendingPlayer outside an Attacks trigger's battlefield filter"
	// The Long Reach of Night's Count$ValidGraveyard sits on its back face
	// (the pool's token entry); the card itself is refused through it
	kp := kfhBuild(t, []string{"The Long Reach of Night"}, nil)
	var got []string
	for _, c := range kp.cards {
		for _, r := range c.Refusals {
			got = append(got, c.Name+": "+r.Reason)
		}
	}
	if all := strings.Join(got, " | "); !strings.Contains(all, why) || len(kp.cards["The Long Reach of Night"].Refusals) == 0 {
		t.Fatalf("The Long Reach of Night refusals %q, want %q", all, why)
	}

	// a static (no trigger at all) naming the defending player
	kp = kfhBuild(t, []string{"Spider-Ham, Peter Porker"}, func(p *pool) {
		c := kfhClone(p.byName["Spider-Ham, Peter Porker"])
		c.Faces[0].Statics[0].Params["Affected"] = "Creature.ControlledBy TriggeredDefendingPlayer"
		p.byName["Spider-Ham, Peter Porker"] = c
	})
	kp.refusedWith(t, "Spider-Ham, Peter Porker", why)
}

func TestCostTypeListSemicolon(t *testing.T) {
	for _, tc := range []struct {
		name string
		want [2]uint16
	}{{"Earthrumbler", [2]uint16{tCreature, tArtifact}}, {"Postmortem Professor", [2]uint16{tInstant, tSorcery}}} {
		kp := kfhBuild(t, []string{tc.name}, nil)
		kp.accepted(t, tc.name)
		var f *lFilter
		for _, a := range kp.cards[tc.name].Abilities {
			if a.Cost.ExileG != nil {
				f = a.Cost.ExileG
			}
		}
		if f == nil || len(f.Clauses) != 2 || f.Clauses[0].Types != tc.want[0] || f.Clauses[1].Types != tc.want[1] || f.Players != 0 {
			t.Fatalf("%s: ExileFromGrave filter %+v, want two clauses %#x, %#x", tc.name, f, tc.want[0], tc.want[1])
		}
	}
}

// TestNicknameSacSelf: Sac<1/NICKNAME> lowers as the source itself
// (kop.CostSacSelf), the same reading as CARDNAME: gorge's
// pay.SacrificeMatchSpec reads Forge's NICKNAME spec as CARDNAME and its
// priority window offers the activation (measured at this pin,
// engine/rules nickname_sac_test.go), so kgen copies gorge.
func TestNicknameSacSelf(t *testing.T) {
	const n = "Syr Ginger, the Meal Ender"
	kp := kfhBuild(t, []string{n}, nil)
	if rs := kp.built[n].refusals; len(rs) != 0 {
		t.Fatalf("%s refused: %q", n, keclReason(rs))
	}
	saw := false
	for i := range kp.built[n].def.Abilities {
		a := &kp.built[n].def.Abilities[i]
		if a.Cost.Flags&kop.CostSacSelf != 0 {
			saw = true
			if a.Cost.Flags&(kop.CostTap|kop.CostSac) != a.Cost.Flags&kop.CostTap {
				t.Errorf("Syr Ginger's activation cost flags %b: a source sacrifice beside a pick", a.Cost.Flags)
			}
		}
	}
	if !saw {
		t.Error("Syr Ginger built with no CostSacSelf activation: Sac<1/NICKNAME> did not lower")
	}
}

func TestAnimateTargetedNegatedType(t *testing.T) {
	const n = "Tezzeret, Cruel Captain"
	kp := kfhBuild(t, []string{n}, nil)
	kp.accepted(t, n)
	found := 0
	for _, f := range kp.pt.filters {
		if f.NotTypes == typeMask(tCreature) {
			found++
			if f.Types != 0 || f.Flags2 != 0 || f.Pred != kop.PredNone {
				t.Fatalf("the !Creature guard filter %+v, want NotTypes creature alone", f)
			}
		}
	}
	if found != 1 {
		t.Fatalf("%d filters with NotTypes creature, want the one DBAnimate guard", found)
	}

	kp = kfhBuild(t, []string{n}, func(p *pool) {
		c := kfhClone(p.byName[n])
		c.Faces[0].SVars["DBAnimate"] = strings.Replace(c.Faces[0].SVars["DBAnimate"], "Targeted.!Creature", "Targeted.!Frobnitz", 1)
		p.byName[n] = c
	})
	kp.refusedWith(t, n, `unknown property "!Frobnitz"`)
}

func TestSpellHeadCostTarget(t *testing.T) {
	kp := kfhBuild(t, []string{"Out of Air"}, nil)
	kp.accepted(t, "Out of Air")
	found := false
	for _, a := range kp.built["Out of Air"].def.Abilities {
		st := a.Static
		if st.Kind != kop.StatReduceCost {
			continue
		}
		found = true
		f := kp.pt.filters[st.Filter2]
		if st.StFlags&kop.StTargetsFilter2 == 0 || f.Zone != kop.ZoneStack || f.Flags&kop.FSpell == 0 || f.Types != typeMask(tCreature) || f.Or != 0 {
			t.Fatalf("Out of Air's reduction %+v with Filter2 %+v, want StTargetsFilter2 and a stack creature-spell filter", st, f)
		}
	}
	if !found {
		t.Fatal("Out of Air has no ReduceCost static")
	}

	kp = kfhBuild(t, []string{"Murder"}, func(p *pool) {
		c := kfhClone(p.byName["Murder"])
		c.Faces[0].Abilities[0].Params["ValidTgts"] = "Spell.Creature"
		p.byName["Murder"] = c
	})
	kp.refusedWith(t, "Murder", "a Spell head outside a ReduceCost ValidTarget$")
}

func TestAbsentCreatureSubtypeHead(t *testing.T) {
	const ham = "Spider-Ham, Peter Porker"
	kp := kfhBuild(t, []string{ham, "Universal Automaton", "Ornithopter"}, nil)
	for _, n := range []string{ham, "Universal Automaton", "Ornithopter"} {
		kp.accepted(t, n)
	}
	i, ok := kp.pt.subIdx["Jackal"]
	if !ok {
		t.Fatalf("Jackal not in the pool subtypes %v", kp.pt.subtypes)
	}
	has := false
	for _, s := range kp.built["Universal Automaton"].subs {
		has = has || int(s) == i
	}
	if !has {
		t.Fatalf("Universal Automaton (a Changeling) subtypes %v lack Jackal (%d)", kp.built["Universal Automaton"].subs, i)
	}
	for _, s := range kp.built["Ornithopter"].subs {
		if int(s) == i {
			t.Fatal("Ornithopter has Jackal")
		}
	}

	kp = kfhBuild(t, []string{ham}, func(p *pool) {
		c := kfhClone(p.byName[ham])
		c.Faces[0].Statics[0].Params["Affected"] = "Frobnitz.Other+YouCtrl"
		p.byName[ham] = c
	})
	kp.refusedWith(t, ham, `unknown filter head "Frobnitz"`)
}
