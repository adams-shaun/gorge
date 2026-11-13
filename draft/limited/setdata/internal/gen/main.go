// Command gen regenerates the embedded per-set booster specs in
// draft/limited/setdata/sets from the trimmed MTGJSON booster inputs in
// limited/setdata/mtgjson. It is a build-time tool only: the setdata
// package itself stays pure and reads nothing but its embedded files.
//
//	go run ./internal/gen -sets FDN,BLB,TRK,MBC
//
// (wired as the //go:generate directive in the parent package; run from
// draft/limited/setdata, where mtgjson/ and sets/ resolve). The output is
// deterministic: setdata.Compile validates the input and re-encodes it
// indented with sorted keys, so a regenerate run is byte-identical to the
// committed file unless the MTGJSON inputs moved.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// Generate compiles one raw trimmed MTGJSON set file into the canonical
// embedded form, after the set's adjustments (adjust). It is main's
// testable body.
func Generate(raw []byte) ([]byte, error) {
	adjusted, err := adjust(raw)
	if err != nil {
		return nil, err
	}
	return setdata.Compile(adjusted)
}

// sheetFolds are the sheets a set's packs leave out, each slot drawn from
// another sheet instead: set code, then dropped sheet to the sheet that
// takes its slot.
var sheetFolds = map[string]map[string]string{
	// MBC playtest cards are out by decision: Forge scripts too few of them, and the slot draws a second wildcard so packs stay 20 cards (pick 2 needs an even pack).
	"MBC": {"playtest": "wildcard"},
}

// adjust applies the set's sheetFolds to a raw trimmed MTGJSON file: the
// dropped sheet leaves every config, each variant's count for it moves to
// the receiving sheet, and the source line says so. A set without folds is
// returned unchanged, byte for byte.
func adjust(raw []byte) ([]byte, error) {
	var head struct {
		Set string `json:"set"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	folds := sheetFolds[head.Set]
	if len(folds) == 0 {
		return raw, nil
	}
	var f map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&f); err != nil {
		return nil, err
	}
	configs, _ := f["configs"].(map[string]any)
	for _, from := range sortedKeys(folds) {
		to := folds[from]
		for _, name := range sortedKeys(configs) {
			cfg, _ := configs[name].(map[string]any)
			sheets, _ := cfg["sheets"].(map[string]any)
			if _, ok := sheets[from]; !ok {
				return nil, fmt.Errorf("%s collation %q: no %q sheet to fold", head.Set, name, from)
			}
			if _, ok := sheets[to]; !ok {
				return nil, fmt.Errorf("%s collation %q: no %q sheet to fold %q into", head.Set, name, to, from)
			}
			delete(sheets, from)
			variants, _ := cfg["variants"].([]any)
			for _, v := range variants {
				contents, _ := v.(map[string]any)["contents"].(map[string]any)
				n, ok := contents[from].(json.Number)
				if !ok {
					continue
				}
				add, err := n.Int64()
				if err != nil {
					return nil, err
				}
				have := int64(0)
				if m, ok := contents[to].(json.Number); ok {
					if have, err = m.Int64(); err != nil {
						return nil, err
					}
				}
				delete(contents, from)
				contents[to] = have + add
			}
		}
		f["source"] = fmt.Sprintf("%v The %s sheet is left out and its slot drawn from the %s sheet.", f["source"], from, to)
	}
	return json.Marshal(f)
}

// sortedKeys lists a map's keys, sorted: no map order may reach the output.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func main() {
	sets := flag.String("sets", "", "comma-separated set codes to (re)generate")
	root := flag.String("root", ".", "draft/limited/setdata directory holding mtgjson/ and sets/")
	mtgjson := flag.String("mtgjson", "mtgjson", "directory holding the trimmed MTGJSON inputs (<dir>/<SET>.json)")
	flag.Parse()
	if *sets == "" {
		fmt.Fprintln(os.Stderr, "usage: gen -sets FDN,BLB,TRK [-root .] [-mtgjson mtgjson]")
		os.Exit(2)
	}
	for _, code := range strings.Split(*sets, ",") {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if err := run(code, *root, *mtgjson); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", filepath.Join(*root, "sets", code+".json"))
	}
}

// run regenerates one set's embedded file from its trimmed MTGJSON input.
func run(code, root, mtgjson string) error {
	raw, err := os.ReadFile(filepath.Join(root, mtgjson, code+".json"))
	if err != nil {
		return err
	}
	out, err := Generate(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "sets", code+".json"), out, 0o644)
}
