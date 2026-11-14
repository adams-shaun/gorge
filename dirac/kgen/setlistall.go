package kgen

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// corpusAllList writes the ALL set list, one name per line, and returns its
// path: <cardsDir>/kgen/ALL.txt. The list is every corpus card's first Name:,
// byte-sorted and unique. It is mechanically extracted from the corpus, so it
// lives beside the corpus, never in the repository. It is rebuilt on every call,
// because a corpus bump must not read a stale list, and the file is rewritten
// only when its bytes change.
func corpusAllList(cardsDir string) (string, error) {
	body, err := corpusAllNames(filepath.Join(cardsDir, "cardsfolder"))
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cardsDir, "kgen")
	path := filepath.Join(dir, "ALL.txt")
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, body) {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Written whole and renamed: the corpus directory is shared between checkouts.
	tmp, err := os.CreateTemp(dir, "ALL.txt.tmp*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// corpusAllNames is the ALL list's bytes over the scripts under root.
func corpusAllNames(root string) ([]byte, error) {
	var names []string
	// WalkDir does not descend a symlinked root (cards-overlay.sh links it);
	// the resolved path makes it, and only names leave this function.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".txt") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if name, ok := firstScriptName(b); ok {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no card scripts under %s", root)
	}
	slices.Sort(names) // Go compares strings bytewise
	names = slices.Compact(names)
	return []byte(strings.Join(names, "\n") + "\n"), nil
}

// firstScriptName reads a script's first Name: line, the front face of a
// multi-face card.
func firstScriptName(script []byte) (string, bool) {
	for len(script) > 0 {
		line, rest, _ := bytes.Cut(script, []byte("\n"))
		if name, ok := bytes.CutPrefix(line, []byte("Name:")); ok {
			return string(bytes.TrimSuffix(name, []byte("\r"))), true
		}
		script = rest
	}
	return "", false
}
