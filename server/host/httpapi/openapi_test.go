package httpapi

// The published HTTP reference (docs/site/api/openapi.json and its YAML
// render, openapi.yaml) is guarded here so it cannot silently rot:
//
//   - TestOpenAPIRoutesMatchSpec: the (method, path) pairs registered on
//     httpapi's mux and on gorged's top mux equal the spec's operations, in
//     both directions, read from SOURCE (every method-qualified pattern
//     literal handed to HandleFunc/Handle); and every spec operation httpapi
//     owns is really served by a fully armed handler (not the catch-all 404,
//     not a 405).
//   - TestOpenAPISchemasMatchGo: every component carrying x-go-type equals
//     the schema internal/tsgen derives from that Go type today, and the
//     decision-kind enum lists exactly decision.Kinds.
//   - TestOpenAPIYAMLIsRendered: openapi.yaml is the render of openapi.json.
//   - TestOpenAPIProtocolSchemaIsRendered: schemas/gorge-protocol.schema.json
//     (the same components as one standalone JSON Schema document, for
//     clients of the stream that need no OpenAPI tooling) is their render.
//
// After changing a wire type or a route, regenerate the derived parts with
//
//	go test ./server/host/httpapi -run 'TestOpenAPI' -update
//
// which rewrites the generated components and openapi.yaml in place (the
// hand-written paths, descriptions and examples are kept), then document any
// new route by hand in openapi.json.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/internal/tsgen"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

var update = flag.Bool("update", false, "rewrite docs/site/api/openapi.json's generated components and openapi.yaml")

const (
	specJSON = "docs/site/api/openapi.json"
	specYAML = "docs/site/api/openapi.yaml"
	// protocolSchema is the stream's wire types as plain JSON Schema.
	protocolSchema = "docs/site/api/schemas/gorge-protocol.schema.json"
	refPre         = "#/components/schemas/"
)

// routeDirs are the packages whose source registers gorged's routes, and
// whether httpapi's own handler serves them (the runtime probe can build that
// handler; gorged's top mux needs a whole server).
var routeDirs = []struct {
	dir     string
	httpapi bool
}{{"server/host/httpapi", true}, {"server/gorged", false}}

// moduleRoot walks up from the test's directory to go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

// loadSpec decodes openapi.json with numbers kept exact.
func loadSpec(t *testing.T, root string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, specJSON))
	if err != nil {
		t.Fatal(err)
	}
	return decodeExact(t, raw).(map[string]any)
}

func decodeExact(t *testing.T, raw []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

// normalise round-trips v through JSON so Go values compare equal to decoded ones.
func normalise(t *testing.T, v any) any {
	t.Helper()
	raw, err := marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return decodeExact(t, raw)
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return b.Bytes(), err
}

// route is one method-qualified pattern, as an OpenAPI (method, path).
type route struct{ method, path string }

func (r route) String() string { return r.method + " " + r.path }

// sourceRoutes reads every method-qualified pattern literal registered in
// dir's non-test files. A 405 twin (handler methodNotAllowed) is not a route.
func sourceRoutes(t *testing.T, dir string) []route {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []route
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if id, ok := call.Args[1].(*ast.Ident); ok && id.Name == "methodNotAllowed" {
				return true
			}
			pat, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			method, path, ok := strings.Cut(pat, " ")
			if !ok {
				return true // a method-less catch-all ("/", "/api/"): not an operation
			}
			out = append(out, route{strings.ToLower(method), strings.ReplaceAll(path, "...}", "}")})
			return true
		})
	}
	return out
}

var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

func specRoutes(spec map[string]any) []route {
	var out []route
	paths, _ := spec["paths"].(map[string]any)
	for p, item := range paths {
		ops, _ := item.(map[string]any)
		for _, m := range httpMethods {
			if _, ok := ops[m]; ok {
				out = append(out, route{m, p})
			}
		}
	}
	return out
}

