// Command repro turns a feedback snapshot directory — the match.json /
// log.json / view.json (plus report.json) a gorged server's feedback
// capture (task fbrepro1) writes beside a bug report — into a working
// reproduction:
//
//	repro <feedback-dir>              replay the report, verify the head,
//	                                  print the board at the report point
//	repro <feedback-dir> -at N        replay to intent N instead
//	repro <feedback-dir> -list        print the intent timeline (the
//	                                  index/player/choice summary a seat
//	                                  needs to find the moment)
//	repro <feedback-dir> -omniscient  show every hand in the summary
//	repro <feedback-dir> -emit-test <pkg>
//	                                  copy the snapshot into
//	                                  <pkg>/testdata/feedback/<id>/ and
//	                                  write a failing test skeleton that
//	                                  loads it with
//	                                  feedback.EngineAt
//
// The replay is the ordinary engine replay (replay.Replay/ReplayTo) against
// a Config rebuilt from match.json — decks from the recorded card names,
// token scripts recompiled from the recorded text — so a final head that
// does not equal log.json's `head` means the engine or the corpus changed
// since the report was filed. That is a real, expected cause (the corpus is
// a moving pin), and repro says so when it happens, naming the first
// event where the replay parted ways with the recording.
//
// Exit codes: 0 a verified replay and summary; 1 a divergence or replay
// error; 2 a usage or load error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil/feedback"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("repro", flag.ContinueOnError)
	fs.SetOutput(stderr)
	at := fs.Int("at", -1, "replay to intent N instead of the capture point")
	list := fs.Bool("list", false, "print the intent timeline instead of the board summary")
	omniscient := fs.Bool("omniscient", false, "show every seat's hand in the summary")
	emit := fs.String("emit-test", "", "write a test skeleton into package <pkg> that reproduces this snapshot")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: repro <feedback-dir> [-at N] [-list] [-omniscient] [-emit-test <pkg>]")
		return 2
	}
	dir := fs.Arg(0)

	l, cfg, meta, err := feedback.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	intentCount := len(l.Intents)
	n := intentCount
	if *at < -1 || *at > intentCount {
		fmt.Fprintf(stderr, "repro: -at must be between 0 and %d (got %d)\n", intentCount, *at)
		return 2
	}
	if *at >= 0 {
		n = *at
	}

	// Every mode is evidence about the complete recording. Verify every
	// generated event and the final recorded head before presenting a
	// timeline or an -at prefix, and before -emit-test writes anything:
	// exit 0 promises a verified replay, and a test bootstrapped from an
	// unverified capture would reproduce nothing. ReplayTo alone can prove
	// only a prefix.
	full, err := replayCapture(l, cfg, meta)
	if err != nil {
		printDivergence(stdout, err)
		return 1
	}
	if *emit != "" {
		if err := emitTest(*emit, dir, meta, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	}
	if *list {
		return listIntents(l, cfg, stdout, stderr)
	}

	e := full
	if *at >= 0 {
		e, err = replay.ReplayTo(l, cfg, n)
		if err != nil {
			printDivergence(stdout, err)
			return 1
		}
	}

	printSummary(e, l, n, meta, *omniscient, stdout)
	return 0
}

// replayCapture verifies every recorded intent and event, then checks the
// redundant head stored by the feedback capture. replay.Replay validates the
// event stream's own chain; the explicit meta.Head check also catches a
// log.json whose head field alone was corrupted.
func replayCapture(l *events.Log, cfg rules.Config, meta feedback.Meta) (*rules.Engine, error) {
	e, err := replay.Replay(l, cfg)
	if err != nil {
		return e, err
	}
	if len(e.L.Events) != len(l.Events) {
		return e, fmt.Errorf("replay reached %d events, the recording has %d", len(e.L.Events), len(l.Events))
	}
	if want := l.HeadAt(len(l.Events)); meta.Head != want {
		return e, fmt.Errorf("log.json head %q does not match its own event chain %q", meta.Head, want)
	}
	if got := e.L.Head(); got != meta.Head {
		return e, fmt.Errorf("replayed head %s, recorded %s", got, meta.Head)
	}
	return e, nil
}

func printDivergence(w io.Writer, err error) {
	fmt.Fprintf(w, "DIVERGED: %v\n", err)
	var div *replay.Divergence
	if errors.As(err, &div) {
		fmt.Fprintln(w, "  a corpus change since the report was filed is a real, expected cause:")
		fmt.Fprintln(w, "  the snapshot replays against the corpus as it was at the report pin.")
	}
}

