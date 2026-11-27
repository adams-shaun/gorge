package encoder_test

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/encoder"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// pinnedHashes are the encoder digests every shipped net and checkpoint
// carries, measured before the registry existed. They never change: a moved
// value invalidates every net trained under that set.
var pinnedHashes = map[string]uint64{
	"entity":     0x9eff0a5fec167119,
	"entity-pod": 0x3ae74ef8f511ce19,
	"mz":         0xa79b26e026c5541f,
	"mz-grave":   0x2d033c319e0aad2c,
	"mz-opphand": 0x6d87f27623496124,
	"mz-oracle":  0xb72b08eb4016895e,
	"mz-ownlib":  0x74f6f973c0a86be7,
	"v1":         0x3d2e6eb7d427bcc3,
}

// TestRegisteredEncoderHashesUnchanged: every registered hash equals the
// pinned value and policynet.EncoderHashFor of its FeatureSet.
func TestRegisteredEncoderHashesUnchanged(t *testing.T) {
	list := encoder.List()
	if len(list) != len(pinnedHashes) {
		t.Fatalf("registered %d encoders, pinned %d", len(list), len(pinnedHashes))
	}
	for _, e := range list {
		s := e.Schema()
		want, ok := pinnedHashes[s.Name]
		if !ok {
			t.Errorf("encoder %q is registered but not pinned", s.Name)
			continue
		}
		if got := s.Hash(); got != want {
			t.Errorf("%s: Hash %#016x, pinned %#016x", s.Name, got, want)
		}
		fs, err := policynet.ParseFeatureSet(s.Name)
		if err != nil {
			t.Errorf("%s: %v", s.Name, err)
			continue
		}
		if got := policynet.EncoderHashFor(fs); got != s.Hash() {
			t.Errorf("%s: Hash %#016x, EncoderHashFor %#016x", s.Name, s.Hash(), got)
		}
		if s.Hidden != fs.Diagnostic() {
			t.Errorf("%s: Hidden %v, Diagnostic %v", s.Name, s.Hidden, fs.Diagnostic())
		}
	}
}

func TestListIsSortedAndComplete(t *testing.T) {
	var names []string
	for _, e := range encoder.List() {
		names = append(names, e.Schema().Name)
	}
	if !slices.IsSorted(names) {
		t.Errorf("List not sorted: %v", names)
	}
	for fs := policynet.FeaturesV1; fs <= policynet.FeaturesPod; fs++ {
		if _, ok := encoder.Lookup(fs.String()); !ok {
			t.Errorf("feature set %s has no registered encoder", fs)
		}
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("duplicate Register did not panic")
		}
	}()
	encoder.Register(encoder.ForFeatureSet(policynet.FeaturesEntity))
}

func TestResolve(t *testing.T) {
	ent := policynet.EncoderHashFor(policynet.FeaturesEntity)
	if _, err := encoder.Resolve("entity", ent, true); err != nil {
		t.Errorf("entity: %v", err)
	}
	if _, err := encoder.Resolve("entity", ent^1, true); err == nil {
		t.Error("hash mismatch accepted")
	}
	if _, err := encoder.Resolve("nope", ent, true); err == nil {
		t.Error("unknown encoder accepted")
	}
	or := policynet.EncoderHashFor(policynet.FeaturesMZOracle)
	if _, err := encoder.Resolve("mz-oracle", or, true); err == nil {
		t.Error("hidden encoder accepted for a seat")
	}
	if _, err := encoder.Resolve("mz-oracle", or, false); err != nil {
		t.Errorf("hidden encoder refused outside a seat: %v", err)
	}
}

func TestSchemaHashUnpinned(t *testing.T) {
	a := encoder.Schema{Name: "x", Version: 1, DenseW: 4}
	b := a
	b.Version = 2
	c := a
	c.Hidden = true
	if a.Hash() == b.Hash() || a.Hash() == c.Hash() {
		t.Error("Schema.Hash ignores a field")
	}
	if a.Hash() != a.Hash() {
		t.Error("unstable")
	}
}
