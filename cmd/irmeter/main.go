// Command irmeter prints how much of the code still reads Forge-spelled
// strings (internal/irmeter): one tab-separated row per metric and scope,
// `metric<TAB>scope<TAB>value`, sorted by metric then scope, then the
// kgen-filter section (irmeter.MeasureKgenFilter), scope dirac/kgen: its
// three refusal-site rows, kgen_filter_refuse_drop_pct (one decimal),
// kgen_filter_refuse_sites (their sum), kgen_refusal_sites_total (every
// refusal site of the package, counted the same way),
// kgen_filter_refuse_site_share_pct (the sum as a percentage of that total,
// one decimal) and kgen_refuse_sites_removed_by_typed (string_only -
// typed_only, at least 0).
// It reads source files only; two runs on one tree print identical bytes.
//
//	go run ./cmd/irmeter
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/internal/irmeter"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "irmeter:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	rows, err := irmeter.Measure(root)
	if err != nil {
		return err
	}
	kf, err := irmeter.MeasureKgenFilter(root)
	if err != nil {
		return err
	}
	return irmeter.Print(os.Stdout, rows, kf)
}

// moduleRoot walks up from the working directory to the directory holding
// go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}
