package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

// TestManifestGuard: a card whose build carries an inactive manifest code,
// in a typed field, an OpExt sub-op, or a pool expression, filter or
// condition it reaches, is refused under the entry's class; an active code
// and a card carrying none build.
func TestManifestGuard(t *testing.T) {
	entry := func(fam, name string, v int64, st manifest.StatusKind) *manifest.Entry {
		why := ""
		if st == manifest.Deferred {
			why = "w"
		}
		return &manifest.Entry{Path: manifest.Dir + "/" + fam + "/" + name, Family: fam, Name: name, Value: v, Status: manifest.Status{Kind: st, Why: why}}
	}
	g := newCodeGuard([]*manifest.Entry{
		entry("TrigEvent", "TrigFoo", 200, manifest.Reserved),
		entry("TrigEvent", "TrigOn", 201, manifest.Active),
		entry("ExtOp", "ExtFoo", 300, manifest.Reserved),
		entry("ExprKind", "ExFoo", 500, manifest.Deferred),
		entry("CounterKind", "CtrFoo", 100, manifest.Reserved),
		entry("Zone", "ZoneFoo", 40, manifest.Reserved),
		entry("TrigFlags2", "Tf2Foo", 1<<30, manifest.Reserved),
	})
	pt := &poolTables{
		exprs:   []kop.Expr{{}, {Kind: 500}, {Kind: kop.ExCount_ - 1, Sub: [2]kop.ValueRef{{K: kop.VExpr, N: 1}}}},
		filters: []kop.Filter{{}, {Zone: 40}},
		conds:   []kop.Cond{{}, {F: 1}},
	}
	trig := func(ev kop.TrigEvent) kop.CardDef {
		return kop.CardDef{Abilities: []kop.Ability{{Trig: kop.Trigger{Event: ev}}}}
	}
	cases := []struct {
		name string
		def  kop.CardDef
		want string
	}{
		{"plain", trig(1), ""},
		{"active", trig(201), ""},
		{"typed", trig(200), "manifest:reserved:TrigEvent/TrigFoo"},
		{"flag bit", kop.CardDef{Abilities: []kop.Ability{{Trig: kop.Trigger{Flags2: 1 | 1<<30}}}}, "manifest:reserved:TrigFlags2/Tf2Foo"},
		{"ext", kop.CardDef{Abilities: []kop.Ability{{Prog: []kop.Instr{kop.ExtInstr(300)}}}}, "manifest:reserved:ExtOp/ExtFoo"},
		{"counter cost", kop.CardDef{Abilities: []kop.Ability{{Cost: kop.Cost{Ctr: 100}}}}, "manifest:reserved:CounterKind/CtrFoo"},
		{"nested expr", kop.CardDef{Abilities: []kop.Ability{{Prog: []kop.Instr{{Op: kop.OpDraw, A: kop.ValueRef{K: kop.VExpr, N: 2}}}}}}, "manifest:deferred:ExprKind/ExFoo"},
		{"filter", kop.CardDef{Abilities: []kop.Ability{{Prog: []kop.Instr{{Op: kop.OpDestroy, F: 1}}}}}, "manifest:reserved:Zone/ZoneFoo"},
		{"cond", kop.CardDef{Abilities: []kop.Ability{{Trig: kop.Trigger{Cond: 1}}}}, "manifest:reserved:Zone/ZoneFoo"},
	}
	var all []*lCard
	var bs []built
	for _, c := range cases {
		all = append(all, &lCard{Name: c.name})
		bs = append(bs, built{def: c.def})
	}
	g.refuse(pt, all, bs)
	for i, c := range cases {
		got := ""
		if rs := all[i].Refusals; len(rs) == 1 {
			got = refusalClasses(rs[0].Reason)[0]
		} else if len(rs) > 1 {
			got = "several"
		}
		if got != c.want {
			t.Errorf("%s: class %q, want %q", c.name, got, c.want)
		}
	}
}

// TestManifestGuardEmbedded: the embedded manifest reads, and this tree's
// built tables carry no inactive code (the guard refuses nothing at the
// cutover; TestCorpusBuildAcceptance holds the counts).
func TestManifestGuardEmbedded(t *testing.T) {
	g, err := embeddedGuard()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.inactive) == 0 || len(g.ext) == 0 {
		t.Errorf("the embedded manifest has no inactive codes: %d typed families, %d ExtOps", len(g.inactive), len(g.ext))
	}
}