// printSummary writes the human summary at the replay point: the game
// facts, every seat's totals and battlefield, the stack, the pending
// decision and the last ~20 described log lines. It reads the engine's own
// pending decision (server-side shape) rather than the projected one —
// repro is a triage tool, not a seat.
func printSummary(e *rules.Engine, l *events.Log, n int, meta feedback.Meta, omniscient bool, stdout io.Writer) {
	g := e.G
	vis := view.Public
	if omniscient {
		vis = view.Omniscient
	}
	v := view.ProjectFor(g, e, view.NoSeat, vis, nil)
	v.Round = view.RoundOf(g, e.L.Events)

	fmt.Fprintf(stdout, "== feedback report %s (table %s, match %d) ==\n", meta.ID, meta.Table, meta.Match)
	if meta.Report != "" {
		fmt.Fprintf(stdout, "snapshot: %s\n", meta.Report)
	}
	seedNote := ""
	for _, u := range meta.TokensUnread {
		seedNote += "\n  token script unavailable at capture: " + u
	}
	fmt.Fprintf(stdout, "replayed %d of %d recorded intents (%d events); recorded head %s\n", n, len(l.Intents), len(e.L.Events), shortHead(meta.Head))
	if seedNote != "" {
		fmt.Fprintf(stdout, "warning:%s\n", seedNote)
	}
	phase := view.PhaseOf(g.Step)
	fmt.Fprintf(stdout, "turn %d, round %d, %s (step %s) — active: seat %d, priority: seat %d\n",
		g.Turn, v.Round, phase, g.Step, g.Active, g.Priority)
	if g.Over {
		if g.Draw {
			fmt.Fprintf(stdout, "game over: draw\n")
		} else {
			fmt.Fprintf(stdout, "game over: seat %d wins\n", g.Winner)
		}
	}
	fmt.Fprintln(stdout, "poison is not modelled in this build (no seat can have poison counters).")

	printSeats(stdout, v.Players, omniscient)
	if len(v.Stack) != 0 {
		fmt.Fprintln(stdout, "stack (top last):")
		for _, s := range v.Stack {
			fmt.Fprintf(stdout, "  - %s [%s, seat %d] %s\n", s.Name, s.Kind, s.Controller, s.Text)
		}
	}
	if d := e.Pending(); d != nil {
		fmt.Fprintf(stdout, "pending: seat %d %s: %s (%d options)\n", d.Player, d.Kind, d.Prompt, len(d.Options))
	}

	fmt.Fprintln(stdout, "last log lines:")
	tail := e.L.Events
	if len(tail) > 20 {
		tail = tail[len(tail)-20:]
	}
	for _, ev := range tail {
		if line := view.Describe(g, ev); line != "" {
			fmt.Fprintf(stdout, "  %s\n", line)
		}
	}
}

// printSeats writes every seat's totals and battlefield. Attachments print
// under the permanent they modify, so the reader sees the Aura/Equipment
// with its host rather than as two unrelated permanents.
//
// The attachment index is built across EVERY seat's battlefield before any
// seat prints: view.ProjectFor groups battlefields by controller, and an
// Aura is routinely controlled by one seat and attached to another seat's
// creature (a Pacifism on the opponent). Indexing per seat would suppress
// that Aura from its controller's section and never show it on the host.
// An attachment whose host is not on any battlefield in the view still
// prints as a standalone line naming the missing host.
func printSeats(stdout io.Writer, players []view.PlayerView, omniscient bool) {
	controller := map[state.ObjID]state.PlayerID{}
	onBattlefield := map[state.ObjID]bool{}
	for i := range players {
		for _, c := range players[i].Battlefield {
			onBattlefield[c.ID] = true
			controller[c.ID] = players[i].ID
		}
	}
	// attached indexes every seat's attachments under the id of the host
	// they modify. Indexing across ALL seats before any seat prints is
	// deliberate: view.ProjectFor groups battlefields by controller, and an
	// Aura is routinely controlled by one seat and attached to another
	// seat's creature (a Pacifism on the opponent), so a per-seat index
	// would suppress that Aura from its controller's section and never show
	// it on the host.
	attached := map[state.ObjID][]view.CardView{}
	for i := range players {
		for _, c := range players[i].Battlefield {
			if c.AttachedTo != 0 && onBattlefield[c.AttachedTo] {
				attached[c.AttachedTo] = append(attached[c.AttachedTo], c)
			}
		}
	}
	for i := range players {
		p := &players[i]
		fmt.Fprintf(stdout, "seat %d %s: life %d, library %d, hand %d, graveyard %d, exile %d\n",
			p.ID, p.Name, p.Life, p.LibrarySize, p.HandSize, p.GraveyardSize, len(p.Exile))
		if len(p.Command) != 0 {
			fmt.Fprintf(stdout, "  command zone: %s\n", cardNames(p.Command))
		}
		if omniscient && p.Hand != nil {
			fmt.Fprintf(stdout, "  hand: %s\n", cardNames(p.Hand))
		}
		for _, c := range p.Battlefield {
			if c.AttachedTo != 0 && onBattlefield[c.AttachedTo] {
				continue // printed with its host
			}
			line := fmt.Sprintf("  - %s", permanentDetail(c))
			if c.AttachedTo != 0 {
				line += fmt.Sprintf(", attached to #%d (not on the battlefield)", c.AttachedTo)
			}
			fmt.Fprintln(stdout, line)
			// Render each attached permanent in full — object id, P/T,
			// tapped, damage and counters — on its own nested line under the
			// host, rather than collapsing it to a name in an "attachments:"
			// suffix. A name-only suffix loses everything that distinguishes
			// two attachments: a tapped Equipment with counters or damage
			// would print identically to a fresh untapped one, and two
			// same-name attachments would be indistinguishable. The
			// cross-controller relationship is preserved: an Aura one seat
			// controls on another seat's permanent names the controlling
			// seat, so the reader can tell whose attachment it is.
			for _, a := range attached[c.ID] {
				nest := fmt.Sprintf("    - %s", permanentDetail(a))
				if controller[a.ID] != controller[c.ID] {
					nest += fmt.Sprintf(" (seat %d's)", controller[a.ID])
				}
				fmt.Fprintln(stdout, nest)
			}
		}
	}
}

