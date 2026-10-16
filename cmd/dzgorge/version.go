package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/contracts"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// gorgeBuild is the gorge build that wrote a record: the module version and
// the VCS revision the binary was built from (runtime/debug.ReadBuildInfo).
// games.jsonl records, their summary and dzg-pack-2 meta carry it.
type gorgeBuild struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

// buildGorge is this binary's gorgeBuild; tests pin it for their goldens.
var buildGorge = readGorgeBuild()

func readGorgeBuild() gorgeBuild {
	b := gorgeBuild{Version: "(unknown)"}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return b
	}
	b.Version = bi.Main.Version
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			b.Revision = s.Value
		}
	}
	return b
}

// versionInfo is dzgorge version's object (contract dzgorge-version).
type versionInfo struct {
	Format string `json:"format"`
	Module struct {
		Path    string `json:"path"`
		Version string `json:"version"`
	} `json:"module"`
	VCS struct {
		Revision string `json:"revision"`
		Modified bool   `json:"modified"`
	} `json:"vcs"`
	Go string `json:"go"`
	// Contracts maps every registered contract's name to its major version.
	Contracts map[string]int `json:"contracts"`
	// EntityEncoderHash is EncoderHashFor(FeaturesEntity), %016x: the hash a
	// dzgw-1 header's encoder_hash and an entity visit record's must equal.
	EntityEncoderHash string `json:"entity_encoder_hash"`
	// CardsCompiler is cards.CompilerFingerprint; IRCacheVersion the IR cache schema.
	CardsCompiler  string `json:"cards_compiler_fingerprint"`
	IRCacheVersion int    `json:"ir_cache_version"`
	// CardsLock is the -cards corpus's cards.lock, null when none is found.
	CardsLock *versionLock `json:"cards_lock"`
}

type versionLock struct {
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
	Digest string `json:"digest"`
}

func buildVersionInfo(cardsDir string) versionInfo {
	var v versionInfo
	v.Format = contracts.Format(contracts.DZGorgeVersion, contracts.DZGorgeVersionVersion)
	v.Module.Version = "(unknown)"
	if bi, ok := debug.ReadBuildInfo(); ok {
		v.Module.Path, v.Module.Version, v.Go = bi.Main.Path, bi.Main.Version, bi.GoVersion
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				v.VCS.Revision = s.Value
			case "vcs.modified":
				v.VCS.Modified = s.Value == "true"
			}
		}
	}
	v.Contracts = map[string]int{}
	for _, c := range contracts.Versions() {
		v.Contracts[c.Name] = c.Version
	}
	v.EntityEncoderHash = fmt.Sprintf("%016x", policynet.EncoderHashFor(policynet.FeaturesEntity))
	v.CardsCompiler = cards.CompilerFingerprint
	v.IRCacheVersion = cards.CacheVersion()
	if l, err := cards.ReadLock(cardsDir); err == nil {
		v.CardsLock = &versionLock{Repo: l.Repo, Commit: l.Commit, Digest: l.Digest}
	}
	return v
}

func runVersion(args []string) int {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	cardsDir := fs.String("cards", ".cards", "gorge corpus directory whose cards.lock to report")
	fs.Parse(args)
	js, err := json.MarshalIndent(buildVersionInfo(*cardsDir), "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "version:", err)
		return 1
	}
	fmt.Println(string(js))
	return 0
}
