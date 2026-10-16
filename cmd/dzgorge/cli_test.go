package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/contracts"
)

// dz runs dzgorge in-process with env and returns exit code, stdout, stderr.
// It resets play's package-level flag variables afterwards.
func dz(t *testing.T, env []string, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Execute(rootCommand(), argv, env, &out, &errb)
	t.Cleanup(func() { cli.Execute(rootCommand(), []string{"play", "--print-config"}, nil, io.Discard, io.Discard) })
	return code, out.String(), errb.String()
}

// effective is play's --print-config for argv, decoded.
func effective(t *testing.T, env []string, argv ...string) map[string]any {
	t.Helper()
	code, out, errb := dz(t, env, append(argv, "--print-config")...)
	if code != 0 {
		t.Fatalf("%q: exit %d: %s", argv, code, errb)
	}
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(out))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("%q: %v\n%s", argv, err, out)
	}
	if m["$schema"] != "dzgorge-config-1" {
		t.Fatalf("%q: $schema %v", argv, m["$schema"])
	}
	return m["play"].(map[string]any)
}

// TestScriptArgvShapes runs the parser over the exact invocations the repo's
// scripts make (scripts/manamodel_ab.sh, scripts/playlines/pl3_run.sh, the
// 2026-10-07 dzgorge benchmark report) and DraftZero's single-dash style.
func TestScriptArgvShapes(t *testing.T) {
	const net = "/mnt/sata/gorge-training/az-c5b/c5.dzgw"
	b := "az:sims=25:autopay:oppnodes:oppfull:morekinds:closedlist:topk=6:gonet=" + net
	for _, tc := range []struct {
		argv []string
		want map[string]string
	}{
		// manamodel_ab.sh job(): fdn set, flag off.
		{[]string{"play", "-decks", "/in/decks", "-pool", "/in/pool.tsv", "-split", "eval", "-a", "sbtactical", "-b", "bot",
			"-pairs", "600", "-seed", "4242", "-workers", "6", "-mulligans", "2", "-mull-heuristic", "-mm-stats", "-no-paid-mana-census",
			"-out", "/o/fdn-r1-off.jsonl"},
			map[string]string{"decks": "/in/decks", "pool": "/in/pool.tsv", "split": "eval", "a": "sbtactical", "b": "bot", "pairs": "600",
				"seed": "4242", "workers": "6", "mulligans": "2", "mull-heuristic": "true", "mm-stats": "true", "no-paid-mana-census": "true",
				"out": "/o/fdn-r1-off.jsonl"}},
		// manamodel_ab.sh: cons set (-only), r3 az spec.
		{[]string{"play", "-decks", "/o/cdecks", "-pool", "/o/cpool.tsv", "-only", "/o/cons.only", "-split", "eval",
			"-a", "az:sims=100:autopay:oppnodes:oppfull:morekinds:mull", "-b", "sbtactical", "-pairs", "600", "-seed", "4242",
			"-workers", "6", "-mulligans", "2", "-mull-heuristic", "-mm-stats", "-out", "/o/cons-r3-on.jsonl"},
			map[string]string{"only": "/o/cons.only", "a": "az:sims=100:autopay:oppnodes:oppfull:morekinds:mull", "b": "sbtactical",
				"mm-stats": "true", "no-paid-mana-census": "false"}},
		// pl3_run.sh play(): a WIN run, and a single-leg replay with :aseed.
		{[]string{"play", "-decks", "/in/decks", "-pool", "/in/pool.tsv", "-split", "eval", "-a", b + ":explore:nonoise:exploreturns=2",
			"-b", "sbtactical", "-pairs", "200", "-seed", "78", "-workers", "8", "-mulligans", "2", "-mull-heuristic", "-lines",
			"-out", "/o/ex2.jsonl"},
			map[string]string{"a": b + ":explore:nonoise:exploreturns=2", "pairs": "200", "seed": "78", "lines": "true", "single": "false"}},
		{[]string{"play", "-decks", "/in/decks", "-pool", "/in/pool.tsv", "-split", "eval", "-only", "/o/only", "-a", b + ":temp=0.5:aseed=3",
			"-b", "sbtactical", "-pairs", "40", "-single", "-seed", "78", "-workers", "8", "-mulligans", "2", "-mull-heuristic",
			"-lines", "-out", "/o/t05.r3.jsonl"},
			map[string]string{"a": b + ":temp=0.5:aseed=3", "single": "true", "pairs": "40", "only": "/o/only"}},
		// DraftZero's shapes: -flag=value, a remote= spec with colons, the -remote-* knobs, -corpus.
		{[]string{"play", "-cards=/g/.cards", "-a", "az:sims=200:autopay:topk=6:remote=unix:/tmp/dzg.sock", "-b=bot",
			"-pairs=50", "-first-pair", "100", "-corpus", "/o/v.jsonl.gz", "-record-features", "entity", "-record-every", "4",
			"-remote-conns", "8", "-remote-wait", "1ms", "-remote-batch=512", "-gonet-cache", "0", "-max-turns", "60", "-progress", "10",
			"-belief-split", "train", "-out", "g.jsonl"},
			map[string]string{"cards": "/g/.cards", "a": "az:sims=200:autopay:topk=6:remote=unix:/tmp/dzg.sock", "b": "bot", "pairs": "50",
				"first-pair": "100", "corpus": "/o/v.jsonl.gz", "record-features": "entity", "record-every": "4", "remote-conns": "8",
				"remote-wait": "1ms", "remote-batch": "512", "gonet-cache": "0", "max-turns": "60", "progress": "10"}},
		// The GNU spellings reach the same values.
		{[]string{"play", "--pairs", "3", "--seed=9", "--single", "-a=bot:autopay"},
			map[string]string{"pairs": "3", "seed": "9", "single": "true", "a": "bot:autopay"}},
	} {
		got := effective(t, nil, tc.argv...)
		for k, want := range tc.want {
			v, ok := got[k]
			if !ok {
				t.Errorf("%q: no key %q", tc.argv, k)
				continue
			}
			var s string
			switch x := v.(type) {
			case string:
				s = x
			default:
				s = strings.TrimSpace(mustJSON(t, x))
			}
			if s != want {
				t.Errorf("%q: %s = %s, want %s", tc.argv, k, s, want)
			}
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestScriptFlagsExist scans the repo scripts that drive dzgorge play for
// every single-dash flag they pass (inline and in bash arrays) and checks
// play defines it, so a renamed flag fails here before a script breaks.
func TestScriptFlagsExist(t *testing.T) {
	var fs *cli.FlagSet
	cmd := playCommand()
	cli.Execute(&cli.Command{Name: "probe", Sub: []*cli.Command{{
		Name: "play", Flags: cmd.Flags,
		Run: func(c *cli.Context, _ []string) error { fs = c.Flags(); return nil },
	}}}, []string{"play"}, nil, io.Discard, io.Discard)
	t.Cleanup(func() { cli.Execute(rootCommand(), []string{"play", "--print-config"}, nil, io.Discard, io.Discard) })
	flagTok := regexp.MustCompile(`(?:^|[\s(])(-[a-z][a-z0-9-]*)\b`)
	arrayLit := regexp.MustCompile(`\w+=\((-[^)]*)\)`)
	n := 0
	for _, script := range []string{"../../scripts/manamodel_ab.sh", "../../scripts/playlines/pl3_run.sh"} {
		raw, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(raw), "\\\n", " ")
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			// The play invocation from "play" on, and the bash arrays it splices in.
			var parts []string
			if i := strings.Index(line, `" play `); i >= 0 {
				parts = append(parts, line[i:])
			}
			for _, m := range arrayLit.FindAllStringSubmatch(line, -1) {
				parts = append(parts, m[1])
			}
			for _, m := range flagTok.FindAllStringSubmatch(strings.Join(parts, " "), -1) {
				name := m[1][1:]
				if name == "o" { // flock -o
					continue
				}
				n++
				if fs.Lookup(name) == nil {
					t.Errorf("%s passes -%s, which dzgorge play does not define", filepath.Base(script), name)
				}
			}
		}
	}
	if n < 15 {
		t.Fatalf("found only %d flags in the scripts: the scan is broken", n)
	}
}

func TestPlayCLIErrors(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		msg  string
	}{
		{[]string{"play", "-pairz", "3"}, "unknown flag -pairz (did you mean --pairs?)"},
		{[]string{"play", "-a", "bogus"}, `want random, bot, sbtactical, az or prior`},
		{[]string{"play", "-a", "az:autopay=0"}, "is a flag and takes no value"},
		{[]string{"play", "-a", `{"kind":"az","simz":3}`}, `policy object: unknown key "simz"`},
		{[]string{"play", "extra"}, `unexpected argument "extra"`},
		{[]string{"plya"}, `unknown command "plya" (did you mean "play"?)`},
		{[]string{"play", "-remote-wait", "soon"}, `flag -remote-wait "soon"`},
	} {
		code, _, errb := dz(t, nil, tc.argv...)
		if code != 2 || !strings.Contains(errb, tc.msg) {
			t.Errorf("%q: exit %d stderr %q, want 2 and %q", tc.argv, code, errb, tc.msg)
		}
	}
	// Run's own required-flag check keeps its exit code 2.
	var errb string
	if _, code := withStderr(t, &errb, func() (string, int) { c, _, _ := dz(t, nil, "play"); return "", c }); code != 2 || !strings.Contains(errb, "-out is required") {
		t.Errorf("play without -out: exit %d %q", code, errb)
	}
}

