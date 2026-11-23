// Command cardaudit writes one row a card joining gorge's claim, dirac's
// support lists and sidecar, and the optional hunt ledgers. It is read-only:
// it plays no games and builds no tables.
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"

	// cardaudit reads effects.Supported() for g_claim but never otherwise
	// touches rules, so without this blank import none of engine/rules/*.go's
	// init() functions (each an effects.RegisterNonAPI call) run in this
	// binary, and Supported() sees "api:" primitives only -- every card whose
	// deck uses a keyword, trigger, static or replacement primitive would be
	// reported missing (kw:Flying alone gates thousands). rules_link_test.go's
	// TestSupportedIncludesNonAPIPrimitives pins that this import stays.
	_ "github.com/adams-shaun/gorge/engine/rules"
)

const (
	sidecarDir = "dirac/support/evidence-keys"

	rowHeader = "card\tg_claim\tg_missing\td_claim\td_lists\tlir_now\tlir_credit\tstale\tkeys\tbranches\tunkeyed\tmissing\tlost\tlegacy\tx_claim\tx_bucket\tx_keys\tdelta_open\tdelta_blame\tg_keys"

	needHeader = "card\towed_keys\towed_uses\tpriority"
)

// opts names the optional inputs. An empty path means the input was not
// supplied, and its columns are "-".
type opts struct {
	HuntKeys      string // the hunt's keys.tsv: ctx card key uses ... compared agreed ... status
	XLedger       string // the X evidence ledger CSV: set,card,...,bucket,...,diff_primary,...
	Diagnosed     string // the diagnosed deltas TSV: kind set card field wrong CR ...
	XMageManifest string // xmage's card claim: a name list, or card<TAB>claim rows
}

// row is one card's joined view. The field order is the column order.
type row struct {
	Card       string
	GClaim     string
	GMissing   string
	DClaim     string
	DLists     string
	LIRNow     string
	LIRCredit  string
	Stale      int
	Keys       string
	Branches   string
	Unkeyed    int
	Missing    string
	Lost       int
	Legacy     int
	XClaim     string
	XBucket    string
	XKeys      string
	DeltaOpen  string
	DeltaBlame string
	GKeys      string

	// Context the -need file sorts on; not written to rows.tsv.
	Plays     int
	OwedUses  int  // hunt uses of the missing keys
	OwedKnown bool // the hunt has a row for the card, even when OwedUses is 0
	FourSeat  bool
	sets      []string
}

// lowerResult is one lowering pass: the cards' LIR keys and, when the caller
// ran kgen's support path, each card's status.
type lowerResult struct {
	keys    map[string]string
	support map[string]kgen.CardSupport
}

// lowerFunc lowers the named cards once.
type lowerFunc func([]string) (lowerResult, error)

// audit is the tests' entry point. It is pure and sorted by card name.
// auditRows is the same function with the read errors main reports; audit is
// the wrapper that keeps the tests' call shape. Its lir callback has no status,
// so a key stands in for lowering; run always supplies kgen's status.
func audit(fsys fs.FS, reg *cards.Registry, lir func([]string) map[string]string, o opts) []row {
	rows, err := auditRows(fsys, reg, func(names []string) (lowerResult, error) {
		return lowerResult{keys: lir(names)}, nil
	}, o)
	if err != nil {
		return nil
	}
	return rows
}

func auditRows(fsys fs.FS, reg *cards.Registry, lower lowerFunc, o opts) ([]row, error) {
	rows, _, err := auditInputs(fsys, reg, lower, o)
	return rows, err
}

