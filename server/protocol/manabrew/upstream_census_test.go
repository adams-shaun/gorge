//go:build manabrew

package manabrew

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// upstreamVersion is the ManaBrew protocol version the census was read
// against. Bumping it means re-reading the upstream docs and the TSV.
const upstreamVersion = "6.4.1"

// upstreamCatalogue is every message, directive, prompt type and error code
// the upstream protocol pages (CC BY 4.0, docs.manabrew.app/protocol) name.
// It is written independently of gorge's own types so a kind upstream adds
// and gorge never heard of fails the census instead of passing it.
var upstreamCatalogue = map[string][]string{
	"engineMessage": {"state", "stateDelta", "display", "prompt", "engineDirective", "error"},
	"clientMessage": {"response", "directive"},
	"directive":     {"concede", "startRecording", "stopRecording", "requestRepeat", "requestRestore", "repeatVote"},
	"prompt": {"chooseNumber", "chooseCards", "chooseColor", "chooseBoolean", "chooseFromSelection",
		"revealCards", "scry", "reorder", "diceRolled", "chooseAction", "payManaCost", "mulligan",
		"mulliganPutBack", "chooseAttackers", "chooseBlockers", "chooseDamageAssignmentOrder",
		"chooseCombatDamageAssignment", "chooseBoardTargets", "gameOver"},
	"errorCode": {"stalePrompt", "wrongPlayer", "wrongPromptType", "unknownActionId", "invalidShape"},
}

type censusRow struct{ class, name, status, code, note string }

