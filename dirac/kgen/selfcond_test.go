package kgen

import "testing"

// TestSelfCondReadKinds pins condRd's read model: every layer-3..7
// characteristic a filter can read is named, and the flags/stats that read
// no characteristic do not force r.all (which would refuse every write).
func TestSelfCondReadKinds(t *testing.T) {
	rd := func(c lClause) condRd { return filterRd(&lFilter{Clauses: []lClause{c}}) }
	cases := []struct {
		name string
		r    condRd
		want func(condRd) bool
	}{
		{"card type", rd(lClause{Types: tCreature}), func(r condRd) bool { return r.types&tCreature != 0 }},
		{"subtype", rd(lClause{Subtype: "Soldier"}), func(r condRd) bool { return r.sub }},
		{"colour", rd(lClause{Colors: "W"}), func(r condRd) bool { return r.col }},
		{"chosen type", rd(lClause{Flags: fChosenType}), func(r condRd) bool { return r.sub }},
		{"chosen colour", rd(lClause{Flags: fChosenColor}), func(r condRd) bool { return r.col }},
		{"keyword", rd(lClause{Flags: fWithFlying}), func(r condRd) bool { return r.kw }},
		{"name", rd(lClause{Flags: fNamedAsSource}), func(r condRd) bool { return r.name }},
		{"basic", rd(lClause{Flags: fBasic}), func(r condRd) bool { return r.types&tBasic != 0 }},
		{"legendary", rd(lClause{Flags: fLegendary}), func(r condRd) bool { return r.types&tLegendary != 0 }},
		{"shared colour", rd(lClause{Flags: fSharesColorWithOtherRemembered}), func(r condRd) bool { return r.col }},
		{"power", rd(lClause{CmpStat: cmpPower}), func(r condRd) bool { return r.pt }},
		{"toughness", rd(lClause{CmpStat: cmpToughness}), func(r condRd) bool { return r.pt }},
		{"power vs toughness", rd(lClause{CmpStat: cmpPowerLTToughness}), func(r condRd) bool { return r.pt }},
		{"has keyword", rd(lClause{CmpStat: cmpHasKw}), func(r condRd) bool { return r.kw }},
		{"multicolour", rd(lClause{CmpStat: cmpMultiColor}), func(r condRd) bool { return r.col }},
		{"historic", rd(lClause{CmpStat: cmpHistoric}), func(r condRd) bool { return r.types&tArtifact != 0 && r.sub }},
		{"counter", rd(lClause{CmpStat: cmpCounters}), func(r condRd) bool { return r == condRd{} }},
		{"mana value", rd(lClause{CmpStat: cmpCMC}), func(r condRd) bool { return r == condRd{} }},
		{"attached", rd(lClause{CmpStat: cmpAttached}), func(r condRd) bool { return r == condRd{} }},
		{"enchanted", rd(lClause{CmpStat: cmpEnchanted}), func(r condRd) bool { return r == condRd{} }},
		{"modified", rd(lClause{CmpStat: cmpModified}), func(r condRd) bool { return r == condRd{} }},
		{"tapped", rd(lClause{Flags: fTapped}), func(r condRd) bool { return r == condRd{} }},
		{"unclassified flag (sneaked)", rd(lClause{Flags: fSneaked}), func(r condRd) bool { return r.all }},
		{"unknown stat", rd(lClause{CmpStat: 255}), func(r condRd) bool { return r.all }},
	}
	for _, c := range cases {
		if !c.want(c.r) {
			t.Errorf("%s: read %+v", c.name, c.r)
		}
	}
}

// TestSelfCondWriteOverlap pins selfCondCheck: only the write class that
// can change what the condition reads refuses; every other pairing lowers.
func TestSelfCondWriteOverlap(t *testing.T) {
	lc := &lowerCtx{out: &lCard{}}
	self := &lFilter{Clauses: []lClause{{Flags: fSelf}}}
	cases := []struct {
		name string
		st   *lStatic
		want bool // refuses
	}{
		{"type read, type write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Types: tCreature}}}}, Override: &lOverride{AddTypes: tCreature}}, true},
		{"type read, subtype write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Types: tCreature}}}}, Override: &lOverride{AddSubtypes: []string{"Golem"}}}, false},
		{"subtype read, subtype write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Subtype: "Soldier"}}}}, Override: &lOverride{AddSubtypes: []string{"Golem"}}}, true},
		{"subtype read, type write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Subtype: "Soldier"}}}}, Override: &lOverride{AddTypes: tCreature}}, false},
		{"colour read, colour write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Colors: "W"}}}}, Override: &lOverride{HasSetColors: true, SetColors: "G"}}, true},
		{"colour read, type write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Colors: "W"}}}}, Override: &lOverride{AddTypes: tCreature}}, false},
		{"chosen type read, subtype write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Flags: fChosenType}}}}, Override: &lOverride{AddSubtypes: []string{"Golem"}}}, true},
		{"chosen colour read, colour write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Flags: fChosenColor}}}}, Override: &lOverride{HasSetColors: true, SetColors: "G"}}, true},
		{"PT read, PT write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{CmpStat: cmpPower}}}}, Override: &lOverride{HasSetPT: true, SetP: 3, SetT: 3}}, true},
		{"PT read, type write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{CmpStat: cmpPower}}}}, Override: &lOverride{AddTypes: tCreature}}, false},
		{"keyword read, keyword write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Flags: fWithFlying}}}}, Override: &lOverride{AddKw: []string{"Flying"}}}, true},
		{"keyword read, type write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Flags: fWithFlying}}}}, Override: &lOverride{AddTypes: tCreature}}, false},
		{"name read, name write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Flags: fNamedAsSource}}}}, Override: &lOverride{Name: "X"}}, true},
		{"name read, type write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{Flags: fNamedAsSource}}}}, Override: &lOverride{AddTypes: tCreature}}, false},
		{"attached read, PT write", &lStatic{F: self, Cond: lCond{Kind: cPresent, F: &lFilter{Clauses: []lClause{{CmpStat: cmpAttached}}}}, Override: &lOverride{HasSetPT: true}}, false},
	}
	for _, c := range cases {
		err := lc.selfCondCheck(c.st)
		if got := err != nil; got != c.want {
			t.Errorf("%s: refused=%v err=%v, want refused=%v", c.name, got, err, c.want)
		}
	}
}
