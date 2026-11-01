package extern

import (
	"errors"
	"testing"
)

// TestParseSpec walks the spec grammar: both transports, with and without
// a model version, and every rejection.
func TestParseSpec(t *testing.T) {
	cases := []struct {
		name   string
		spec   string
		ep     Endpoint
		ver    string
		enough bool // true = must parse
	}{
		{"exec plain", "ext:exec:/usr/local/bin/myagent", Endpoint{"exec", "/usr/local/bin/myagent"}, "", true},
		{"exec versioned", "ext:exec:/usr/local/bin/myagent@v2", Endpoint{"exec", "/usr/local/bin/myagent"}, "v2", true},
		{"unix plain", "ext:unix:/tmp/agents/a.sock", Endpoint{"unix", "/tmp/agents/a.sock"}, "", true},
		{"unix versioned", "ext:unix:/tmp/agents/a.sock@7", Endpoint{"unix", "/tmp/agents/a.sock"}, "7", true},
		{"empty target", "ext:exec:", Endpoint{}, "", false},
		{"bad transport", "ext:tcp:/tmp/a.sock", Endpoint{}, "", false},
		{"two @", "ext:exec:/bin/x@a@b", Endpoint{}, "", false},
		{"missing ext:", "exec:/bin/x", Endpoint{}, "", false},
		{"empty version", "ext:exec:/bin/x@", Endpoint{}, "", false},
		{"no colon", "ext:unix", Endpoint{}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep, ver, err := ParseSpec(tc.spec)
			if !tc.enough {
				if err == nil {
					t.Fatalf("ParseSpec(%q) = %+v, %q, want an ErrSpec error", tc.spec, ep, ver)
				}
				if !errors.Is(err, ErrSpec) {
					t.Fatalf("ParseSpec(%q) error %v does not wrap ErrSpec", tc.spec, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSpec(%q): %v", tc.spec, err)
			}
			if ep != tc.ep || ver != tc.ver {
				t.Fatalf("ParseSpec(%q) = %+v, %q, want %+v, %q", tc.spec, ep, ver, tc.ep, tc.ver)
			}
			if !IsSpec(tc.spec) {
				t.Fatalf("IsSpec(%q) = false", tc.spec)
			}
		})
	}
	if IsSpec("exec:/bin/x") {
		t.Fatal(`IsSpec("exec:/bin/x") = true`)
	}
}
