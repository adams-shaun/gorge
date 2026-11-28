// Command irmeter prints how much of the code still reads Forge-spelled
// strings (internal/irmeter): one tab-separated row per metric and scope,
// `metric<TAB>scope<TAB>value`, sorted by metric then scope, then the
// kgen-filter section (irmeter.MeasureKgenFilter): its three refusal-site
// rows and kgen_filter_refuse_drop_pct, one decimal. It reads source
// files only; two runs on one tree print identical bytes.
//
//	go run ./cmd/irmeter
package main

import (
	"bufio"
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
	w := bufio.NewWriter(os.Stdout)
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%d\n", r.Metric, r.Scope, r.Value)
	}
	kf, err := irmeter.MeasureKgenFilter(root)
	if err != nil {
		return err
	}
	for _, r := range kf.Rows() {
		fmt.Fprintf(w, "%s\t%s\t%d\n", r.Metric, r.Scope, r.Value)
	}
	fmt.Fprintln(w, kf.DropRow())
	return w.Flush()
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