func parseCensus(data string) (version string, rows []censusRow, errs []string) {
	for i, line := range strings.Split(data, "\n") {
		if v, ok := strings.CutPrefix(line, "# upstream protocol version:"); ok {
			version = strings.TrimSpace(v)
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 5 {
			errs = append(errs, "line "+strconv.Itoa(i+1)+": want 5 tab-separated columns")
			continue
		}
		r := censusRow{f[0], f[1], f[2], f[3], f[4]}
		switch r.status {
		case "mapped", "n/a":
			if r.code != "" {
				errs = append(errs, r.class+" "+r.name+": "+r.status+" carries no code")
			}
		case "refused":
			if r.code == "" {
				errs = append(errs, r.class+" "+r.name+": refused needs the code it is refused with")
			}
		default:
			errs = append(errs, r.class+" "+r.name+": status "+r.status+" is not mapped, refused or n/a")
		}
		rows = append(rows, r)
	}
	return version, rows, errs
}

// censusProblems is the whole census check, over a TSV text, so the test can
// prove the failure path by feeding it a doctored table.
func censusProblems(data string) []string {
	version, rows, errs := parseCensus(data)
	if version != upstreamVersion {
		errs = append(errs, "version pin is "+version+", want "+upstreamVersion)
	}
	listed := map[string]bool{}
	for _, r := range rows {
		if listed[r.class+"/"+r.name] {
			errs = append(errs, "duplicate row "+r.class+"/"+r.name)
		}
		listed[r.class+"/"+r.name] = true
	}
	known := map[string]bool{}
	for class, names := range upstreamCatalogue {
		for _, n := range names {
			known[class+"/"+n] = true
			if !listed[class+"/"+n] {
				errs = append(errs, "upstream "+class+" "+n+" is not listed in the census")
			}
		}
	}
	for _, r := range rows {
		if !known[r.class+"/"+r.name] {
			errs = append(errs, "census row "+r.class+"/"+r.name+" is not in the upstream catalogue")
		}
	}
	sort.Strings(errs)
	return errs
}

func TestDocsUpstreamCensus(t *testing.T) {
	b, err := os.ReadFile("testdata/upstream_census.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range censusProblems(string(b)) {
		t.Error(p)
	}
}

// TestDocsUpstreamCensusFailsOnDrift proves the check can fail: a dropped
// row, an invented one, a wrong version and a refusal without a code.
func TestDocsUpstreamCensusFailsOnDrift(t *testing.T) {
	b, err := os.ReadFile("testdata/upstream_census.tsv")
	if err != nil {
		t.Fatal(err)
	}
	good := string(b)
	if p := censusProblems(good); len(p) != 0 {
		t.Fatalf("precondition: census is not clean: %v", p)
	}
	cases := map[string]string{
		"unlisted upstream kind": strings.Replace(good, "prompt\tscry\tmapped\t\t\n", "", 1),
		"row upstream lacks":     good + "prompt\tfutureThing\tmapped\t\t\n",
		"wrong version":          strings.Replace(good, "6.4.1", "6.5.0", 1),
		"refused without code":   strings.Replace(good, "refused\tbad_request", "refused\t", 1),
	}
	for name, doctored := range cases {
		if doctored == good {
			t.Fatalf("%s: doctoring changed nothing", name)
		}
		if len(censusProblems(doctored)) == 0 {
			t.Errorf("%s: census passed", name)
		}
	}
}

// TestSchemaCensusMatchesCodecs holds each row's status to what the codecs
// do: mapped means the decoder accepts it, refused or n/a means it does not.
func TestSchemaCensusMatchesCodecs(t *testing.T) {
	b, err := os.ReadFile("testdata/upstream_census.tsv")
	if err != nil {
		t.Fatal(err)
	}
	_, rows, _ := parseCensus(string(b))
	codes := map[string]bool{string(CodeStalePrompt): true, string(CodeWrongPlayer): true,
		string(CodeWrongPromptType): true, string(CodeUnknownActionID): true, string(CodeInvalidShape): true}
	mappedPrompts := map[string]bool{}
	for _, r := range rows {
		mapped := r.status == "mapped"
		switch r.class {
		case "prompt":
			_, ok := promptInputCases[r.name]
			if ok != mapped {
				t.Errorf("prompt %s: census says %s, promptInputCases has it: %v", r.name, r.status, ok)
			}
			if mapped {
				mappedPrompts[r.name] = true
			}
		case "errorCode":
			if codes[r.name] != mapped {
				t.Errorf("errorCode %s: census says %s, code constant exists: %v", r.name, r.status, codes[r.name])
			}
		case "engineMessage":
			var m EngineMessage
			_, err := Decode([]byte(`{"kind":"`+r.name+`"}`), &m)
			if (err == nil || !strings.Contains(err.Error(), "unknown kind discriminator")) != mapped {
				t.Errorf("engineMessage %s: census says %s, decode err = %v", r.name, r.status, err)
			}
		case "clientMessage":
			var m ClientMessage
			_, err := Decode([]byte(`{"kind":"`+r.name+`"}`), &m)
			if (err == nil || !strings.Contains(err.Error(), "unknown kind discriminator")) != mapped {
				t.Errorf("clientMessage %s: census says %s, decode err = %v", r.name, r.status, err)
			}
		case "directive":
			var m ClientMessage
			_, err := Decode([]byte(`{"kind":"directive","directive":{"type":"`+r.name+`"}}`), &m)
			if (err == nil) != mapped {
				t.Errorf("directive %s: census says %s, decode err = %v", r.name, r.status, err)
			}
		}
	}
	if len(mappedPrompts) != len(promptInputCases) {
		t.Errorf("census maps %d prompts, promptInputCases registers %d", len(mappedPrompts), len(promptInputCases))
	}
}

// TestDocsManabrewPageNamesOnlyWhatGorgeSends keeps docs/site/api/manabrew.md
// off the engine kinds gorge never emits. It reads the whole paragraph that
// starts "Engine messages are", however the lines are wrapped.
func TestDocsManabrewPageNamesOnlyWhatGorgeSends(t *testing.T) {
	b, err := os.ReadFile("../../../docs/site/api/manabrew.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, para := range strings.Split(string(b), "\n\n") {
		if !strings.HasPrefix(para, "Engine messages are") {
			continue
		}
		para = strings.ReplaceAll(para, "\n", " ")
		if !strings.Contains(para, "gorge sends") {
			t.Errorf("the engine-message paragraph does not say what gorge sends: %s", para)
		}
		for _, bad := range []string{"`stateDelta`,", "`display`,", "`display` and"} {
			if strings.Contains(para, bad) {
				t.Errorf("the engine-message paragraph lists a kind gorge never sends (%s): %s", bad, para)
			}
		}
		return
	}
	t.Error("docs/site/api/manabrew.md has no 'Engine messages are' paragraph")
}