// auditInputs is auditRows with the joined inputs the rows were built from,
// which -facts reads for the fields a row does not carry.
func auditInputs(fsys fs.FS, reg *cards.Registry, lower lowerFunc, o opts) ([]row, inputs, error) {
	ev, err := readEvidence(fsys)
	if err != nil {
		return nil, inputs{}, err
	}
	sc, err := readSidecars(fsys)
	if err != nil {
		return nil, inputs{}, err
	}
	ref, err := readRefused(fsys)
	if err != nil {
		return nil, inputs{}, err
	}
	legacy, err := readLegacy(fsys)
	if err != nil {
		return nil, inputs{}, err
	}
	hunt, err := readHunt(o.HuntKeys)
	if err != nil {
		return nil, inputs{}, err
	}
	xl, err := readXLedger(o.XLedger)
	if err != nil {
		return nil, inputs{}, err
	}
	dx, err := readDiagnosed(o.Diagnosed)
	if err != nil {
		return nil, inputs{}, err
	}
	xm, err := readXMage(o.XMageManifest)
	if err != nil {
		return nil, inputs{}, err
	}

	names := universe(reg, ev, ref, sc, legacy, hunt, xl, dx, xm)
	lr, err := lower(append([]string(nil), names...))
	if err != nil {
		return nil, inputs{}, err
	}
	if lr.keys == nil {
		lr.keys = map[string]string{}
	}
	in := inputs{
		reg:       reg,
		supported: effects.Supported(),
		lirNow:    lr.keys,
		support:   lr.support,
		ev:        ev,
		ref:       ref,
		sc:        sc,
		legacy:    legacy,
		hunt:      hunt,
		xl:        xl,
		dx:        dx,
		xm:        xm,
	}
	rows := make([]row, 0, len(names))
	for _, name := range names {
		rows = append(rows, buildRow(name, in))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Card < rows[j].Card })
	return rows, in, nil
}

// inputs is the read-only view one row is built from.
type inputs struct {
	reg       *cards.Registry
	supported map[string]bool
	lirNow    map[string]string
	support   map[string]kgen.CardSupport // nil when only LIR keys are known
	ev        *evidence
	ref       map[string]*cardset.RefusedEntry
	sc        map[string]sidecarRow
	legacy    map[string]bool
	hunt      *huntIndex
	xl        *xledgerIndex
	dx        *diagnosedIndex
	xm        map[string]string // nil when -xmage-manifest was not supplied
}

// evidence is the evidence lists, both by card and by list.
type evidence struct {
	lists map[string][]string // card -> the list names it is in, sorted
}

// sidecarRow is one dirac/support/evidence-keys/<list>.tsv row.
type sidecarRow struct {
	Card            string
	LIR             string
	Plays           int
	Keys            string
	Unused          string
	Branches        string
	MissingBranches string
}

func readEvidence(fsys fs.FS) (*evidence, error) {
	files, err := fs.Glob(fsys, path.Join(cardset.EvidenceDir, "*.txt"))
	if err != nil {
		return nil, err
	}
	e := &evidence{lists: map[string][]string{}}
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		list := strings.TrimSuffix(path.Base(f), ".txt")
		for _, n := range readLines(b) {
			// a row's trailing tab-separated columns (an evidence row's seats
			// field) are not part of the name
			if name, _, found := strings.Cut(n, "\t"); found {
				n = name
			}
			e.lists[n] = append(e.lists[n], list)
		}
	}
	for n := range e.lists {
		e.lists[n] = dedupeSorted(e.lists[n])
	}
	return e, nil
}

// listsFor is the list names for a card, by its full name or, for a split
// card, its first face (coverage rows name a split card by its first face).
func (e *evidence) listsFor(name string) []string {
	out := append([]string(nil), e.lists[name]...)
	if f := firstFace(name); f != "" {
		out = append(out, e.lists[f]...)
	}
	return dedupeSorted(out)
}

func (e *evidence) evidenced(name string) bool { return len(e.listsFor(name)) > 0 }

func readSidecars(fsys fs.FS) (map[string]sidecarRow, error) {
	files, err := fs.Glob(fsys, path.Join(sidecarDir, "*.tsv"))
	if err != nil {
		return nil, err
	}
	out := map[string]sidecarRow{}
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		recs, err := readTSV(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, rec := range recs {
			card := rec["card"]
			if card == "" {
				continue
			}
			if _, dup := out[card]; dup {
				continue
			}
			plays, _ := strconv.Atoi(rec["plays"])
			out[card] = sidecarRow{
				Card:            card,
				LIR:             rec["lir16"],
				Plays:           plays,
				Keys:            rec["keys"],
				Unused:          rec["unused"],
				Branches:        rec["branches"],
				MissingBranches: rec["missing_branches"],
			}
		}
	}
	return out, nil
}

