package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/internal/azmcts"
)

// The object form of a policy spec, for a config file (or a -a/-b value
// starting with '{'):
//
//	{"kind": "az", "sims": 200, "autopay": true, "topk": 6, "gonet": "model.dzgw"}
//
// Its keys are the spec string's keys. A flag key (autopay, mull, ...) is a
// JSON bool (false is the same as leaving it out); sims, worlds, exploreturns,
// temp, second, aseed, cpuct, fpu, cands and topk are numbers; net, gonet,
// remote, leaf and prior are strings; kinds is an array of kind names;
// skippass is true (1) or a number. An unknown key or a value of the wrong
// type is an error. Unlike the string form, a path value may contain ':'
// (gonet=, net=), and mull with keep7, or morekinds with kinds, is an error
// rather than "the last one wins".
//
// Both forms set every key through policySpec.setKey, so an object and the
// string it renders to (canonicalSpec) parse to the identical struct. The
// spec's Raw, recorded in games.jsonl's pols and the run summary, is that
// canonical string: kind, then the keys in policyKeys order.

// policyKeyType is a key's JSON type in the object form.
type policyKeyType int

const (
	pkFlag   policyKeyType = iota // bool: true sets the flag key
	pkNumber                      // a JSON number, passed to setKey as written
	pkString
	pkKinds    // an array of kind names
	pkSkipPass // true or a number
)

// policyKeys are the object form's keys (besides kind) in canonical order:
// the order canonicalSpec writes them in. remote is last because in the
// string form it takes the rest of the spec.
var policyKeys = []struct {
	key string
	typ policyKeyType
}{
	{"sims", pkNumber}, {"net", pkString}, {"gonet", pkString}, {"leaf", pkString}, {"prior", pkString},
	{"worlds", pkNumber}, {"explore", pkFlag}, {"nonoise", pkFlag}, {"exploreturns", pkNumber},
	{"temp", pkNumber}, {"second", pkNumber}, {"aseed", pkNumber}, {"cpuct", pkNumber}, {"fpu", pkNumber},
	{"cands", pkNumber}, {"autopay", pkFlag}, {"mull", pkFlag}, {"keep7", pkFlag}, {"topk", pkNumber},
	{"oppnodes", pkFlag}, {"oppfull", pkFlag}, {"skippass", pkSkipPass}, {"morekinds", pkFlag},
	{"kinds", pkKinds}, {"closedlist", pkFlag}, {"remote", pkString},
}

// jsonType names a decoded JSON value's type for an error.
func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a bool"
	case json.Number:
		return "a number"
	case string:
		return "a string"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	}
	return fmt.Sprintf("%T", v)
}

