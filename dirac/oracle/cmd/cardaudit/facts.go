package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// fact is one card's line of the -facts JSONL. Every field is a fact the
// audit already joined or a hash of the card's script; none is script text.
// Everything outside Private is the public view; Private holds the engine
// primitive sets a consumer that republishes the file must drop.
type fact struct {
	Name         string        `json:"name"`
	NormName     string        `json:"norm_name"`
	ForgeRef     string        `json:"forge_ref"`
	GorgeSHA     string        `json:"gorge_sha"`
	ScriptSHA256 string        `json:"script_sha256"`
	Abilities    factAbilities `json:"abilities"`
	GClaim       string        `json:"g_claim"`
	GMissing     []string      `json:"g_missing"`
	DClaim       string        `json:"d_claim"`
	DLists       []string      `json:"d_lists"`
	LIRNow       string        `json:"lir_now"`
	LIRCredit    string        `json:"lir_credit"`
	Lowering     factLowering  `json:"lowering"`
	Refused      *factRefused  `json:"refused"`
	Keys         string        `json:"keys"`
	Branches     string        `json:"branches"`
	Private      factPrivate   `json:"private"`
}

// factAbilities counts what the card's faces carry, across every face.
type factAbilities struct {
	Abilities    int `json:"abilities"`
	Triggers     int `json:"triggers"`
	Statics      int `json:"statics"`
	Replacements int `json:"replacements"`
	Keywords     int `json:"keywords"`
}

// factLowering is kgen's status for the card and the IR node of each lowering
// refusal it recorded. Status is "" when the run had no kgen support map.
type factLowering struct {
	Status string   `json:"status"`
	Nodes  []string `json:"nodes"`
}

// factRefused is the card's dirac/support/refused entry; null when it has none.
type factRefused struct {
	Why string `json:"why"`
}

// factPrivate is the part of a fact not meant to leave the operator's hands.
type factPrivate struct {
	Prims []string `json:"prims"`
}

// factsOpts is what a fact reads outside the audit's joined inputs. Every
// field has a fallback in its zero value, so a fixture run needs none.
type factsOpts struct {
	GorgeSHA    string                            // the module's git HEAD; "" when unknown
	ForgeCommit string                            // cards.lock's pinned forge commit
	ForgeBase   string                            // cards.lock's fetched_path of the scripts
	ReadScript  func(path string) ([]byte, error) // nil: no script hash
}

// buildFacts is one fact a row, in the rows' order.
func buildFacts(rows []row, in inputs, fo factsOpts) []fact {
	out := make([]fact, 0, len(rows))
	for _, r := range rows {
		out = append(out, buildFact(r, in, fo))
	}
	return out
}

func buildFact(r row, in inputs, fo factsOpts) fact {
	f := fact{
		Name:      r.Card,
		NormName:  cards.NormalizeName(r.Card),
		GorgeSHA:  fo.GorgeSHA,
		GClaim:    undash(r.GClaim),
		GMissing:  nonNil(splitList(undash(r.GMissing))),
		DClaim:    r.DClaim,
		DLists:    nonNil(splitList(undash(r.DLists))),
		LIRNow:    undash(r.LIRNow),
		LIRCredit: undash(r.LIRCredit),
		Keys:      undash(r.Keys),
		Branches:  undash(r.Branches),
		Lowering:  factLowering{Nodes: []string{}},
		Private:   factPrivate{Prims: []string{}},
	}
	if c, ok := in.reg.Lookup(r.Card); ok {
		f.Abilities = abilitiesOf(c)
		f.Private.Prims = primsOf(c)
		f.ForgeRef, f.ScriptSHA256 = scriptFacts(c, fo)
	}
	if cs, ok := supportFor(r.Card, in.support); ok {
		f.Lowering.Status = cs.Status.String()
		f.Lowering.Nodes = nonNil(append([]string(nil), cs.Nodes...))
	}
	if e, ok := in.ref[r.Card]; ok {
		f.Refused = &factRefused{Why: e.Why}
	} else if ff := firstFace(r.Card); ff != "" {
		if e, ok := in.ref[ff]; ok {
			f.Refused = &factRefused{Why: e.Why}
		}
	}
	return f
}

