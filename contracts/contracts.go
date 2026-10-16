// Package contracts is gorge's registry of external contracts: every file
// layout, wire and grammar that a program outside this module reads or
// writes (DraftZero's trainer first among them). Each contract has a name, a
// major version and golden files whose SHA-256 the registry pins
// (registry.json, checked by TestGoldensMatchRegistry), so a change to what
// gorge emits cannot land without the author seeing that it changed.
//
// The rules (docs/contracts.md):
//
//   - An additive change (a new field a reader may ignore, a new table entry
//     appended at the end) keeps the major version: update the golden, its
//     hash in registry.json, and add a CHANGELOG line.
//   - Anything else (a renamed, removed, retyped or reordered field, a
//     changed meaning) is a new major version. The reader keeps accepting the
//     previous major for one release.
//   - A reader refuses a major version it does not know, by name.
//
// The package is stdlib-only and imports nothing from gorge, so any consumer
// (or a reviewer) can read the contract list without pulling in the engine.
package contracts

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
)

// Contract names and their current major versions. A Go constant pair per
// registry entry; TestConstantsMatchRegistry holds them to registry.json.
const (
	// DZGGames is dzgorge play's games.jsonl record and its .summary.json.
	DZGGames        = "dzg-games"
	DZGGamesVersion = 1
	// DZGPack is dzgorge pack's training-shard directory (.npy + meta.json).
	DZGPack        = "dzg-pack"
	DZGPackVersion = 2
	// AZVisits is the visit corpus (policynet.VisitRecord, record_type azvisits-v1).
	AZVisits        = "azvisits"
	AZVisitsVersion = 1
	// DZGW is the .dzgw network-weights file internal/gonet loads.
	DZGW        = "dzgw"
	DZGWVersion = 1
	// DZGRemote is the GZB1/GZR1 batch scorer wire (cmd/dzgorge remote=).
	DZGRemote        = "dzg-remote"
	DZGRemoteVersion = 1
	// GorgeAgent is draftlab/extern/wire's gorge-agent protocol.
	GorgeAgent        = "gorge-agent"
	GorgeAgentVersion = 1
	// GorgeProtocol is protocol's host-client wire (protocol.Version).
	GorgeProtocol        = "gorge-protocol"
	GorgeProtocolVersion = 1
	// DZGorgeSpec is dzgorge's policy spec grammar (az:sims=100:autopay:...).
	DZGorgeSpec        = "dzgorge-spec"
	DZGorgeSpecVersion = 1
	// DZGorgeVersion is dzgorge version's JSON object.
	DZGorgeVersion        = "dzgorge-version"
	DZGorgeVersionVersion = 1
	// DZGorgeConfig is dzgorge's --config file and --print-config output.
	DZGorgeConfig        = "dzgorge-config"
	DZGorgeConfigVersion = 1
)

// Format is a contract's versioned format tag, "<name>-<major>" (for
// example "dzg-pack-2"), the spelling a file's format field carries.
func Format(name string, major int) string { return name + "-" + strconv.Itoa(major) }

// Golden is one golden file a contract's owner test reproduces byte for byte.
type Golden struct {
	// Path is repo-relative, slash-separated.
	Path string `json:"path"`
	// SHA256 is the file's hex SHA-256.
	SHA256 string `json:"sha256"`
}

// Contract is one registry entry.
type Contract struct {
	Name        string   `json:"name"`
	Version     int      `json:"version"`
	Description string   `json:"description"`
	Owner       string   `json:"owner"` // the producing package, repo-relative
	Goldens     []Golden `json:"goldens"`
}

//go:embed registry.json
var registryJSON []byte

var registry = mustParse(registryJSON)

func mustParse(b []byte) []Contract {
	var r struct {
		Contracts []Contract `json:"contracts"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		panic("contracts: registry.json: " + err.Error())
	}
	seen := map[string]bool{}
	for _, c := range r.Contracts {
		if c.Name == "" || c.Version < 1 || seen[c.Name] {
			panic(fmt.Sprintf("contracts: registry.json: bad or duplicate entry %q v%d", c.Name, c.Version))
		}
		seen[c.Name] = true
	}
	return r.Contracts
}

// All is every registered contract, in registry order. The slice is a copy.
func All() []Contract {
	out := make([]Contract, len(registry))
	copy(out, registry)
	return out
}

// Lookup is the named contract.
func Lookup(name string) (Contract, bool) {
	for _, c := range registry {
		if c.Name == name {
			return c, true
		}
	}
	return Contract{}, false
}

// NameVersion is one contract's name and major version.
type NameVersion struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// Versions is every contract's name and major version, in registry order.
func Versions() []NameVersion {
	out := make([]NameVersion, len(registry))
	for i, c := range registry {
		out[i] = NameVersion{c.Name, c.Version}
	}
	return out
}