// permanentDetail renders one permanent's public characteristics — name,
// object id, power/toughness, damage, tapped and counters — as the shared
// body of both a host's own line and an attached line. Reusing it for the
// nested attachment means an attachment carries the same detail a
// standalone permanent would, so nothing is lost when an Aura or Equipment
// is printed under its host instead of as its own battlefield entry.
func permanentDetail(c view.CardView) string {
	line := fmt.Sprintf("%s (#%d) %d/%d", c.Name, c.ID, c.Power, c.Toughness)
	if c.Damage != 0 {
		line += fmt.Sprintf(", damage %d", c.Damage)
	}
	if c.Tapped {
		line += ", tapped"
	}
	if ks := counterKinds(c.Counters); ks != "" {
		line += ", counters " + ks
	}
	return line
}

// cardNames renders a CardView list as "Name (#id)" comma-joined, in list
// order.
func cardNames(cs []view.CardView) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = fmt.Sprintf("%s (#%d)", c.Name, c.ID)
	}
	return strings.Join(parts, ", ")
}

// counterKinds renders a CardView's counters as "kind:n" joined, in sorted
// kind order (a map's range order must not reach output).
func counterKinds(cs map[string]int32) string {
	if len(cs) == 0 {
		return ""
	}
	kinds := make([]string, 0, len(cs))
	for k := range cs {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts := make([]string, len(kinds))
	for i, k := range kinds {
		parts[i] = fmt.Sprintf("%s:%d", k, cs[k])
	}
	return strings.Join(parts, ", ")
}

func shortHead(h string) string {
	if len(h) > 16 {
		return h[:16] + "…"
	}
	return h
}

// listIntents prints the intent timeline: for each recorded intent, its
// index, the seat that answered, the decision kind and prompt it answered,
// and the labels of the options the answer chose — everything a seat needs
// to find the moment a report is about. run has already verified the complete
// replay before calling this renderer. This second engine is driven forward
// one Submit at a time only so every decision can be rendered as it was asked;
// it is not the recording's validation boundary.
func listIntents(l *events.Log, cfg rules.Config, stdout, stderr io.Writer) int {
	e, err := replay.ReplayTo(l, cfg, 0)
	if err != nil {
		fmt.Fprintf(stdout, "DIVERGED before intent 0: %v\n", err)
		return 1
	}
	for i, it := range l.Intents {
		if e.G.Over {
			fmt.Fprintf(stdout, "%4d  (game already over; recording carries %d intents)\n", i, len(l.Intents))
			break
		}
		d := e.Pending()
		if d == nil {
			fmt.Fprintln(stderr, "repro: no decision pending at intent", i)
			return 1
		}
		what := choiceSummary(d, it)
		fmt.Fprintf(stdout, "%4d  seat %d  %s  %s\n", i, it.Player, d.Kind, what)
		if err := e.Submit(it); err != nil {
			fmt.Fprintf(stderr, "repro: intent %d rejected: %v\n", i, err)
			return 1
		}
	}
	if e.G.Over {
		if e.G.Draw {
			fmt.Fprintln(stdout, "game over: draw")
		} else {
			fmt.Fprintf(stdout, "game over: seat %d wins\n", e.G.Winner)
		}
	}
	return 0
}

// choiceSummary renders one intent's answer against the decision it
// answered: the chosen options' labels, or the raw indices when the
// decision does not offer labels for them.
func choiceSummary(d *decision.Decision, it decision.Intent) string {
	chosen := d.Chosen(it)
	if len(chosen) == 0 {
		return fmt.Sprintf("choices %v — %s", it.Choices, d.Prompt)
	}
	labels := make([]string, len(chosen))
	for i, o := range chosen {
		labels[i] = o.Label
	}
	return fmt.Sprintf("%s — %s", strings.Join(labels, "; "), d.Prompt)
}

// emitTest copies the snapshot into <pkg>/testdata/feedback/<id>/ and
// writes a failing test skeleton into <pkg> that loads the copy with
// feedback.EngineAt and ends in the TODO. <pkg> is a package directory:
// absolute, or relative to the repo root (the same root the corpus was
// found at).
//
// The skeleton is written as the package's EXTERNAL test package
// (`package <name>_test`, the `_test` suffix on the declaration, not the
// file name): feedback imports the engine's upstream packages — rules,
// replay, cards, state, events — so a skeleton declaring the internal
// package would make the target depend on feedback and feedback depend on
// the target, which the go toolchain rejects as an import cycle exactly
// when the natural target is an engine package (emitting into `rules` was
// the failure that motivated this). An external test file sits outside
// that cycle by construction, wherever the snapshot is emitted.
func emitTest(pkg, dir string, meta feedback.Meta, stdout io.Writer) error {
	root, err := feedback.Root()
	if err != nil {
		return err
	}
	pkgDir := pkg
	if !filepath.IsAbs(pkgDir) {
		pkgDir = filepath.Join(root, pkg)
	}
	pkgName, err := packageOf(pkgDir)
	if err != nil {
		return err
	}

	id := meta.ID
	san := sanitize(id)
	dst := filepath.Join(pkgDir, "testdata", "feedback", id)
	testPath := filepath.Join(pkgDir, "repro_feedback_"+san+"_test.go")
	// Refuse before writing anything: a skeleton someone filled in, or a
	// snapshot a test already pins, must survive a repeated emit byte for
	// byte.
	if _, err := os.Stat(testPath); err == nil {
		return fmt.Errorf("repro: %s already exists; not overwriting", testPath)
	}
	if ents, err := os.ReadDir(dst); err == nil && len(ents) > 0 {
		return fmt.Errorf("repro: %s already holds a snapshot; not overwriting", dst)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("repro: %w", err)
	}
	for _, f := range []string{"report.json", "match.json", "log.json", "view.json"} {
		src := filepath.Join(dir, f)
		raw, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue // view.json is optional; report.json too
			}
			return fmt.Errorf("repro: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dst, f), raw, 0o644); err != nil {
			return fmt.Errorf("repro: %w", err)
		}
	}

	body := fmt.Sprintf(`// Code generated by cmd/repro -emit-test from feedback report %s; DO NOT EDIT.
// It reproduces the state the report was filed at: the snapshot under
// testdata/feedback/%s/ is replayed to every recorded intent. Assert the
// reported behaviour at the TODO, then drop this notice.
//
// This is the target package's EXTERNAL test package (the _test suffix on
// the declaration): feedback imports the engine tier, so the internal test
// package would form an import cycle wherever the snapshot is emitted into
// an engine package. The external package may import feedback freely.
package %s_test

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil/feedback"
)

func TestFeedbackRepro%s(t *testing.T) {
	e := feedback.EngineAt(t, filepath.Join("testdata", "feedback", %q), -1)
	_ = e
	t.Fatal("TODO: assert the reported behaviour")
}
`, id, id, pkgName, san, id)
	if err := os.WriteFile(testPath, []byte(body), 0o644); err != nil {
		return fmt.Errorf("repro: %w", err)
	}
	fmt.Fprintf(stdout, "wrote %s and %s\n", dst, testPath)
	fmt.Fprintf(stdout, "next: go test <pkg> -run TestFeedbackRepro%s — it fails on the TODO; replace it with the assertion\n", san)
	return nil
}

// packageOf reads a package's name off its first non-test .go file. A
// directory with no Go source cannot receive a test skeleton.
func packageOf(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("repro: package %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.PackageClauseOnly)
		if err != nil {
			return "", fmt.Errorf("repro: package %s: %v", dir, err)
		}
		return f.Name.Name, nil
	}
	return "", fmt.Errorf("repro: package %s has no .go files to name it", dir)
}

// sanitize folds a report id into a legal Go identifier body: report ids
// are timestamps plus random hex, so this only ever has to drop the '-'.
func sanitize(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