func routeSet(rs []route) map[route]bool {
	m := map[route]bool{}
	for _, r := range rs {
		m[r] = true
	}
	return m
}

func sortedDiff(a, b map[route]bool) []string {
	var out []string
	for r := range a {
		if !b[r] {
			out = append(out, r.String())
		}
	}
	sort.Strings(out)
	return out
}

func TestOpenAPIRoutesMatchSpec(t *testing.T) {
	root := moduleRoot(t)
	spec := loadSpec(t, root)
	var code []route
	var served []route
	for _, d := range routeDirs {
		rs := sourceRoutes(t, filepath.Join(root, d.dir))
		code = append(code, rs...)
		if d.httpapi {
			served = append(served, rs...)
		}
	}
	codeSet, specSet := routeSet(code), routeSet(specRoutes(spec))
	if missing := sortedDiff(codeSet, specSet); len(missing) > 0 {
		t.Errorf("routes registered in code but absent from %s (document them):\n  %s", specJSON, strings.Join(missing, "\n  "))
	}
	if stale := sortedDiff(specSet, codeSet); len(stale) > 0 {
		t.Errorf("operations in %s that no code registers (remove them):\n  %s", specJSON, strings.Join(stale, "\n  "))
	}
	probeServed(t, served)
}

// probeServed asks a fully armed handler for each httpapi route and checks
// it is really routed: neither the /api/ catch-all's 404 nor a 405 twin.
func probeServed(t *testing.T, rs []route) {
	t.Helper()
	reg, err := host.New(host.Options{
		LoadDeck: func(string) (host.Deck, error) { return host.Deck{}, errors.New("no decks in the route probe") },
		Sleep:    func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	h := NewHandler(reg, Options{
		Seat:       func(*http.Request) (SeatClaim, bool) { return SeatClaim{Table: "t1"}, true },
		CreateGame: func(CreateGameOptions) (CreateGameResponse, error) { return CreateGameResponse{}, errors.New("probe") },
		Events:     stubEvents{},
		Decisions:  func(host.TableID, int) ([]protocol.ReviewDecision, error) { return nil, nil },
	})
	fill := strings.NewReplacer("{t}", "t1", "{k}", "1", "{id}", "e1")
	for _, r := range rs {
		ctx, cancel := context.WithCancel(context.Background())
		if r.path == "/api/stream" {
			cancel() // the stream returns once its hello is written
		}
		req := httptest.NewRequest(strings.ToUpper(r.method), fill.Replace(r.path), strings.NewReader("{}")).WithContext(ctx)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		cancel()
		var body protocol.ErrorBody
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code == http.StatusMethodNotAllowed || (rec.Code == http.StatusNotFound && body.Message == "no such endpoint") {
			t.Errorf("%s is in the source but the armed handler does not route it: %d %s", r, rec.Code, rec.Body.String())
		}
	}
}

// stubEvents arms /api/events for the route probe.
type stubEvents struct{}

func (stubEvents) CreateEvent(protocol.CreateEventRequest) (protocol.CreateEventResponse, error) {
	return protocol.CreateEventResponse{}, nil
}
func (stubEvents) Events() []protocol.EventState { return nil }
func (stubEvents) EventState(string, string) (protocol.EventState, error) {
	return protocol.EventState{}, nil
}
func (stubEvents) JoinEvent(string, string, protocol.JoinEventRequest) (protocol.EventState, error) {
	return protocol.EventState{}, nil
}
func (stubEvents) DraftPack(string, string) (protocol.DraftPack, error) {
	return protocol.DraftPack{}, nil
}
func (stubEvents) Pick(string, string, protocol.PickRequest) (protocol.DraftPack, error) {
	return protocol.DraftPack{}, nil
}
func (stubEvents) EventPool(string, string) (protocol.EventPool, error) {
	return protocol.EventPool{}, nil
}
func (stubEvents) CheckDeck(string, string, protocol.DeckRequest) (protocol.DeckCheck, error) {
	return protocol.DeckCheck{}, nil
}
func (stubEvents) SaveDeck(string, string, protocol.DeckRequest) (protocol.DeckCheck, error) {
	return protocol.DeckCheck{}, nil
}
func (stubEvents) Ready(string, string, protocol.ReadyRequest) (protocol.EventState, error) {
	return protocol.EventState{}, nil
}
func (stubEvents) Sideboard(string, string, protocol.SideboardRequest) (protocol.EventState, error) {
	return protocol.EventState{}, nil
}
func (stubEvents) EventLog(string, string) (protocol.EventLog, error) {
	return protocol.EventLog{}, nil
}
func (stubEvents) SubscribeEvent(string, string, string) error { return nil }
func (stubEvents) Sets() []protocol.EventSetInfo               { return nil }
func (stubEvents) Personas() protocol.EventPersonas            { return protocol.EventPersonas{} }

// schemaRoots are the Go types the spec's generated components come from:
// every body httpapi and gorged marshal or decode, and everything they reach.
var schemaRoots = []struct{ pkg, name string }{
	{"github.com/adams-shaun/gorge/server/protocol", "Frame"},
	{"github.com/adams-shaun/gorge/server/protocol", "FrameType"},
	{"github.com/adams-shaun/gorge/server/protocol", "Hello"},
	{"github.com/adams-shaun/gorge/server/protocol", "Widget"},
	{"github.com/adams-shaun/gorge/server/protocol", "MatchStart"},
	{"github.com/adams-shaun/gorge/server/protocol", "Snapshot"},
	{"github.com/adams-shaun/gorge/server/protocol", "EventBody"},
	{"github.com/adams-shaun/gorge/server/protocol", "DecisionBody"},
	{"github.com/adams-shaun/gorge/server/protocol", "MatchEnd"},
	{"github.com/adams-shaun/gorge/server/protocol", "TableHaltedBody"},
	{"github.com/adams-shaun/gorge/server/protocol", "TableStateBody"},
	{"github.com/adams-shaun/gorge/server/protocol", "Overflow"},
	{"github.com/adams-shaun/gorge/server/protocol", "ErrorBody"},
	{"github.com/adams-shaun/gorge/server/protocol", "MatchInfo"},
	{"github.com/adams-shaun/gorge/server/protocol", "Subscribe"},
	{"github.com/adams-shaun/gorge/server/protocol", "Unsubscribe"},
	{"github.com/adams-shaun/gorge/server/protocol", "BotPolicyList"},
	{"github.com/adams-shaun/gorge/server/protocol", "Capabilities"},
	{"github.com/adams-shaun/gorge/server/protocol", "DeckTally"},
	{"github.com/adams-shaun/gorge/server/protocol", "EventState"},
	{"github.com/adams-shaun/gorge/server/protocol", "DraftPack"},
	{"github.com/adams-shaun/gorge/server/protocol", "PodProgress"},
	{"github.com/adams-shaun/gorge/server/protocol", "SeriesUpdate"},
	{"github.com/adams-shaun/gorge/server/protocol", "CreateEventRequest"},
	{"github.com/adams-shaun/gorge/server/protocol", "CreateEventResponse"},
	{"github.com/adams-shaun/gorge/server/protocol", "JoinEventRequest"},
	{"github.com/adams-shaun/gorge/server/protocol", "PickRequest"},
	{"github.com/adams-shaun/gorge/server/protocol", "DeckRequest"},
	{"github.com/adams-shaun/gorge/server/protocol", "DeckCheck"},
	{"github.com/adams-shaun/gorge/server/protocol", "EventPool"},
	{"github.com/adams-shaun/gorge/server/protocol", "ReadyRequest"},
	{"github.com/adams-shaun/gorge/server/protocol", "SideboardRequest"},
	{"github.com/adams-shaun/gorge/server/protocol", "EventLog"},
	{"github.com/adams-shaun/gorge/server/protocol", "EventSubscribe"},
	{"github.com/adams-shaun/gorge/server/protocol", "EventSetInfo"},
	{"github.com/adams-shaun/gorge/server/protocol", "EventPersonas"},
	{"github.com/adams-shaun/gorge/server/protocol", "ReviewDecision"},
	{"github.com/adams-shaun/gorge/engine/decision", "Decision"},
	{"github.com/adams-shaun/gorge/engine/decision", "Intent"},
	{"github.com/adams-shaun/gorge/engine/decision", "Kind"},
	{"github.com/adams-shaun/gorge/engine/view", "View"},
	{"github.com/adams-shaun/gorge/server/host/httpapi", "DeckInfo"},
	{"github.com/adams-shaun/gorge/server/host/httpapi", "CreateGameRequest"},
	{"github.com/adams-shaun/gorge/server/host/httpapi", "CreateGameResponse"},
}

// schemaOverrides state the wire form of the reached types that marshal
// themselves.
var schemaOverrides = map[string]map[string]any{
	"github.com/adams-shaun/gorge/engine/view.Visibility": {"type": "string", "enum": []any{"seat", "public", "omniscient"}},
}

// goComponents derives the components from source, normalised to decoded JSON.
func goComponents(t *testing.T) map[string]any {
	t.Helper()
	l := tsgen.NewLoader()
	roots := make([]types.Type, 0, len(schemaRoots))
	for _, r := range schemaRoots {
		ty, err := l.Lookup(r.pkg, r.name)
		if err != nil {
			t.Fatal(err)
		}
		roots = append(roots, ty)
	}
	comps, err := tsgen.JSONSchema(tsgen.SchemaOptions{Roots: roots, RefPrefix: refPre, Overrides: schemaOverrides, Fset: l.FileSet()})
	if err != nil {
		t.Fatal(err)
	}
	return normalise(t, comps).(map[string]any)
}

func TestOpenAPISchemasMatchGo(t *testing.T) {
	root := moduleRoot(t)
	spec := loadSpec(t, root)
	gen := goComponents(t)
	comps, _ := spec["components"].(map[string]any)
	schemas, _ := comps["schemas"].(map[string]any)
	if *update {
		if schemas == nil {
			schemas = map[string]any{}
		}
		for name, s := range schemas {
			if m, ok := s.(map[string]any); ok && m[tsgen.GoTypeKey] != nil {
				if _, still := gen[name]; !still {
					delete(schemas, name)
				}
			}
		}
		for name, s := range gen {
			schemas[name] = s
		}
		if comps == nil {
			comps = map[string]any{}
			spec["components"] = comps
		}
		comps["schemas"] = schemas
		writeSpec(t, root, spec)
		return
	}
	var drift []string
	for name, s := range gen {
		if !reflect.DeepEqual(schemas[name], s) {
			drift = append(drift, name)
		}
	}
	for name, s := range schemas {
		if m, ok := s.(map[string]any); ok && m[tsgen.GoTypeKey] != nil && gen[name] == nil {
			drift = append(drift, name+" (no longer reached from schemaRoots)")
		}
	}
	sort.Strings(drift)
	if len(drift) > 0 {
		t.Errorf("%s's generated components are stale: %s\nrerun: go test ./server/host/httpapi -run TestOpenAPI -update", specJSON, strings.Join(drift, ", "))
	}
	// The closed decision-kind set, checked against the committed spec
	// directly rather than through the generator.
	kind, _ := schemas["Kind"].(map[string]any)
	var enum []string
	for _, v := range asSlice(kind["enum"]) {
		enum = append(enum, fmt.Sprint(v))
	}
	var want []string
	for _, k := range decision.Kinds {
		want = append(want, string(k))
	}
	if !reflect.DeepEqual(enum, want) {
		t.Errorf("components.schemas.Kind.enum = %v, want decision.Kinds %v", enum, want)
	}
}

func asSlice(v any) []any { s, _ := v.([]any); return s }

func TestOpenAPIYAMLIsRendered(t *testing.T) {
	root := moduleRoot(t)
	spec := loadSpec(t, root)
	want := renderYAML(spec)
	if *update {
		if err := os.WriteFile(filepath.Join(root, specYAML), []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(filepath.Join(root, specYAML))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is not the render of %s; rerun: go test ./server/host/httpapi -run TestOpenAPI -update", specYAML, specJSON)
	}
}

// protocolSchemaDoc renders the spec's components as a standalone JSON
// Schema 2020-12 document: every schema under $defs, every $ref rewritten
// from #/components/schemas/ to #/$defs/.
func protocolSchemaDoc(t *testing.T, spec map[string]any) []byte {
	t.Helper()
	comps, _ := spec["components"].(map[string]any)
	doc := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "https://adams-shaun.github.io/gorge/api/schemas/gorge-protocol.schema.json",
		"title":       "gorge host-client wire (server/protocol)",
		"description": "Rendered from openapi.json's components by server/host/httpapi's openapi_test.go; do not edit. Validate one stream frame against #/$defs/StreamFrame.",
		"$defs":       comps["schemas"],
	}
	raw, err := marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(raw, []byte(`"`+refPre), []byte(`"#/$defs/`))
}

func TestOpenAPIProtocolSchemaIsRendered(t *testing.T) {
	root := moduleRoot(t)
	want := protocolSchemaDoc(t, loadSpec(t, root))
	path := filepath.Join(root, protocolSchema)
	if *update {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is not the render of %s; rerun: go test ./server/host/httpapi -run TestOpenAPI -update", protocolSchema, specJSON)
	}
}

func writeSpec(t *testing.T, root string, spec map[string]any) {
	t.Helper()
	raw, err := marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, specJSON), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// renderYAML renders decoded JSON as block YAML: keys sorted, every string
// double-quoted (a JSON string literal is a valid YAML double-quoted scalar),
// numbers, booleans and null as JSON spells them.
func renderYAML(v any) string {
	var b strings.Builder
	b.WriteString("# Rendered from openapi.json by server/host/httpapi's openapi_test.go; do not edit.\n")
	yamlValue(&b, v, 0, false)
	return b.String()
}

func yamlScalar(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "null", true
	case bool:
		return strconv.FormatBool(x), true
	case json.Number:
		return x.String(), true
	case string:
		raw, _ := json.Marshal(x)
		return string(raw), true
	case map[string]any:
		if len(x) == 0 {
			return "{}", true
		}
	case []any:
		if len(x) == 0 {
			return "[]", true
		}
	}
	return "", false
}

func yamlKey(k string) string {
	plain := k != ""
	for i, r := range k {
		if !(r == '_' || r == '-' || r == '.' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0)) {
			plain = false
		}
	}
	if plain && k != "true" && k != "false" && k != "null" && k != "yes" && k != "no" && k != "on" && k != "off" {
		return k
	}
	raw, _ := json.Marshal(k)
	return string(raw)
}

// yamlValue writes v at indent; inSeq means the caller already wrote "- ".
func yamlValue(b *strings.Builder, v any, indent int, inSeq bool) {
	pad := strings.Repeat("  ", indent)
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if !(inSeq && i == 0) {
				b.WriteString(pad)
			}
			b.WriteString(yamlKey(k) + ":")
			if s, ok := yamlScalar(x[k]); ok {
				b.WriteString(" " + s + "\n")
				continue
			}
			b.WriteString("\n")
			yamlValue(b, x[k], indent+1, false)
		}
	case []any:
		for _, e := range x {
			b.WriteString(pad + "- ")
			if s, ok := yamlScalar(e); ok {
				b.WriteString(s + "\n")
				continue
			}
			if _, isMap := e.(map[string]any); isMap {
				yamlValue(b, e, indent+1, true)
				continue
			}
			b.WriteString("\n")
			yamlValue(b, e, indent+1, false)
		}
	default:
		s, _ := yamlScalar(x)
		b.WriteString(pad + s + "\n")
	}
}