func undash(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func abilitiesOf(c *cards.Card) factAbilities {
	var a factAbilities
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		a.Abilities += len(f.Abilities)
		a.Triggers += len(f.Triggers)
		a.Statics += len(f.Statics)
		a.Replacements += len(f.Repls)
		a.Keywords += len(f.Keywords)
	}
	return a
}

// primsOf is the card's primitive set and the value heads its SVars read:
// the keys g_missing is taken from.
func primsOf(c *cards.Card) []string {
	return nonNil(dedupeSorted(append(c.Primitives(), c.ValueHeads()...)))
}

// scriptFacts is the card's forge reference and the sha256 of its script. The
// reference is "<forge commit>:<fetched path>/<script>", the way git names a
// blob; it degrades to the script's path under cardsfolder when cards.lock
// was not read. The hash is "" when the script cannot be read.
func scriptFacts(c *cards.Card, fo factsOpts) (ref, sum string) {
	rel := filepath.ToSlash(c.Path)
	if i := strings.LastIndex(rel, "/cardsfolder/"); i >= 0 {
		rel = rel[i+len("/cardsfolder/"):]
	}
	ref = rel
	if fo.ForgeCommit != "" {
		ref = fo.ForgeCommit + ":" + path.Join(fo.ForgeBase, rel)
	}
	if fo.ReadScript != nil {
		if b, err := fo.ReadScript(c.Path); err == nil {
			h := sha256.Sum256(b)
			sum = hex.EncodeToString(h[:])
		}
	}
	return ref, sum
}

// writeFacts writes one JSON object a line. encoding/json writes struct fields
// in declaration order, so the bytes depend only on the facts.
func writeFacts(w io.Writer, facts []fact) error {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	for i := range facts {
		if err := enc.Encode(&facts[i]); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// factsOptsFor reads what the run supplies a fact: the module's HEAD and the
// corpus lock. Anything unreadable leaves its field at the fallback.
func factsOptsFor(root, cardsDir string) factsOpts {
	fo := factsOpts{GorgeSHA: gitHead(root)}
	if b, err := os.ReadFile(filepath.Join(cardsDir, "cards.lock")); err == nil {
		var lock struct {
			Commit       string   `json:"commit"`
			FetchedPath  string   `json:"fetched_path"`
			FetchedPaths []string `json:"fetched_paths"`
		}
		if json.Unmarshal(b, &lock) == nil {
			fo.ForgeCommit = lock.Commit
			fo.ForgeBase = lock.FetchedPath
			if fo.ForgeBase == "" && len(lock.FetchedPaths) > 0 {
				fo.ForgeBase = lock.FetchedPaths[0]
			}
		}
	}
	fo.ReadScript = func(p string) ([]byte, error) {
		b, err := os.ReadFile(p)
		if err == nil {
			return b, nil
		}
		// A corpus moved since it was compiled records the old directory.
		if i := strings.LastIndex(filepath.ToSlash(p), "/cardsfolder/"); i >= 0 {
			return os.ReadFile(filepath.Join(cardsDir, filepath.FromSlash(filepath.ToSlash(p)[i+1:])))
		}
		return nil, err
	}
	return fo
}

// gitHead is root's HEAD commit, or "" when root is not a git checkout.
func gitHead(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// checkFactsPath refuses a -facts path inside the module root unless git
// ignores it: the file names every card's script hash, and is never committed.
// ignored reports whether git ignores p; a path outside root needs no answer.
func checkFactsPath(root, p string, ignored func(root, p string) bool) error {
	absRoot, err := realAbs(root)
	if err != nil {
		return err
	}
	absP, err := realAbs(p)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absP)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	if !ignored(absRoot, absP) {
		return fmt.Errorf("-facts %s is inside the module and not gitignored: write it outside the repo or under an ignored path", p)
	}
	return nil
}

// realAbs is p made absolute with symlinks resolved in its longest existing
// prefix, so a path through a symlink compares against the module's real root.
func realAbs(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	var rest []string
	for cur := abs; ; {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				real = filepath.Join(real, rest[i])
			}
			return real, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = append(rest, filepath.Base(cur))
		cur = parent
	}
}

// gitIgnored asks git whether p is ignored in root's checkout. A git that
// cannot answer (not a checkout, not installed) says no.
func gitIgnored(root, p string) bool {
	return exec.Command("git", "-C", root, "check-ignore", "-q", "--", p).Run() == nil
}
