// Command gen regenerates the embedded per-set booster specs in
// draft/limited/setdata/sets from the trimmed MTGJSON booster inputs in
// limited/setdata/mtgjson. It is a build-time tool only: the setdata
// package itself stays pure and reads nothing but its embedded files.
//
//	go run ./internal/gen -sets FDN,BLB
//
// (wired as the //go:generate directive in the parent package; run from
// draft/limited/setdata, where mtgjson/ and sets/ resolve). The output is
// deterministic: setdata.Compile validates the input and re-encodes it
// indented with sorted keys, so a regenerate run is byte-identical to the
// committed file unless the MTGJSON inputs moved.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// Generate compiles one raw trimmed MTGJSON set file into the canonical
// embedded form. It is main's testable body.
func Generate(raw []byte) ([]byte, error) {
	return setdata.Compile(raw)
}

func main() {
	sets := flag.String("sets", "", "comma-separated set codes to (re)generate")
	root := flag.String("root", ".", "draft/limited/setdata directory holding mtgjson/ and sets/")
	mtgjson := flag.String("mtgjson", "mtgjson", "directory holding the trimmed MTGJSON inputs (<dir>/<SET>.json)")
	flag.Parse()
	if *sets == "" {
		fmt.Fprintln(os.Stderr, "usage: gen -sets FDN,BLB [-root .] [-mtgjson mtgjson]")
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
