package cards

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	for _, src := range []string{
		"Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n",
		"Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n",
		"Name:Delver of Secrets\nManaCost:U\nTypes:Creature Human Wizard\nPT:1/1\nOracle:x\nALTERNATE\nName:Insectile Aberration\nTypes:Creature Human Insect\nPT:3/2\nK:Flying\nOracle:x\n",
	} {
		c, _ := ParseBytes("fixture.txt", []byte(src))
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		r.Add(c)
	}
	return r
}

func TestRegistryLookupNormalisation(t *testing.T) {
	r := fixtureRegistry(t)
	for _, name := range []string{
		"Lightning Bolt", "lightning bolt", "  Lightning  Bolt ",
		"Delver of Secrets", "Delver of Secrets // Insectile Aberration",
	} {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("Lookup(%q) missed", name)
		}
	}
	if _, ok := r.Lookup("Not A Card"); ok {
		t.Error("Lookup matched a card that does not exist")
	}
}

func TestRegistryCacheRoundTrip(t *testing.T) {
	r := fixtureRegistry(t)
	path := filepath.Join(t.TempDir(), "ir.gob.gz")
	if err := r.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(back.Cards) != len(r.Cards) {
		t.Fatalf("cards = %d, want %d", len(back.Cards), len(r.Cards))
	}
	// Structure must survive, not just names: the sub-ability tree and the
	// intrinsic mana ability are what the engine actually consumes.
	bolt, ok := back.Lookup("Lightning Bolt")
	if !ok {
		t.Fatal("bolt missing after round trip")
	}
	if sa := bolt.Faces[0].SpellAbility(); sa == nil || sa.Params["NumDmg"] != "3" {
		t.Fatalf("bolt spell ability = %+v", sa)
	}
	mtn, _ := back.Lookup("Mountain")
	if len(mtn.Faces[0].ManaAbilities()) != 1 {
		t.Fatal("intrinsic mana ability lost in round trip")
	}
}

func TestRegistryRejectsTruncatedCache(t *testing.T) {
	r := fixtureRegistry(t)
	tests := []struct {
		name    string
		mutate  func(path string) error // corrupts the cache file
		wantErr bool
	}{
		{
			name:    "intact_cache",
			mutate:  func(path string) error { return nil }, // no mutation
			wantErr: false,
		},
		{
			name: "truncate_by_1_byte",
			mutate: func(path string) error {
				fi, err := os.Stat(path)
				if err != nil {
					return err
				}
				return os.Truncate(path, fi.Size()-1)
			},
			wantErr: true,
		},
		{
			name: "truncate_by_4_bytes",
			mutate: func(path string) error {
				fi, err := os.Stat(path)
				if err != nil {
					return err
				}
				return os.Truncate(path, fi.Size()-4)
			},
			wantErr: true,
		},
		{
			name: "truncate_by_8_bytes_whole_trailer",
			mutate: func(path string) error {
				fi, err := os.Stat(path)
				if err != nil {
					return err
				}
				return os.Truncate(path, fi.Size()-8)
			},
			wantErr: true,
		},
		{
			name: "truncate_by_10_bytes",
			mutate: func(path string) error {
				fi, err := os.Stat(path)
				if err != nil {
					return err
				}
				return os.Truncate(path, fi.Size()-10)
			},
			wantErr: true,
		},
		{
			name: "flip_byte_in_trailer",
			mutate: func(path string) error {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if len(data) < 8 {
					return nil // skip if file too small
				}
				// Flip a bit in the last 8 bytes (the trailer)
				data[len(data)-3] ^= 0x01
				return os.WriteFile(path, data, 0o644)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			path := filepath.Join(tmpdir, "ir.gob.gz")
			if err := r.Save(path); err != nil {
				t.Fatalf("Save: %v", err)
			}
			if err := tt.mutate(path); err != nil {
				t.Fatalf("mutate: %v", err)
			}
			_, err := LoadRegistry(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadRegistry error = %v, want error = %v", err, tt.wantErr)
			}
			// For the intact case, verify structure survives
			if !tt.wantErr {
				back, _ := LoadRegistry(path)
				bolt, ok := back.Lookup("Lightning Bolt")
				if !ok || bolt.Faces[0].SpellAbility() == nil {
					t.Fatal("intact cache lost structure")
				}
			}
		})
	}
}

// writeCardFile drops one Forge-shaped script into dir for CompileDir to pick
// up, creating dir (and any missing parents, e.g. a fixture's cardsfolder or
// tokenscripts sibling) first.
func writeCardFile(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestCompileDirDiagnosesCardWithNoNamedFace covers the real corpus defect
// found in bind_liberate.txt/start_fire.txt: a script built entirely from
// CopyFaceFrom (a directive this parser doesn't resolve) parses without
// error into a Card whose only Face has every field, including Name, at its
// zero value. That must not vanish silently — it should surface as exactly
// one diagnostic per card, and Coverage must still treat the card as absent
// rather than "supported".
func TestCompileDirDiagnosesCardWithNoNamedFace(t *testing.T) {
	dir := t.TempDir()
	writeCardFile(t, dir, "nameless.txt", "CopyFaceFrom:Bind\nAlternateMode:Split\n")

	r, diags, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("diags = %+v, want exactly 1", diags)
	}
	if !strings.HasSuffix(diags[0].Path, "nameless.txt") {
		t.Errorf("diag path = %q, want it to name nameless.txt", diags[0].Path)
	}
	if !strings.Contains(diags[0].Msg, "no named face") {
		t.Errorf("diag msg = %q, want it to say the card has no named face", diags[0].Msg)
	}

	if len(r.Cards) != 1 {
		t.Fatalf("Cards = %d, want 1 (the card still compiles, just flagged)", len(r.Cards))
	}
	cv := r.Coverage(map[string]bool{})
	if cv.Cards != 0 {
		t.Errorf("Coverage.Cards = %d, want 0 (nameless card excluded)", cv.Cards)
	}
	if cv.Supported != 0 {
		t.Errorf("Coverage.Supported = %d, want 0 (nameless card must not count as playable)", cv.Supported)
	}
}

// TestCompileDirIgnoresNamelessAlternateFace is the regression guard for the
// distinction the fix must preserve: a card is only diagnosed when NO face
// has a name. A normal single-face card, and a two-face card whose primary
// face is named and whose ALTERNATE is a nameless CopyFaceFrom stub (the
// shape used by ~20 legitimate split/transform cards in the real corpus),
// must both produce zero "no named face" diagnostics — otherwise the check
// would blow through the diagnostic budget on cards that are working as
// designed.
func TestCompileDirIgnoresNamelessAlternateFace(t *testing.T) {
	dir := t.TempDir()
	writeCardFile(t, dir, "normal.txt", "Name:Normal Card\nTypes:Sorcery\nOracle:x\n")
	writeCardFile(t, dir, "splitcard.txt",
		"Name:Primary Face\nTypes:Creature\nOracle:x\nALTERNATE\nCopyFaceFrom:SomeAlt\n")

	r, diags, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir: %v", err)
	}
	for _, d := range diags {
		if strings.Contains(d.Msg, "no named face") {
			t.Errorf("unexpected nameless-face diagnostic: %+v", d)
		}
	}
	if len(r.Cards) != 2 {
		t.Fatalf("Cards = %d, want 2", len(r.Cards))
	}
	cv := r.Coverage(map[string]bool{})
	if cv.Cards != 2 {
		t.Errorf("Coverage.Cards = %d, want 2 (both cards have a named face)", cv.Cards)
	}
}
