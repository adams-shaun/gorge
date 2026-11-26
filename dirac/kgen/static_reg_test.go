package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// regPanic runs f and reports whether it panicked (the registry's
// duplicate-registration contract).
func regPanic(f func()) (panicked bool) {
	defer func() { panicked = recover() != nil }()
	f()
	return false
}

// TestStaticModeRegistry covers the static build registry (static_reg.go):
// a duplicate registration panics, an arm registered in the map lowers
// through (*cardBuilder).static's dispatch, and both a moved arm (Override)
// and an unmoved arm (BeamGrant) still lower to the same records the switch
// built before the move.
func TestStaticModeRegistry(t *testing.T) {
	// a duplicate of an already-registered real mode panics and the map
	// entry is untouched
	if !regPanic(func() { registerStaticBuild("Override", cbOverrideStatic) }) {
		t.Fatal("duplicate registration of Override did not panic")
	}

	// a fresh registration lowers through the dispatch in static()
	ran := false
	registerStaticBuild("RegTestArm", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		ran = true
		st.Kind = kop.StatBeamGrant
		return []uint8{7}, nil
	})
	t.Cleanup(func() { delete(staticBuilds, "RegTestArm") })
	cb := &cardBuilder{pt: newPoolTables(&lowered{}, nil)}
	ka, add, err := cb.static(lStatic{Kind: "RegTestArm"})
	if err != nil {
		t.Fatalf("registered arm static: %v", err)
	}
	if !ran {
		t.Fatal("registered arm body never ran: static() did not dispatch through staticBuilds")
	}
	if ka.Static.Kind != kop.StatBeamGrant {
		t.Fatalf("registered arm static kind %d, want StatBeamGrant", ka.Static.Kind)
	}
	if len(add) != 1 || add[0] != 7 {
		t.Fatalf("registered arm add %v, want [7]", add)
	}

	// the moved Override arm lowers like the switch arm it replaced: Kind,
	// the battlefield filter, the override record and AddChosen
	mov := &cardBuilder{pt: newPoolTables(&lowered{}, nil)}
	f, err := parseFilter("Creature", func(string) bool { return false }, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	ova, _, err := mov.static(lStatic{Kind: "Override", F: f, Override: &lOverride{}, AddChosen: true})
	if err != nil {
		t.Fatalf("Override static: %v", err)
	}
	if ova.Static.Kind != kop.StatOverride {
		t.Fatalf("Override static kind %d, want StatOverride", ova.Static.Kind)
	}
	if ova.Static.Filter == 0 {
		t.Fatal("Override static lowered no filter")
	}
	if ova.Static.Override == 0 {
		t.Fatal("Override static lowered no override record")
	}
	if !ova.Static.AddChosen {
		t.Fatal("Override static lost AddChosen")
	}

	// the unmoved BeamGrant arm still lowers through the switch
	unmov := &cardBuilder{pt: newPoolTables(&lowered{}, nil)}
	beama, _, err := unmov.static(lStatic{Kind: "BeamGrant"})
	if err != nil {
		t.Fatalf("BeamGrant static: %v", err)
	}
	if beama.Static.Kind != kop.StatBeamGrant {
		t.Fatalf("BeamGrant static kind %d, want StatBeamGrant", beama.Static.Kind)
	}

	// the other two moved arms lower to their kinds
	for _, tc := range []struct {
		kind string
		want kop.StaticKind
	}{
		{"FlipCoinMod", kop.StatFlipCoinMod},
		{"ActivationLimit", kop.StatActivationLimit},
	} {
		ka, _, err := unmov.static(lStatic{Kind: tc.kind})
		if err != nil {
			t.Fatalf("%s static: %v", tc.kind, err)
		}
		if ka.Static.Kind != tc.want {
			t.Fatalf("%s static kind %d, want %d", tc.kind, ka.Static.Kind, tc.want)
		}
	}
}
