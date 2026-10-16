package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// acceptedSpecs are the accepted specs of the dzgorge-spec golden.
func acceptedSpecs(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(goldenDir, "spec_grammar.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var line struct {
			Spec     string          `json:"spec"`
			Parsed   json.RawMessage `json:"parsed"`
			Rejected bool            `json:"rejected"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			t.Fatal(err)
		}
		if !line.Rejected {
			out = append(out, line.Spec)
		}
	}
	if len(out) < 10 {
		t.Fatalf("only %d accepted specs in the golden", len(out))
	}
	return out
}

// objectSpec reads the object form from JSON text through the flag value,
// as a config file does, and loads it.
func objectSpec(t *testing.T, raw string) (*policySpec, error) {
	t.Helper()
	var f policyFlag
	if err := f.SetJSON(json.RawMessage(raw)); err != nil {
		return nil, err
	}
	return f.resolve()
}

// TestPolicyObjectRoundTrip: every accepted spec of the golden, converted to
// the object form and back, is the identical struct; the object's canonical
// string (its Raw) parses to the identical struct too, Raw included.
func TestPolicyObjectRoundTrip(t *testing.T) {
	for _, s := range acceptedSpecs(t) {
		p, err := parsePolicy(s)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		obj := policyObjectOf(p)
		raw, err := json.Marshal(obj)
		if err != nil {
			t.Fatal(err)
		}
		q, err := objectSpec(t, string(raw))
		if err != nil {
			t.Fatalf("%q -> %s: %v", s, raw, err)
		}
		canon := q.Raw
		q.Raw = p.Raw
		if !reflect.DeepEqual(p, q) {
			t.Errorf("%q -> %s:\n string %+v\n object %+v", s, raw, *p, *q)
		}
		q.Raw = canon
		r, err := parsePolicy(canon)
		if err != nil {
			t.Fatalf("canonical %q of %q: %v", canon, s, err)
		}
		if !reflect.DeepEqual(r, q) {
			t.Errorf("canonical %q:\n string %+v\n object %+v", canon, *r, *q)
		}
		// The flag's string form of the object is that canonical string.
		var f policyFlag
		if err := f.SetJSON(raw); err != nil || f.String() != canon {
			t.Errorf("%s: flag String %q, want %q (%v)", raw, f.String(), canon, err)
		}
	}
}

func TestPolicyObjectCanonical(t *testing.T) {
	for _, tc := range []struct{ obj, raw string }{
		{`{"kind":"bot"}`, "bot"},
		{`{"kind":"az","topk":6,"sims":200,"autopay":true,"oppfull":true,"oppnodes":true,"morekinds":true}`,
			"az:sims=200:autopay:topk=6:oppnodes:oppfull:morekinds"},
		{`{"kind":"az","sims":5,"autopay":false,"skippass":true}`, "az:sims=5:skippass"},
		{`{"kind":"az","sims":5,"skippass":2,"kinds":["priority","target"]}`, "az:sims=5:skippass=2:kinds=priority,target"},
		{`{"kind":"az","sims":5,"remote":"unix:/tmp/dzg.sock","mull":true}`, "az:sims=5:mull:remote=unix:/tmp/dzg.sock"},
	} {
		var f policyFlag
		if err := f.SetJSON(json.RawMessage(tc.obj)); err != nil {
			t.Fatalf("%s: %v", tc.obj, err)
		}
		if f.String() != tc.raw {
			t.Errorf("%s: Raw %q, want %q", tc.obj, f.String(), tc.raw)
		}
		// The flag-value spelling of the object ('{' first) is the same.
		var g policyFlag
		if err := g.Set(" " + tc.obj); err != nil || g.String() != tc.raw {
			t.Errorf("Set(%s): %q %v", tc.obj, g.String(), err)
		}
	}
}

func TestPolicyObjectRejects(t *testing.T) {
	for _, tc := range []struct{ obj, msg string }{
		{`{"sims":5}`, "kind: want a string"},
		{`{"kind":3}`, "kind: want a string"},
		{`{"kind":"bogus"}`, "want random, bot, sbtactical, az or prior"},
		{`{"kind":"az","simz":5}`, `unknown key "simz"`},
		{`{"kind":"az","Sims":5}`, `unknown key "Sims"`},
		{`{"kind":"az","sims":"200"}`, "sims: want a number, got a string"},
		{`{"kind":"az","sims":1.5}`, "sims"},
		{`{"kind":"az","autopay":"yes"}`, "autopay: want a bool, got a string"},
		{`{"kind":"az","autopay":1}`, "autopay: want a bool, got a number"},
		{`{"kind":"az","gonet":1}`, "gonet: want a string, got a number"},
		{`{"kind":"az","kinds":"priority"}`, "kinds: want an array of kind names"},
		{`{"kind":"az","kinds":["priority",2]}`, "kinds[1]: want a string"},
		{`{"kind":"az","kinds":["nope"]}`, "unknown searched kind"},
		{`{"kind":"az","skippass":"x"}`, "skippass: want true or a number"},
		{`{"kind":"az","temp":0}`, "temp: want > 0"},
		{`{"kind":"az","leaf":"none"}`, "leaf=none"},
		{`{"kind":"bot","temp":0.5}`, "are az keys"},
		{`{"kind":"bot","closedlist":true}`, "closedlist is an az key"},
		{`{"kind":"prior"}`, "prior needs net="},
		{`{"kind":"az","net":"a.gpol","gonet":"b.dzgw"}`, "exclusive"},
		{`{"kind":"az","mull":true,"keep7":true}`, "mull and keep7 are exclusive"},
		{`{"kind":"az","morekinds":true,"kinds":["priority"]}`, "morekinds and kinds are exclusive"},
		{`["az"]`, "want a policy spec string or object, got an array"},
		{`3`, "want a policy spec string or object, got a number"},
	} {
		var f policyFlag
		err := f.SetJSON(json.RawMessage(tc.obj))
		if err == nil || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%s: err %v, want %q", tc.obj, err, tc.msg)
		}
	}
}

// TestPolicyObjectColonPath: the object form carries a path with ':', which
// the string form cannot (its keys are ':'-separated).
func TestPolicyObjectColonPath(t *testing.T) {
	var f policyFlag
	if err := f.SetJSON(json.RawMessage(`{"kind":"az","sims":5,"gonet":"C:/models/c5:best.dzgw"}`)); err != nil {
		t.Fatal(err)
	}
	if f.spec.GoNet != "C:/models/c5:best.dzgw" {
		t.Fatalf("gonet %q", f.spec.GoNet)
	}
	if _, err := lexPolicy("az:sims=5:gonet=C:/models/c5:best.dzgw"); err == nil {
		t.Fatal("the string form accepted a colon in a path")
	}
}

// TestPolicyKeysCoverGrammar: the object form's flag keys are exactly the
// string form's flag keys, and every key the string form reads has an
// object key.
func TestPolicyKeysCoverGrammar(t *testing.T) {
	var objFlags, want []string
	seen := map[string]bool{}
	for _, k := range policyKeys {
		if seen[k.key] {
			t.Fatalf("policyKeys lists %q twice", k.key)
		}
		seen[k.key] = true
		if k.typ == pkFlag {
			objFlags = append(objFlags, k.key)
		}
	}
	for k := range flagKeys {
		want = append(want, k)
	}
	sort.Strings(objFlags)
	sort.Strings(want)
	if !reflect.DeepEqual(objFlags, want) {
		t.Fatalf("object flag keys %v, string flag keys %v", objFlags, want)
	}
	for _, k := range policyKeys {
		p, _ := newPolicy("az", "az")
		v := "1"
		switch k.key {
		case "leaf":
			v = "heuristic"
		case "prior":
			v = "uniform"
		case "kinds":
			v = "priority"
		}
		if err := p.setKey("az", k.key, v); err != nil && strings.Contains(err.Error(), "unknown key") {
			t.Errorf("setKey does not know object key %q", k.key)
		}
	}
}