// policyFromObject reads the object form (decoded with UseNumber): its keys
// are type-checked, rendered to the canonical spec string, and applied with
// setKey; check runs too. The network is not loaded (load).
func policyFromObject(m map[string]any) (*policySpec, error) {
	kind, ok := m["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("policy object: kind: want a string (random, bot, sbtactical, az or prior), got %s", jsonType(m["kind"]))
	}
	known := map[string]policyKeyType{}
	for _, k := range policyKeys {
		known[k.key] = k.typ
	}
	var unknown []string
	for k := range m {
		if _, ok := known[k]; !ok && k != "kind" {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("policy object: unknown key %q", unknown[0])
	}
	type kv struct{ k, v string }
	var kvs []kv
	for _, pk := range policyKeys {
		x, present := m[pk.key]
		if !present {
			continue
		}
		bad := func(want string) error {
			return fmt.Errorf("policy object: %s: want %s, got %s", pk.key, want, jsonType(x))
		}
		switch pk.typ {
		case pkFlag:
			b, ok := x.(bool)
			if !ok {
				return nil, bad("a bool")
			}
			if b {
				kvs = append(kvs, kv{pk.key, ""})
			}
		case pkNumber:
			n, ok := x.(json.Number)
			if !ok {
				return nil, bad("a number")
			}
			kvs = append(kvs, kv{pk.key, string(n)})
		case pkString:
			s, ok := x.(string)
			if !ok {
				return nil, bad("a string")
			}
			kvs = append(kvs, kv{pk.key, s})
		case pkKinds:
			arr, ok := x.([]any)
			if !ok {
				return nil, bad("an array of kind names")
			}
			names := make([]string, len(arr))
			for i, e := range arr {
				if names[i], ok = e.(string); !ok {
					return nil, fmt.Errorf("policy object: kinds[%d]: want a string, got %s", i, jsonType(e))
				}
			}
			kvs = append(kvs, kv{pk.key, strings.Join(names, ",")})
		case pkSkipPass:
			switch v := x.(type) {
			case bool:
				if v {
					kvs = append(kvs, kv{pk.key, ""})
				}
			case json.Number:
				kvs = append(kvs, kv{pk.key, string(v)})
			default:
				return nil, bad("true or a number")
			}
		}
	}
	if m["mull"] == true && m["keep7"] == true {
		return nil, fmt.Errorf("policy object: mull and keep7 are exclusive")
	}
	_, hasKinds := m["kinds"]
	if m["morekinds"] == true && hasKinds {
		return nil, fmt.Errorf("policy object: morekinds and kinds are exclusive")
	}
	raw := kind
	for _, e := range kvs {
		raw += ":" + e.k
		if e.v != "" || !flagKeys[e.k] && e.k != "skippass" {
			raw += "=" + e.v
		}
	}
	p, err := newPolicy(raw, kind)
	if err != nil {
		return nil, err
	}
	for _, e := range kvs {
		if err := p.setKey(raw, e.k, e.v); err != nil {
			return nil, err
		}
	}
	if err := p.check(raw); err != nil {
		return nil, err
	}
	return p, nil
}

// kindNames are the searched kinds' names, in ParseKinds' order.
func kindNames(k azmcts.Kinds) []string {
	var out []string
	for _, e := range []struct {
		on   bool
		name string
	}{
		{k.Priority, "priority"}, {k.Attackers, "attackers"}, {k.Blockers, "blockers"}, {k.Target, "target"},
		{k.Modes, "modes"}, {k.Optional, "optional"}, {k.Choose, "choose"}, {k.MultiTarget, "multitarget"},
	} {
		if e.on {
			out = append(out, e.name)
		}
	}
	return out
}

// policyObjectOf is a parsed spec's object form: kind and every key not at
// its default. policyFromObject reads it back to the same struct (Raw aside).
func policyObjectOf(p *policySpec) map[string]any {
	m := map[string]any{"kind": p.Kind}
	set := func(k string, on bool, v any) {
		if on {
			m[k] = v
		}
	}
	set("sims", p.Sims != azmcts.DefaultOptions().Sims, p.Sims)
	set("net", p.Net != "", p.Net)
	set("gonet", p.GoNet != "", p.GoNet)
	set("remote", p.Remote != "", p.Remote)
	set("leaf", p.HeurLeaf, "heuristic")
	set("prior", p.UniPrior, "uniform")
	set("worlds", p.Worlds != 0, p.Worlds)
	set("explore", p.Explore, true)
	set("nonoise", p.NoNoise, true)
	set("exploreturns", p.ExploreTurns != 0, p.ExploreTurns)
	set("temp", p.Temp != 0, p.Temp)
	set("second", p.Second != 0, p.Second)
	set("aseed", p.ASeed != 0, p.ASeed)
	set("cpuct", p.CPUCT != -1, p.CPUCT)
	set("fpu", p.FPU != -1, p.FPU)
	set("cands", p.Cands != -1, p.Cands)
	set("autopay", p.AutoPay, true)
	set("mull", p.Mull == botpolicy.MulliganLands, true)
	set("keep7", p.Mull == botpolicy.MulliganNever, true)
	set("topk", p.TopK != 0, p.TopK)
	set("oppnodes", p.OppNodes, true)
	set("oppfull", p.OppFull, true)
	set("closedlist", p.Closed, true)
	set("skippass", p.SkipPass != 0, p.SkipPass)
	set("kinds", p.Kinds != (azmcts.Kinds{}), kindNames(p.Kinds))
	return m
}

// policyFlag is play's -a and -b: a spec string, or the object form (from a
// config file, or a flag or environment value starting with '{'). Set checks
// the keys; resolve loads the network.
type policyFlag struct {
	text string         // the string form as given
	obj  map[string]any // the object form as given, or nil
	spec *policySpec    // lexed and checked
}

func newPolicyFlag(def string) *policyFlag {
	f := &policyFlag{}
	if err := f.Set(def); err != nil {
		panic(err)
	}
	return f
}

func (f *policyFlag) Type() string { return "policy" }

// String is the spec's Raw: the string as given, or the object's canonical string.
func (f *policyFlag) String() string {
	if f.spec == nil {
		return f.text
	}
	return f.spec.Raw
}

func (f *policyFlag) Set(s string) error {
	if strings.HasPrefix(strings.TrimSpace(s), "{") {
		return f.SetJSON(json.RawMessage(s))
	}
	p, err := lexPolicy(s)
	if err != nil {
		return err
	}
	*f = policyFlag{text: s, spec: p}
	return nil
}

func (f *policyFlag) SetJSON(raw json.RawMessage) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		return fmt.Errorf("policy object: %v", err)
	}
	switch v := x.(type) {
	case string:
		return f.Set(v)
	case map[string]any:
		p, err := policyFromObject(v)
		if err != nil {
			return err
		}
		*f = policyFlag{obj: v, spec: p}
		return nil
	}
	return fmt.Errorf("want a policy spec string or object, got %s", jsonType(x))
}

// JSON is the form it was given in.
func (f *policyFlag) JSON() any {
	if f.obj != nil {
		return f.obj
	}
	return f.text
}

// resolve is the loaded spec (a fresh copy: each call loads its own network).
func (f *policyFlag) resolve() (*policySpec, error) {
	p := *f.spec
	if err := p.load(p.Raw); err != nil {
		return nil, err
	}
	return &p, nil
}