// TestPlayEnvAndConfig: GORGE_DZGORGE_PLAY_<FLAG> and a config file, with
// precedence flag > env > config > default.
func TestPlayEnvAndConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(cfg, []byte(`{"play": {"pairs": 7, "seed": 5, "a": {"kind": "az", "sims": 3, "autopay": true}, "mull-heuristic": true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := effective(t, []string{"GORGE_DZGORGE_PLAY_SEED=6", "GORGE_DZGORGE_PLAY_MAX_TURNS=30", "GORGE_DZGORGE_CONFIG=" + cfg}, "play", "-pairs", "8")
	if mustJSON(t, got["pairs"]) != "8" || mustJSON(t, got["seed"]) != "6" || mustJSON(t, got["max-turns"]) != "30" ||
		got["mull-heuristic"] != true || mustJSON(t, got["a"]) != `{"autopay":true,"kind":"az","sims":3}` {
		t.Fatalf("effective: %v", got)
	}
	// A typo anywhere in the file is an error, not only in the invoked section.
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{"pack": {"shrad": 3}}`), 0o644)
	if code, _, errb := dz(t, nil, "play", "--config", bad); code != 2 || !strings.Contains(errb, `unknown key "pack.shrad" (did you mean "pack.shard"?)`) {
		t.Fatalf("typo in another section: exit %d %q", code, errb)
	}
}