func readRefused(fsys fs.FS) (map[string]*cardset.RefusedEntry, error) {
	entries, err := cardset.ReadRefused(fsys)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*cardset.RefusedEntry, len(entries))
	for i := range entries {
		out[entries[i].Card] = &entries[i]
	}
	return out, nil
}

func readLegacy(fsys fs.FS) (map[string]bool, error) {
	out := map[string]bool{}
	b, err := fs.ReadFile(fsys, path.Join(sidecarDir, "LEGACY.txt"))
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, n := range readLines(b) {
		out[n] = true
	}
	return out, nil
}

// huntRow is the part of the hunt's keys.tsv one row needs: Status and Ctx
// name the engine sides. compared/agreed are outcomes, not sides, so they are
// not read.
type huntRow struct {
	Ctx, Status string
}

type huntIndex struct {
	g, x     map[string][]string
	fourSeat map[string]bool
	uses     map[string]map[string]int // card -> key -> hunt uses
}

func readHunt(file string) (*huntIndex, error) {
	idx := &huntIndex{
		g:        map[string][]string{},
		x:        map[string][]string{},
		fourSeat: map[string]bool{},
		uses:     map[string]map[string]int{},
	}
	if file == "" {
		return idx, nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	recs, err := readTSV(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	for _, rec := range recs {
		card, key := rec["card"], rec["key"]
		if card == "" || key == "" {
			continue
		}
		uses, _ := strconv.Atoi(rec["uses"])
		h := huntRow{Ctx: rec["ctx"], Status: rec["status"]}
		g, x := keySides(h)
		if g {
			idx.g[card] = append(idx.g[card], key)
		}
		if x {
			idx.x[card] = append(idx.x[card], key)
		}
		if isFourSeatCtx(h.Ctx) {
			idx.fourSeat[card] = true
		}
		// A key's uses are the card's, not the context's: the contexts that
		// reached the key repeat the same number, so keep the largest. The
		// map exists for every row, so a recorded uses=0 is a real zero and
		// not the same thing as the hunt having no row for the card.
		if idx.uses[card] == nil {
			idx.uses[card] = map[string]int{}
		}
		if cur, seen := idx.uses[card][key]; !seen || uses > cur {
			idx.uses[card][key] = uses
		}
	}
	for _, m := range []map[string][]string{idx.g, idx.x} {
		for k := range m {
			m[k] = dedupeSorted(m[k])
		}
	}
	return idx, nil
}

// owedUses is the hunt's uses for the card's missing keys. A missing key the
// hunt does not record owes one use; ok is false when the hunt has no row for
// the card.
func (h *huntIndex) owedUses(card, missing string) (int, bool) {
	uses := h.uses[card]
	if uses == nil {
		if f := firstFace(card); f != "" {
			uses = h.uses[f]
		}
	}
	if uses == nil {
		return 0, false
	}
	n := 0
	for _, key := range splitList(missing) {
		if u, ok := uses[key]; ok {
			n += u
		} else {
			n++
		}
	}
	return n, true
}

// keySides reads the hunt row's engine columns from the status token. Only
// exact tokens count: both/agree/agreed/same name both engines, x/xmage/x-only
// name X alone, g/gorge/g-only name gorge alone. Any other status (diverged,
// open, ...) names no side and falls through to the ctx column, and then to
// both engines.
func keySides(h huntRow) (g, x bool) {
	s := strings.ToLower(strings.TrimSpace(h.Status))
	switch s {
	case "both", "agree", "agreed", "same":
		return true, true
	case "x", "xmage", "x-only", "xonly":
		return false, true
	case "g", "gorge", "g-only", "gonly":
		return true, false
	}
	c := strings.ToLower(h.Ctx)
	if strings.Contains(c, "xmage") || c == "x" {
		return false, true
	}
	if strings.Contains(c, "gorge") || c == "g" {
		return true, false
	}
	return true, true
}

// isFourSeatCtx marks the hunt contexts that play a pod of three or more.
func isFourSeatCtx(ctx string) bool {
	c := strings.ToLower(ctx)
	return strings.Contains(c, "4seat") || strings.Contains(c, "4-seat") ||
		strings.Contains(c, "four-seat") || strings.Contains(c, "four seat") ||
		strings.Contains(c, "pod")
}

type xledgerIndex struct {
	bucket map[string]string
	open   map[string]int
	sets   map[string][]string
}

func readXLedger(file string) (*xledgerIndex, error) {
	if file == "" {
		return nil, nil
	}
	idx := &xledgerIndex{bucket: map[string]string{}, open: map[string]int{}, sets: map[string][]string{}}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	recs, err := readCSV(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	for _, rec := range recs {
		card := rec["card"]
		if card == "" {
			continue
		}
		if set := rec["set"]; set != "" {
			idx.sets[card] = append(idx.sets[card], set)
		}
		if b := strings.ToUpper(strings.TrimSpace(rec["bucket"])); b != "" {
			if cur, ok := idx.bucket[card]; !ok || bucketRank(b) > bucketRank(cur) {
				idx.bucket[card] = b
			}
		}
		if d := strings.TrimSpace(rec["diff_primary"]); d != "" && d != "-" && d != "0" &&
			!strings.EqualFold(d, "none") && !strings.EqualFold(d, "false") {
			idx.open[card]++
		}
	}
	for k := range idx.sets {
		idx.sets[k] = dedupeSorted(idx.sets[k])
	}
	return idx, nil
}

func bucketRank(b string) int {
	switch strings.ToUpper(strings.TrimSpace(b)) {
	case "HAS-DIFFERENCES":
		return 3
	case "NO-CROSS-ENGINE":
		return 2
	case "AGREEING":
		return 1
	}
	return 0
}

type diagnosedIndex struct {
	blame map[string][5]int // X, F, G, harness, open
}

func readDiagnosed(file string) (*diagnosedIndex, error) {
	if file == "" {
		return nil, nil
	}
	idx := &diagnosedIndex{blame: map[string][5]int{}}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	recs, err := readTSV(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	for _, rec := range recs {
		card := rec["card"]
		if card == "" {
			continue
		}
		n := idx.blame[card]
		switch strings.ToLower(strings.TrimSpace(rec["kind"])) {
		case "x":
			n[0]++
		case "f":
			n[1]++
		case "g":
			n[2]++
		case "harness":
			n[3]++
		default:
			n[4]++
		}
		idx.blame[card] = n
	}
	return idx, nil
}

func (d *diagnosedIndex) String(card string) string {
	n := d.blame[card]
	return fmt.Sprintf("X:%d,F:%d,G:%d,harness:%d,open:%d", n[0], n[1], n[2], n[3], n[4])
}

// readXMage reads xmage's card claim: a JSON name list, a name list, or
// card<TAB>claim rows. A listed card without a claim is "supported".
func readXMage(file string) (map[string]string, error) {
	if file == "" {
		return nil, nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) > 0 && (trimmed[0] == '[' || trimmed[0] == '{') {
		return readXMageJSON(trimmed)
	}
	out := map[string]string{}
	lines := readLines(b)
	if len(lines) == 0 {
		return out, nil
	}
	headed := false
	for _, h := range strings.Split(lines[0], "\t") {
		if strings.EqualFold(strings.TrimSpace(h), "card") {
			headed = true
			break
		}
	}
	if !headed {
		for _, line := range lines {
			fields := strings.SplitN(line, "\t", 2)
			card := strings.TrimSpace(fields[0])
			if card == "" {
				continue
			}
			claim := "supported"
			if len(fields) == 2 && strings.TrimSpace(fields[1]) != "" {
				claim = strings.TrimSpace(fields[1])
			}
			out[card] = claim
		}
		return out, nil
	}
	recs, err := readTSV(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	for _, rec := range recs {
		card := rec["card"]
		if card == "" {
			continue
		}
		claim := rec["claim"]
		if claim == "" {
			claim = rec["status"]
		}
		if claim == "" {
			claim = rec["supported"]
		}
		if claim == "" {
			claim = "supported"
		}
		out[card] = claim
	}
	return out, nil
}

func readXMageJSON(b []byte) (map[string]string, error) {
	out := map[string]string{}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				out[s] = "supported"
			}
		}
	case map[string]any:
		for k, val := range x {
			switch vv := val.(type) {
			case string:
				out[k] = vv
			case bool:
				if vv {
					out[k] = "supported"
				} else {
					out[k] = "missing"
				}
			case []any:
				for _, e := range vv {
					if s, ok := e.(string); ok {
						out[s] = "supported"
					}
				}
			}
		}
	}
	return out, nil
}

// universe is every card in the registry plus every name any input names,
// sorted. A registry card is named as the evidence lists name it: by its first
// face, so a split card is one row. A card in no list (unverified or
// not-lowered) still gets its row.
func universe(reg *cards.Registry, ev *evidence, ref map[string]*cardset.RefusedEntry, sc map[string]sidecarRow,
	legacy map[string]bool, hunt *huntIndex, xl *xledgerIndex, dx *diagnosedIndex, xm map[string]string) []string {
	seen := map[string]bool{}
	add := func(n string) {
		if n != "" {
			seen[n] = true
		}
	}
	for _, c := range reg.AllCards() {
		if c != nil && len(c.Faces) > 0 && c.Faces[0] != nil {
			add(c.Faces[0].Name)
		}
	}
	for n := range ev.lists {
		add(n)
	}
	for n := range ref {
		add(n)
	}
	for n := range sc {
		add(n)
	}
	for n := range legacy {
		add(n)
	}
	for n := range hunt.g {
		add(n)
	}
	for n := range hunt.x {
		add(n)
	}
	for n := range hunt.fourSeat {
		add(n)
	}
	if xl != nil {
		for n := range xl.bucket {
			add(n)
		}
		for n := range xl.open {
			add(n)
		}
	}
	if dx != nil {
		for n := range dx.blame {
			add(n)
		}
	}
	for n := range xm {
		add(n)
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func buildRow(name string, in inputs) row {
	r := row{
		Card:       name,
		GClaim:     "-",
		GMissing:   "-",
		DLists:     "-",
		LIRNow:     "-",
		LIRCredit:  "-",
		Keys:       "-",
		Branches:   "-",
		Missing:    "-",
		XClaim:     "-",
		XBucket:    "-",
		XKeys:      "-",
		DeltaOpen:  "-",
		DeltaBlame: "-",
		GKeys:      "-",
	}
	var c *cards.Card
	if found, ok := in.reg.Lookup(name); ok {
		c = found
		miss := in.reg.Unsupported(c, in.supported)
		if len(miss) == 0 {
			r.GClaim = "supported"
		} else {
			r.GClaim = "missing"
			r.GMissing = strings.Join(dedupeSorted(miss), ",")
		}
		r.Unkeyed = countUnkeyed(c)
	}

	now := lirKeyFor(name, c, in.lirNow)
	if now != "" {
		r.LIRNow = now
	}
	if s, ok := in.sc[name]; ok {
		r.LIRCredit = dash(s.LIR)
		r.Keys = dash(s.Keys)
		r.Branches = dash(s.Branches)
		r.Plays = s.Plays
		r.Missing = missingString(s)
	}
	if r.LIRNow != "-" && r.LIRCredit != "-" && r.LIRNow != r.LIRCredit {
		r.Stale = 1
	}
	if cs, ok := supportFor(name, in.support); ok {
		r.FourSeat = !cs.TwoSeat
	}

	r.DClaim = diracClaim(name, in, now)
	r.DLists = dash(strings.Join(in.ev.listsFor(name), ","))
	if _, ok := in.sc[name]; ok && !in.ev.evidenced(name) {
		r.Lost = 1
	}
	if in.legacy[name] || (firstFace(name) != "" && in.legacy[firstFace(name)]) {
		r.Legacy = 1
	}

	if in.xl != nil {
		if b, ok := in.xl.bucket[name]; ok {
			r.XBucket = b
		}
		r.DeltaOpen = strconv.Itoa(in.xl.open[name])
		r.sets = append([]string(nil), in.xl.sets[name]...)
	}
	if in.dx != nil {
		r.DeltaBlame = in.dx.String(name)
	}
	if in.xm != nil {
		if claim, ok := in.xm[name]; ok {
			r.XClaim = claim
		} else {
			r.XClaim = "missing"
		}
	}
	r.GKeys = dash(strings.Join(in.hunt.g[name], ","))
	r.XKeys = dash(strings.Join(in.hunt.x[name], ","))
	if in.hunt.fourSeat[name] {
		r.FourSeat = true
	}
	if n, ok := in.hunt.owedUses(name, r.Missing); ok {
		r.OwedUses = n
		r.OwedKnown = true
	}
	return r
}

// supportFor is kgen's status for a card, by its full name or, for a split
// card, its first face.
func supportFor(name string, m map[string]kgen.CardSupport) (kgen.CardSupport, bool) {
	if cs, ok := m[name]; ok {
		return cs, true
	}
	if f := firstFace(name); f != "" {
		if cs, ok := m[f]; ok {
			return cs, true
		}
	}
	return kgen.CardSupport{}, false
}

// diracClaim is the support status one home: refused, then kgen's own status
// (not lowered, refused, unverified, supported). Without a support map, as in
// the fixture wrapper, a LIR key is the only lowering signal.
func diracClaim(name string, in inputs, now string) string {
	if _, ok := in.ref[name]; ok {
		return "refused"
	}
	if f := firstFace(name); f != "" {
		if _, ok := in.ref[f]; ok {
			return "refused"
		}
	}
	if cs, ok := supportFor(name, in.support); ok {
		switch cs.Status {
		case cardset.StatusNotLowered:
			return "not-lowered"
		case cardset.StatusRefused:
			return "refused"
		case cardset.StatusUnverified:
			return "unverified"
		case cardset.StatusSupported:
			return "supported"
		}
	}
	if now == "" {
		return "not-lowered"
	}
	if !in.ev.evidenced(name) {
		return "unverified"
	}
	return "supported"
}

func lirKeyFor(name string, c *cards.Card, m map[string]string) string {
	if k := m[name]; k != "" {
		return k
	}
	if f := firstFace(name); f != "" {
		if k := m[f]; k != "" {
			return k
		}
	}
	if c != nil && len(c.Faces) > 0 && c.Faces[0] != nil {
		if k := m[c.Faces[0].Name]; k != "" {
			return k
		}
	}
	return ""
}

func missingString(s sidecarRow) string {
	var out []string
	out = append(out, splitList(s.Unused)...)
	out = append(out, splitList(s.MissingBranches)...)
	out = dedupeSorted(out)
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ",")
}

func countUnkeyed(c *cards.Card) int {
	n := 0
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		n += len(f.Statics) + len(f.Repls) + len(f.Keywords)
	}
	return n
}

func firstFace(name string) string {
	if i := strings.Index(name, " // "); i > 0 {
		return name[:i]
	}
	return ""
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	j := 0
	for i := range out {
		if i == 0 || out[i] != out[j-1] {
			out[j] = out[i]
			j++
		}
	}
	return out[:j]
}

// readLines is a blank/comment-stripped text file.
func readLines(b []byte) []string {
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// readTSV reads a headed tab-separated table, splitting fields on raw tabs and
// never unquoting. A card name such as `"Ach! Hans, Run!"` is a literal name in
// the sidecar (kgen's readCoverageRows keeps it raw); encoding/csv would strip
// the quotes, and the lookup would miss, silently dropping the joined columns.
func readTSV(b []byte) ([]map[string]string, error) {
	var header []string
	out := []map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if header == nil {
			header = fields
			continue
		}
		m := make(map[string]string, len(header)+1)
		for i, h := range header {
			if i < len(fields) {
				m[h] = fields[i]
			}
		}
		out = append(out, m)
	}
	// A file with no header line (empty, or comments only) has no records.
	return out, nil
}

// readCSV reads a headed comma-separated table (the -xledger form). CSV is the
// right reader there: those fields are generated CSV, not raw card names.
func readCSV(b []byte) ([]map[string]string, error) {
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = ','
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.Comment = '#'
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return []map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []map[string]string{}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		m := make(map[string]string, len(header)+1)
		for i, h := range header {
			if i < len(rec) {
				m[h] = rec[i]
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func writeRows(w io.Writer, rows []row) error {
	bw := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(bw, rowHeader); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := fmt.Fprintf(bw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%d\t%s\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Card, r.GClaim, r.GMissing, r.DClaim, r.DLists, r.LIRNow, r.LIRCredit, r.Stale,
			r.Keys, r.Branches, r.Unkeyed, r.Missing, r.Lost, r.Legacy, r.XClaim, r.XBucket,
			r.XKeys, r.DeltaOpen, r.DeltaBlame, r.GKeys); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// writeNeed writes the hunt plan's need/<ctx>.tsv: the supported cards whose
// missing keys are still owed, 4-seat cards first and then fewer plays first.
// owed_uses is the hunt's uses of the owed keys when the hunt has a row for the
// card; otherwise it is one use per owed key (the use the evidence has not yet
// exercised).
func writeNeed(w io.Writer, rows []row) error {
	var need []row
	for _, r := range rows {
		if r.DClaim == "supported" && r.Missing != "" && r.Missing != "-" {
			need = append(need, r)
		}
	}
	sort.Slice(need, func(i, j int) bool {
		if need[i].FourSeat != need[j].FourSeat {
			return need[i].FourSeat
		}
		if need[i].Plays != need[j].Plays {
			return need[i].Plays < need[j].Plays
		}
		return need[i].Card < need[j].Card
	})
	bw := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(bw, needHeader); err != nil {
		return err
	}
	for i, r := range need {
		owed := len(splitList(r.Missing))
		if r.OwedKnown {
			owed = r.OwedUses
		}
		if _, err := fmt.Fprintf(bw, "%s\t%s\t%d\t%d\n", r.Card, r.Missing, owed, i+1); err != nil {
			return err
		}
	}
	return bw.Flush()
}

type summaryCounts struct {
	cards, claimed, evidenced, stale, lost, legacy, covered int
	keyHit, keyTotal, branchHit, branchTotal                int
	buckets                                                 map[string]int
}

func newSummaryCounts() *summaryCounts {
	return &summaryCounts{buckets: map[string]int{}}
}

func (c *summaryCounts) add(r row) {
	c.cards++
	if r.GClaim == "supported" {
		c.claimed++
	}
	if r.DLists != "-" {
		c.evidenced++
	}
	c.stale += r.Stale
	c.lost += r.Lost
	c.legacy += r.Legacy
	kh, kt := ratio(r.Keys)
	bh, bt := ratio(r.Branches)
	c.keyHit, c.keyTotal = c.keyHit+kh, c.keyTotal+kt
	c.branchHit, c.branchTotal = c.branchHit+bh, c.branchTotal+bt
	if r.Keys != "-" && r.Missing == "-" && kh == kt && bh == bt {
		c.covered++
	}
	if r.XBucket != "-" {
		c.buckets[r.XBucket]++
	}
}

// otherBuckets counts the cards whose x_bucket is a value the ledger wrote
// that the three named columns do not cover, so none vanishes from the summary.
func (c *summaryCounts) otherBuckets() int {
	n := 0
	for k, v := range c.buckets {
		switch k {
		case "HAS-DIFFERENCES", "NO-CROSS-ENGINE", "AGREEING":
		default:
			n += v
		}
	}
	return n
}

func ratio(s string) (int, int) {
	hit, total, ok := strings.Cut(s, "/")
	if !ok {
		return 0, 0
	}
	h, _ := strconv.Atoi(hit)
	t, _ := strconv.Atoi(total)
	return h, t
}

func writeSummary(w io.Writer, rows []row) error {
	lists := map[string]*summaryCounts{}
	sets := map[string]*summaryCounts{}
	all := newSummaryCounts()
	for _, r := range rows {
		all.add(r)
		if r.DLists != "-" {
			for _, list := range splitList(r.DLists) {
				if lists[list] == nil {
					lists[list] = newSummaryCounts()
				}
				lists[list].add(r)
			}
		}
		for _, set := range r.sets {
			if sets[set] == nil {
				sets[set] = newSummaryCounts()
			}
			sets[set].add(r)
		}
	}
	bw := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(bw, "| group | cards | claimed | evidenced | stale | lost | legacy | keys | branches | covered | HAS-DIFFERENCES | NO-CROSS-ENGINE | AGREEING | other |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(bw, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|"); err != nil {
		return err
	}
	for _, g := range sortedKeys(lists) {
		if err := summaryLine(bw, "list:"+g, lists[g]); err != nil {
			return err
		}
	}
	for _, g := range sortedKeys(sets) {
		if err := summaryLine(bw, "set:"+g, sets[g]); err != nil {
			return err
		}
	}
	if err := summaryLine(bw, "ALL", all); err != nil {
		return err
	}
	return bw.Flush()
}

func sortedKeys(m map[string]*summaryCounts) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func summaryLine(w io.Writer, group string, c *summaryCounts) error {
	_, err := fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d | %d | %d/%d | %d/%d | %d | %d | %d | %d | %d |\n",
		group, c.cards, c.claimed, c.evidenced, c.stale, c.lost, c.legacy, c.keyHit, c.keyTotal,
		c.branchHit, c.branchTotal, c.covered, c.buckets["HAS-DIFFERENCES"],
		c.buckets["NO-CROSS-ENGINE"], c.buckets["AGREEING"], c.otherBuckets())
	return err
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "cardaudit:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("cardaudit", flag.ContinueOnError)
	cardsDir := flags.String("cards", ".cards", "the compiled corpus directory")
	root := flags.String("root", ".", "the module root holding dirac/support")
	out := flags.String("out", "", "write the rows TSV here (default stdout when no other output is named)")
	summary := flags.String("summary", "", "write the markdown summary here")
	need := flags.String("need", "", "write the need TSV here")
	facts := flags.String("facts", "", "write the per-card facts JSONL here (never committed: a path inside -root must be gitignored)")
	var o opts
	flags.StringVar(&o.HuntKeys, "hunt-keys", "", "the hunt's keys.tsv")
	flags.StringVar(&o.XLedger, "xledger", "", "the X evidence ledger CSV")
	flags.StringVar(&o.Diagnosed, "diagnosed", "", "the diagnosed deltas TSV")
	flags.StringVar(&o.XMageManifest, "xmage-manifest", "", "xmage's card claim list")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *facts != "" {
		if err := checkFactsPath(*root, *facts, gitIgnored); err != nil {
			return err
		}
	}
	reg, err := cards.SharedCorpus(*cardsDir)
	if err != nil {
		return err
	}
	lower := func(names []string) (lowerResult, error) {
		load := make([]string, 0, len(names))
		seen := map[string]bool{}
		for _, n := range names {
			c, ok := reg.Lookup(n)
			if !ok {
				continue
			}
			f := n
			if len(c.Faces) > 0 && c.Faces[0] != nil && c.Faces[0].Name != "" {
				f = c.Faces[0].Name
			}
			if !seen[f] {
				seen[f] = true
				load = append(load, f)
			}
		}
		if len(load) == 0 {
			return lowerResult{keys: map[string]string{}}, nil
		}
		support, err := kgen.SupportFor(*cardsDir, load)
		if err != nil {
			return lowerResult{}, fmt.Errorf("lowering: %w", err)
		}
		keys := make(map[string]string, len(support))
		for name, cs := range support {
			if cs.LIRKey != "" {
				keys[name] = cs.LIRKey
			}
		}
		return lowerResult{keys: keys, support: support}, nil
	}
	rows, in, err := auditInputs(os.DirFS(*root), reg, lower, o)
	if err != nil {
		return err
	}
	wrote := false
	if *facts != "" {
		fo := factsOptsFor(*root, *cardsDir)
		if err := writeFile(*facts, func(w io.Writer) error { return writeFacts(w, buildFacts(rows, in, fo)) }); err != nil {
			return err
		}
		wrote = true
	}
	if *out != "" {
		if err := writeFile(*out, func(w io.Writer) error { return writeRows(w, rows) }); err != nil {
			return err
		}
		wrote = true
	}
	if *summary != "" {
		if err := writeFile(*summary, func(w io.Writer) error { return writeSummary(w, rows) }); err != nil {
			return err
		}
		wrote = true
	}
	if *need != "" {
		if err := writeFile(*need, func(w io.Writer) error { return writeNeed(w, rows) }); err != nil {
			return err
		}
		wrote = true
	}
	if !wrote {
		return writeRows(os.Stdout, rows)
	}
	return nil
}

func writeFile(name string, write func(io.Writer) error) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
