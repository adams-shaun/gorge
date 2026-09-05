package tsgen

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type inner struct {
	N    int32            `json:"n"`
	Tags map[string]int32 `json:"tags,omitempty"`
	Pair [2]uint32        `json:"pair"`
	Raw  json.RawMessage  `json:"raw"`
	Skip string           `json:"-"`
}

type outer struct {
	ID       uint64  `json:"id"`
	Name     string  `json:"name,omitempty"`
	Flag     bool    `json:"flag"`
	Inner    inner   `json:"inner"`
	Inners   []inner `json:"inners"`
	MaybeN   *int32  `json:"maybe_n"`
	Kind     kindT   `json:"kind"`
	Bytes    []byte  `json:"bytes"`
	Any      any     `json:"any"`
	Optional *inner  `json:"optional,omitempty"`
}

type kindT string

func TestGenerateMatchesFixture(t *testing.T) {
	got, err := Generate(Options{
		Roots:  []reflect.Type{reflect.TypeOf(outer{})},
		Unions: map[string][]string{"Kind": {"a", "b"}},
		Header: "// test header\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/fixture.ts")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("generated:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateRejectsTwoStructsWithOneName(t *testing.T) {
	_, err := Generate(Options{Roots: []reflect.Type{reflect.TypeOf(outer{}), reflect.TypeOf(struct{ X int }{})}})
	if err == nil {
		t.Fatal("anonymous struct accepted")
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	o := Options{Roots: []reflect.Type{reflect.TypeOf(outer{})}, Unions: map[string][]string{"Z": {"z"}, "A": {"a"}}}
	a, _ := Generate(o)
	b, _ := Generate(o)
	if a != b {
		t.Fatal("two runs differ")
	}
	if strings.Index(a, "export type A") > strings.Index(a, "export type Z") {
		t.Fatal("unions are not emitted in sorted order")
	}
}