// TestContractConfig pins dzgorge-config: an example config and play's
// --print-config of it (every play flag, its type and its default).
func TestContractConfig(t *testing.T) {
	example := filepath.Join(goldenDir, "config_example.json")
	code, out, errb := dz(t, nil, "play", "--config", example, "--print-config")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	if want := `"$schema": "` + contracts.Format(contracts.DZGorgeConfig, contracts.DZGorgeConfigVersion) + `"`; !strings.Contains(out, want) {
		t.Fatalf("print-config does not carry %s:\n%s", want, out)
	}
	checkGolden(t, "config_print.json", []byte(out))
	// Round trip: the printed configuration loads and prints identically.
	p := filepath.Join(t.TempDir(), "rt.json")
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out2, errb := dz(t, nil, "play", "--config", p, "--print-config"); code != 0 || out2 != out {
		t.Fatalf("round trip: exit %d %s\n%s", code, errb, out2)
	}
	// Every other subcommand's section of the example applies too.
	for _, sub := range []string{"pack", "coverage", "version"} {
		if code, _, errb := dz(t, nil, sub, "--config", example, "--print-config"); code != 0 {
			t.Fatalf("%s: exit %d %s", sub, code, errb)
		}
	}
}

// TestVersionStillJSON: version's output is still one JSON object.
func TestVersionStillJSON(t *testing.T) {
	code, out, errb := dz(t, nil, "version", "-cards", t.TempDir())
	var v map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &v) != nil || v["format"] != "dzgorge-version-1" {
		t.Fatalf("version: exit %d %q %q", code, out, errb)
	}
}
