package rules

import (
	"sort"
	"strings"
	"testing"

	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cost_grammar_class_test.go: the variable and compound cost grammar class
// Four mechanisms, each with an end-to-end pin through the
// oracle runner and one corpus census:
//
//   - the open-ended group exile ("exile one or more ... with total mana
//     value X", "any number ... with fifteen or more black mana symbols"),
//     CostPart.AtLeast + CostPart.Group (CR 601.2b announces the X the set
//     must total; CR 601.2h pays the set);
//   - the target-bound cost filter (a Sac/Discard spec reading an X the
//     ability's targets fix: CR 601.2c chooses targets before CR 601.2h pays,
//     via CR 602.2b), cost_target_bound.go;
//   - the ChooseCreatureType<N> cost token (CR 601.2h: choosing is the
//     payment; the type is recorded on the spell and the permanent it
//     becomes);
//   - the CastInfo fold that dropped an optional-cost cast's {X}.

// runScenarioExpectFail plays a scenario and returns its failures (the test
// asserts a specific one).
func runScenarioExpectFail(t *testing.T, raw string) []string {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, _, _ := runOracleScenario(reg, sc)
	return fails
}

func lastSnapPerm(res OracleResult, name string) (OracleSnapPerm, bool) {
	snap := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range snap.Permanents {
		if p.Name == name {
			return p, true
		}
	}
	return OracleSnapPerm{}, false
}

// TestGroupExileCostAnnouncesReachableTotals: Fabrication Foundry's "{2}{W},
// {T}, Exile one or more other artifacts you control with total mana value X:
// Return target artifact card with mana value X or less". X is announced
// from the totals the artifacts can reach (Millstone 2 + Bonesplitter 1:
// 1, 2, 3), the exile election must total exactly X, and the target is
// returned.
func TestGroupExileCostAnnouncesReachableTotals(t *testing.T) {
	const setup = `"setup":{"p0":{"battlefield":["Fabrication Foundry","Millstone","Bonesplitter"],"graveyard":["Ornithopter"]},"p1":{"battlefield":["Grizzly Bears"]}}`
	res, run := runFixtureScenario(t, `{"name":"foundry",`+setup+`,"steps":[
		{"op":"activate","seat":0,"card":"p0:Fabrication Foundry","ability_index":1,"mana":"CCW","targets":["p0:Ornithopter"],
		 "answers":[{"kind":"choose","pick":["X = 3"]},{"kind":"choose","pick":["Millstone","Bonesplitter"]}]},
		{"op":"resolve","seat":0}]}`)
	if d := firstDecisionOfKind(run.decisions, "x"); d == nil || d.Options != 3 {
		t.Fatalf("X announcement = %+v, want the three reachable totals 1, 2, 3", d)
	}
	exile := res.Snapshots[len(res.Snapshots)-1].Players[0].Exile
	sort.Strings(exile)
	if strings.Join(exile, ",") != "Bonesplitter,Millstone" {
		t.Errorf("p0 exile = %v, want Bonesplitter and Millstone (total 3 = X)", exile)
	}
	if _, ok := lastSnapPerm(res, "Ornithopter"); !ok {
		t.Errorf("Ornithopter was not returned to the battlefield")
	}
	// An election short of the announced total is not an answer
	// (Decision.MinSum/MaxSum): Millstone alone totals 2, not 3.
	fails := runScenarioExpectFail(t, `{"name":"foundry-short",`+setup+`,"steps":[
		{"op":"activate","seat":0,"card":"p0:Fabrication Foundry","ability_index":1,"mana":"CCW","targets":["p0:Ornithopter"],
		 "answers":[{"kind":"choose","pick":["X = 3"]},{"kind":"choose","pick":["Millstone"]}]}]}`)
	if len(fails) == 0 || !strings.Contains(strings.Join(fails, " "), "required sum") {
		t.Errorf("a short election was accepted: fails=%v", fails)
	}
}

func firstDecisionOfKind(ds []OracleDecision, pickKind string) *OracleDecision {
	for i := range ds {
		for _, k := range ds[i].PickKinds {
			if k == pickKind {
				return &ds[i]
			}
		}
	}
	return nil
}

// TestGroupExileFloorAndExiledWithLink: Baron Helmut Zemo's Boast "Exile any
// number of black cards from your graveyard with fifteen or more black mana
// symbols among their mana costs: Copy those exiled cards." The floor is a
// set rule (4 + 4 + 5 + 4 = 17 symbols pays; 12 does not offer the ability),
// and the exiled cards are linked to Zemo, which is what "those exiled
// cards" (Defined$ ExiledWith) reads -- the copies are offered.
func TestGroupExileFloorAndExiledWithLink(t *testing.T) {
	steps := `"steps":[{"op":"attack","seat":0,"defender":"p1","attackers":["p0:Baron Helmut Zemo"]},{"op":"pass_to","step":"main2"},
		{"op":"activate","seat":0,"card":"p0:Baron Helmut Zemo","ability_index":0},{"op":"resolve","seat":0}]`
	_, run := runFixtureScenario(t, `{"name":"zemo","setup":{"p0":{"battlefield":["Baron Helmut Zemo"],
		"graveyard":["Phyrexian Obliterator","Nefarious Lich","Pox Plague","Lich"]},"p1":{"battlefield":["Grizzly Bears"]}},`+steps+`}`)
	zemo := findByName(run.e, "Baron Helmut Zemo", 0)
	linked := 0
	for _, id := range run.e.G.Zone(state.ZExile, 0) {
		if o := run.e.G.Obj(id); o != nil && o.ExiledWith == zemo {
			linked++
		}
	}
	if linked != 4 {
		t.Errorf("%d exiled cards are linked to Zemo, want 4", linked)
	}
	offered := false
	for _, d := range run.decisions {
		if d.Kind == "mode" && d.Resume == "play" && len(d.OptionRefs) == 4 {
			offered = true
		}
	}
	if !offered {
		t.Errorf("the copies of the four exiled cards were not offered: %+v", run.decisions)
	}
	fails := runScenarioExpectFail(t, `{"name":"zemo-short","setup":{"p0":{"battlefield":["Baron Helmut Zemo"],
		"graveyard":["Phyrexian Obliterator","Nefarious Lich","Lich"]},"p1":{"battlefield":["Grizzly Bears"]}},`+steps+`}`)
	if len(fails) == 0 || !strings.Contains(strings.Join(fails, " "), "not offered") {
		t.Errorf("Zemo's Boast was offered with only 12 black mana symbols: fails=%v", fails)
	}
}

// TestTargetBoundSacrificeFollowsTheTarget: Sidisi, Regent of the Mire's
// "{T}, Sacrifice a creature you control with mana value X other than
// Sidisi: Return target creature card with mana value X plus 1". The target
// is chosen first (CR 601.2c before 601.2h): only a graveyard creature whose
// mana value minus one some creature matches is offered (Centaur Courser 3,
// not Colossal Dreadmaw 6), and the sacrifice is asked after it, offering
// only the mana-value-2 creature.
func TestTargetBoundSacrificeFollowsTheTarget(t *testing.T) {
	res, run := runFixtureScenario(t, `{"name":"sidisi","setup":{"p0":{"battlefield":["Sidisi, Regent of the Mire","Llanowar Elves","Grizzly Bears"],
		"graveyard":["Centaur Courser","Colossal Dreadmaw"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[
		{"op":"activate","seat":0,"card":"p0:Sidisi, Regent of the Mire","ability_index":0},{"op":"resolve","seat":0}]}`)
	tgt, sac := -1, -1
	for i, d := range run.decisions {
		switch {
		case d.Kind == "target" && tgt < 0:
			tgt = i
			if len(d.OptionRefs) != 1 || d.OptionRefs[0] != "p0:Centaur Courser" {
				t.Errorf("target options %v, want only Centaur Courser", d.OptionRefs)
			}
		case len(d.PickKinds) > 0 && d.PickKinds[0] == "sacrifice":
			sac = i
			if len(d.OptionRefs) != 1 || d.OptionRefs[0] != "p0:Grizzly Bears" {
				t.Errorf("sacrifice options %v, want only Grizzly Bears (mana value 3 - 1)", d.OptionRefs)
			}
		}
	}
	if tgt < 0 || sac < 0 || tgt > sac {
		t.Fatalf("target decision #%d, sacrifice decision #%d: want the target first (CR 601.2c before 601.2h)", tgt, sac)
	}
	if _, ok := lastSnapPerm(res, "Centaur Courser"); !ok {
		t.Errorf("Centaur Courser was not returned")
	}
	// No creature at any target's mana value minus one: not offered.
	fails := runScenarioExpectFail(t, `{"name":"sidisi-none","setup":{"p0":{"battlefield":["Sidisi, Regent of the Mire","Llanowar Elves"],
		"graveyard":["Colossal Dreadmaw"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[
		{"op":"activate","seat":0,"card":"p0:Sidisi, Regent of the Mire","ability_index":0}]}`)
	if len(fails) == 0 || !strings.Contains(strings.Join(fails, " "), "not offered") {
		t.Errorf("Sidisi was offered with no sacrifice matching any target: fails=%v", fails)
	}
}

// TestChooseCreatureTypeCostRecordsTheType: Caller of the Hunt's "As an
// additional cost to cast this spell, choose a creature type" is asked and
// recorded on the permanent: two Bears make it 2/2 (it survives).
func TestChooseCreatureTypeCostRecordsTheType(t *testing.T) {
	res, _ := runFixtureScenario(t, `{"name":"caller","setup":{"p0":{"hand":["Caller of the Hunt"],"battlefield":["Grizzly Bears","Runeclaw Bear"]},"p1":{"battlefield":["Llanowar Elves"]}},"steps":[
		{"op":"cast","seat":0,"card":"p0:Caller of the Hunt","mana":"CCG","answers":[{"kind":"choose","pick":["Bear"]}]},{"op":"resolve","seat":0}]}`)
	p, ok := lastSnapPerm(res, "Caller of the Hunt")
	if !ok || p.PT != "2/2" {
		t.Fatalf("Caller of the Hunt = %+v (on battlefield %v), want a 2/2 counting the chosen Bears", p, ok)
	}
}

// TestOptionalChooseTypeBeholdKeepsX: Celestial Reunion cast with its
// optional "choose a creature type and behold two creatures of that type"
// paid keeps its {X} (the CastInfo fold dropped it), offers only a type the
// behold can pay, and puts the found card of the chosen type onto the
// battlefield.
func TestOptionalChooseTypeBeholdKeepsX(t *testing.T) {
	res, run := runFixtureScenario(t, `{"name":"reunion","setup":{"p0":{"hand":["Celestial Reunion"],"battlefield":["Runeclaw Bear","Bear Cub"],
		"library_top":["Grizzly Bears","Ornithopter"]},"p1":{"battlefield":["Llanowar Elves"]}},"steps":[
		{"op":"cast","seat":0,"card":"p0:Celestial Reunion","mana":"CCG","cast_mode":"optionalcost","answers":[{"kind":"choose","pick":["X = 2"]}]},
		{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["Grizzly Bears"]}]}]}`)
	if d := firstDecisionOfKind(run.decisions, "type"); d == nil || d.Options != 1 || d.First != "Bear" {
		t.Fatalf("type choice = %+v, want only the payable Bear", d)
	}
	if _, ok := lastSnapPerm(res, "Grizzly Bears"); !ok {
		t.Errorf("the found Bear did not enter the battlefield: the paid branch or X was lost")
	}
}

// TestCostGrammarCensus is the class ratchet over every corpus Cost$: each
// open-ended exile count parses to an AtLeast part with its withTotal* group
// modelled, each ChooseCreatureType token parses to Cost.ChooseType, and
// each cost spec comparing against an X the source binds to its targets is
// a Sac or Discard part spelled with X -- the shapes cost_target_bound.go
// serves. Each carrier list is pinned in both directions.
func TestCostGrammarCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	group, chooseType, targetBound := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			walkCostParams(f, func(raw string) {
				for it := newCostTokenIter(raw); ; {
					tok, ok := it.Next()
					if !ok {
						break
					}
					one := ParseCost(tok)
					head, payload, found := strings.Cut(tok, "<")
					count, rest, _ := strings.Cut(strings.TrimSuffix(payload, ">"), "/")
					spec, _, _ := strings.Cut(rest, "/")
					switch {
					case found && strings.HasSuffix(count, "+") && isDigitRun(strings.TrimSuffix(count, "+")):
						if len(one.Unknown) != 0 || len(one.Exile) != 1 || !one.Exile[0].AtLeast {
							t.Errorf("%s: %s does not parse to an AtLeast exile part: %+v", f.Name, tok, one)
						} else if strings.Contains(tok, "withTotal") && !one.Exile[0].Group.Active() {
							t.Errorf("%s: %s keeps its withTotal predicate unmodelled", f.Name, tok)
						}
						group[f.Name] = true
					case strings.HasPrefix(tok, "ChooseCreatureType<"):
						if len(one.Unknown) != 0 || one.ChooseType != 1 {
							t.Errorf("%s: %s does not parse to one ChooseType: %+v", f.Name, tok, one)
						}
						chooseType[f.Name] = true
					}
					if !found {
						continue
					}
					for _, name := range []string{"X", "Y", "Z"} {
						if !costvocab.SpecComparesVar(spec, name) || !svarReadsTargets(f.SVars[name]) {
							continue
						}
						if name != costTargetVarName || (head != "Sac" && head != "Discard") {
							t.Errorf("%s: target-bound cost token %s is not a Sac/Discard part spelled with %s", f.Name, tok, costTargetVarName)
						}
						targetBound[f.Name] = true
					}
				}
			})
		}
	}
	for _, tc := range []struct {
		what string
		got  map[string]bool
		want []string
	}{
		{"group exile", group, []string{"Baron Helmut Zemo", "Fabrication Foundry", "The Capitoline Triad"}},
		{"ChooseCreatureType", chooseType, []string{"Caller of the Hunt", "Celestial Reunion"}},
		{"target-bound cost filter", targetBound, []string{"Kozilek, the Great Distortion", "Scrap Welder", "Sidisi, Regent of the Mire"}},
	} {
		got := make([]string, 0, len(tc.got))
		for n := range tc.got {
			got = append(got, n)
		}
		sort.Strings(got)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s carriers = %v, want %v (a new carrier needs its shape checked; a vanished one, the table updated)", tc.what, got, tc.want)
		}
	}
}
