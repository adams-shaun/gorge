package contracts

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/protocol"
)

// repoRoot is the module root: this package sits one directory below it.
const repoRoot = ".."

// TestGoldensMatchRegistry hashes every golden registry.json lists. The
// owner package's own test reproduces the golden byte for byte, so a hash
// mismatch here means what gorge emits changed.
func TestGoldensMatchRegistry(t *testing.T) {
	for _, c := range All() {
		if len(c.Goldens) == 0 {
			t.Errorf("%s: no goldens", c.Name)
		}
		for _, g := range c.Goldens {
			b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(g.Path)))
			if err != nil {
				t.Errorf("%s v%d: golden %s: %v", c.Name, c.Version, g.Path, err)
				continue
			}
			sum := sha256.Sum256(b)
			if got := hex.EncodeToString(sum[:]); got != g.SHA256 {
				t.Errorf("%s v%d: a contract golden changed: %s has sha256 %s, registry.json pins %s.\n"+
					"If the change is additive, update the hash here and add a CHANGELOG line; "+
					"if it is breaking, bump the major (contracts.go and registry.json) and keep the reader "+
					"accepting the old major for one release (docs/contracts.md).",
					c.Name, c.Version, g.Path, got, g.SHA256)
			}
		}
	}
}

// TestConstantsMatchRegistry holds the Go constants, and the version
// constants the owner packages declare, to registry.json.
func TestConstantsMatchRegistry(t *testing.T) {
	want := []NameVersion{
		{DZGGames, DZGGamesVersion},
		{DZGPack, DZGPackVersion},
		{AZVisits, AZVisitsVersion},
		{DZGW, DZGWVersion},
		{DZGRemote, DZGRemoteVersion},
		{GorgeAgent, GorgeAgentVersion},
		{GorgeProtocol, GorgeProtocolVersion},
		{DZGorgeSpec, DZGorgeSpecVersion},
		{DZGorgeVersion, DZGorgeVersionVersion},
	}
	got := Versions()
	if len(got) != len(want) {
		t.Fatalf("registry has %d contracts, the constants name %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("registry entry %d is %v, the constants say %v", i, got[i], want[i])
		}
	}
	if protocol.Version != GorgeProtocolVersion {
		t.Errorf("protocol.Version %d, registry %d", protocol.Version, GorgeProtocolVersion)
	}
	if wire.Version != GorgeAgentVersion || wire.Protocol != GorgeAgent+"/"+strconv.Itoa(GorgeAgentVersion) {
		t.Errorf("wire %s v%d, registry %s v%d", wire.Protocol, wire.Version, GorgeAgent, GorgeAgentVersion)
	}
	if policynet.VisitSchemaVersion != AZVisitsVersion || policynet.VisitRecordType != "azvisits-v1" {
		t.Errorf("policynet visits %s v%d, registry %s v%d", policynet.VisitRecordType, policynet.VisitSchemaVersion, AZVisits, AZVisitsVersion)
	}
	if Format(DZGPack, 2) != "dzg-pack-2" {
		t.Errorf("Format: %s", Format(DZGPack, 2))
	}
}

// TestPackKindsAppendOnly pins PackKinds against its golden: the golden must
// be a prefix of the table (nothing reordered or removed), and the table may
// only grow by appending.
func TestPackKindsAppendOnly(t *testing.T) {
	f, err := os.Open("testdata/pack_kinds.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var pinned []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			pinned = append(pinned, l)
		}
	}
	if len(pinned) > len(PackKinds) {
		t.Fatalf("PackKinds lost entries: the golden pins %d kinds, the table has %d. Removing a kind renumbers every "+
			"dzg-pack shard: that is a new dzg-pack major, never an edit", len(pinned), len(PackKinds))
	}
	for i, k := range pinned {
		if PackKinds[i] != k {
			t.Fatalf("PackKinds[%d] is %q, the golden pins %q: the table is append-only (a reorder renumbers every "+
				"dzg-pack shard on disk)", i, PackKinds[i], k)
		}
	}
	if len(PackKinds) > len(pinned) {
		t.Fatalf("PackKinds gained %v: an append is additive -- add it to contracts/testdata/pack_kinds.txt, update "+
			"that golden's hash in registry.json and add a CHANGELOG line", PackKinds[len(pinned):])
	}
	for i, k := range PackKinds {
		if PackKindIndex(k) != i {
			t.Fatalf("PackKinds has %q twice", k)
		}
	}
}

// TestPackKindsCoverDecisionKinds: every decision.Kind a visit record can
// carry has an index (pack refuses a record whose kind has none).
func TestPackKindsCoverDecisionKinds(t *testing.T) {
	for _, k := range []decision.Kind{decision.KPriority, decision.KTarget, decision.KAttackers, decision.KBlockers,
		decision.KMulligan, decision.KModes, decision.KTriggerOrder, decision.KTriggerOptional, decision.KCommanderZone,
		decision.KChoose, decision.KReplacement, decision.KArrange, decision.KStartingPlayer} {
		if PackKindIndex(string(k)) < 0 {
			t.Errorf("decision kind %q has no PackKinds index: append it", k)
		}
	}
}
