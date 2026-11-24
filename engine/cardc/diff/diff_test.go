package diff

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cardc/slice"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestCanon(t *testing.T) {
	sv := map[string]string{"X": "Count$xPaid", "Y": "X"}
	cases := []struct{ name, key, in, want string }{
		{"whitespace", "Amount", "  a   b ", "a b"},
		{"svar", "NumDmg", "X", "=Count$xPaid"},
		{"svar one level", "NumDmg", "Y", "=X"},
		{"list", "ValidTgts", "Planeswalker,Creature", "Creature,Planeswalker"},
		{"plus terms", "ValidCards", "Creature.Red+Tapped,Artifact", "Artifact,Creature.Red+Tapped"},
		{"plus terms sorted", "ValidCards", "Creature.Tapped+Red", "Creature.Red+Tapped"},
		{"addkeyword", "AddKeyword", "Flying & Haste", "Flying & Haste"},
		{"addkeyword sorted", "AddKeyword", "Haste & Flying", "Flying & Haste"},
		{"bool", "SorcerySpeed", "true", "True"},
		{"bool false", "SorcerySpeed", "FALSE", "False"},
		{"cost unsplit", "Cost", "T,Sac<1/Creature>", "T,Sac<1/Creature>"},
	}
	for _, c := range cases {
		if got := canonKey(c.key, c.in, sv); got != c.want {
			t.Errorf("%s: %q -> %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func parse(t *testing.T, text string) *cards.Card {
	t.Helper()
	c, diags := gorgebe.ParseScript("zz.txt", []byte(text))
	if len(diags) > 0 || c == nil {
		t.Fatalf("parse: %v\n%s", diags, text)
	}
	return c
}

const hdr = "Name:Zz Probe\nManaCost:1 R\nTypes:Instant\n"

func rowsOf(t *testing.T, a, b string) []Row {
	t.Helper()
	return Card("Zz Probe", parse(t, hdr+a), parse(t, hdr+b))
}

func TestCardRows(t *testing.T) {
	t.Run("amount", func(t *testing.T) {
		r := rowsOf(t, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\n", "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2\n")
		if len(r) != 1 || r[0].Field != "[0]NumDmg" || r[0].Family != "amounts" || r[0].Cardc != "3" || r[0].Forge != "2" ||
			r[0].Item != "A:SP:DealDamage#0" || r[0].Key() != "0/A:SP:DealDamage#0/[0]NumDmg" || len(r[0].Sig()) != 8 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("list order", func(t *testing.T) {
		if r := rowsOf(t, "A:SP$ Destroy | ValidTgts$ Creature,Artifact\n", "A:SP$ Destroy | ValidTgts$ Artifact,Creature\n"); len(r) != 0 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("svar names", func(t *testing.T) {
		if r := rowsOf(t, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ X\nSVar:X:Count$xPaid\n",
			"A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ Y\nSVar:Y:Count$xPaid\n"); len(r) != 0 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("target default", func(t *testing.T) {
		if r := rowsOf(t, "A:SP$ Destroy | ValidTgts$ Creature | TargetMax$ 1\n", "A:SP$ Destroy | ValidTgts$ Creature\n"); len(r) != 0 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("trigger line", func(t *testing.T) {
		r := rowsOf(t, "T:Mode$ ChangesZone | ValidCard$ Card.Self | Execute$ TrigA\nSVar:TrigA:DB$ Draw | NumCards$ 1\n",
			"T:Mode$ ChangesZone | ValidCard$ Creature | Execute$ TrigA\nSVar:TrigA:DB$ Draw | NumCards$ 1\n")
		if len(r) != 1 || r[0].Field != "ValidCard" || r[0].Family != "trigger" || r[0].Item != "T:ChangesZone:Draw#0" {
			t.Errorf("%+v", r)
		}
	})
	t.Run("chain node", func(t *testing.T) {
		r := rowsOf(t, "A:SP$ Draw | NumCards$ 1 | SubAbility$ D\nSVar:D:DB$ Draw | NumCards$ 2\n",
			"A:SP$ Draw | NumCards$ 1 | SubAbility$ D\nSVar:D:DB$ Draw | NumCards$ 3\n")
		if len(r) != 1 || r[0].Field != "[1]NumCards" {
			t.Errorf("%+v", r)
		}
	})
	t.Run("extra ability", func(t *testing.T) {
		r := rowsOf(t, "A:SP$ Draw | NumCards$ 1\nK:Flash\n", "A:SP$ Draw | NumCards$ 1\n")
		if len(r) != 1 || r[0].Family != Shape || r[0].Field != "-" || r[0].Cardc != "present" || r[0].Forge != "" {
			t.Errorf("%+v", r)
		}
	})
	t.Run("face counts", func(t *testing.T) {
		two := parse(t, hdr+"ALTERNATE\nName:Zz Back\nManaCost:no cost\nTypes:Instant\n")
		r := Card("Zz Probe", two, parse(t, hdr))
		if len(r) != 1 || r[0].Item != "faces" || r[0].Field != "-" || r[0].Family != Shape || r[0].Cardc != "2" || r[0].Forge != "1" {
			t.Errorf("%+v", r)
		}
	})
	t.Run("description only", func(t *testing.T) {
		if r := rowsOf(t, "A:SP$ Draw | NumCards$ 1 | SpellDescription$ One.\n", "A:SP$ Draw | NumCards$ 1 | SpellDescription$ Two.\n"); len(r) != 0 {
			t.Errorf("%+v", r)
		}
	})
}

func TestFamiliesDisjoint(t *testing.T) {
	seen := map[string]string{}
	for _, f := range familyKeys {
		for _, k := range f.keys {
			if prev, dup := seen[k]; dup {
				t.Errorf("key %s is in %s and %s", k, prev, f.family)
			}
			seen[k] = f.family
		}
	}
	if len(Families()) != len(familyKeys)+1 {
		t.Error("Families")
	}
}

func TestDiffIdentityIsClean(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range []string{"fdn", "legacy"} {
		names, err := mtgish.SliceNames(root, s)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			c, ok := forge.Lookup(name)
			if !ok {
				continue
			}
			b, err := os.ReadFile(c.Path)
			if err != nil {
				t.Fatal(err)
			}
			re, diags := gorgebe.ParseScript(c.Path, b)
			if len(diags) > 0 {
				continue
			}
			n++
			if rows := Card(name, re, c); len(rows) > 0 {
				t.Errorf("%s: %d rows, first %+v", name, len(rows), rows[0])
			}
		}
	}
	if n == 0 {
		t.Error("no cards compared")
	}
	t.Logf("%d cards", n)
}

func TestDiffSlice(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := slice.Build(root, forge, "fdn", "legacy")
	if errors.Is(err, slice.ErrNoData) {
		t.Skip(mtgish.SkipMessage)
	}
	if err != nil {
		t.Fatal(err)
	}
	fams := map[string]bool{}
	for _, f := range Families() {
		fams[f] = true
	}
	perFam := map[string]int{}
	served, differing, total := 0, 0, 0
	for _, e := range reg.Entries() {
		if e.Origin != gorgebe.OriginCardc {
			continue
		}
		served++
		a, b := Card(e.Name, e.Card, e.Forge), Card(e.Name, e.Card, e.Forge)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: rows not deterministic", e.Name)
		}
		if len(a) > 0 {
			differing++
		}
		for _, r := range a {
			total++
			perFam[r.Family]++
			if !fams[r.Family] || r.Family == Shape {
				t.Errorf("%s: row %s has family %q", e.Name, r.Key(), r.Family)
			}
		}
	}
	var sb strings.Builder
	for f, n := range perFam {
		sb.WriteString(f + "=" + string(rune('0'+n/100%10)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10)) + " ")
	}
	t.Logf("served %d, differing %d, rows %d; %s", served, differing, total, sb.String())
}
