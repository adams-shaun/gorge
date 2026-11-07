package searchseat_test

// The paired-Commander observation floor: a search seat's feed captures a
// whole two-seat Commander game, and the honest root it builds at a Commander
// decision keeps each player's commanders in their live zones and their cast
// counts. Constructed behaviour is pinned by the existing tests.

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	ss "github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// commanderRootConfig is a two-seat Commander game: the decks' resolved
// cards, each file's commander indices, the format's 40 life and the seed
// the root tests share.
func commanderRootConfig(t *testing.T, a, b string) rules.Config {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	da, err := testutil.LoadRepoDeck(reg, a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := testutil.LoadRepoDeck(reg, b)
	if err != nil {
		t.Fatal(err)
	}
	fa := testutil.RepoDeckFile(t, a)
	fb := testutil.RepoDeckFile(t, b)
	return rules.Config{
		Seed:         9001,
		Names:        []string{a, b},
		Decks:        [][]*cards.Card{da, db},
		Commanders:   [][]int{fa.CommanderIndices(), fb.CommanderIndices()},
		Tokens:       reg.Tokens,
		Format:       rules.FormatCommander,
		StartingLife: 40,
	}
}

// commanderBelief is a closed-list belief over every other Commander repo
// deck, so it always includes the opponent's own list without being just
// that list.
func commanderBelief(t *testing.T, reg *cards.Registry, actor string) *searchprobe.Belief {
	t.Helper()
	var decks [][]*cards.Card
	for _, name := range testutil.RepoDeckNames() {
		if name == actor {
			continue
		}
		f, err := testutil.LoadRepoDeckFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(f.CommanderNames()) == 0 {
			continue
		}
		d, err := testutil.LoadRepoDeck(reg, name)
		if err != nil {
			t.Fatal(err)
		}
		decks = append(decks, d)
	}
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// commanderNames is every roster name in the game, keyed for a frame scan.
func commanderNames(t *testing.T, e *rules.Engine) map[string]bool {
	t.Helper()
	names := make(map[string]bool)
	for i := range e.G.Players {
		for _, id := range e.G.Players[i].Commanders {
			o := e.G.Obj(id)
			if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
				t.Fatalf("fixture: commander %d has no card", id)
			}
			names[o.Card.Faces[0].Name] = true
		}
	}
	if len(names) == 0 {
		t.Fatal("fixture: the game has no commanders")
	}
	return names
}

// commanderRefs folds f's newly introduced identities into refs, the
// accumulated observation references of every commander the feed saw. A
// commander introduced in the command zone keeps that reference when it is
// later cast, so a frame that merely DISPLAYS it has no new identity for it.
func commanderRefs(refs map[uint32]bool, f searchprobe.Frame, commanders map[string]bool) bool {
	introduced := false
	for _, identity := range f.Identities {
		if commanders[identity.Name] {
			refs[identity.ID] = true
			introduced = true
		}
	}
	return introduced
}

// frameShowsCommanderOutsideCommand reports whether f displays an
// accumulated commander reference anywhere but the command zone.
func frameShowsCommanderOutsideCommand(f searchprobe.Frame, refs map[uint32]bool) bool {
	if len(refs) == 0 {
		return false
	}
	for _, p := range f.Board.Players {
		for _, zone := range [][]searchprobe.BoardCard{p.Battlefield, p.Hand, p.Graveyard, p.Exile} {
			for _, c := range zone {
				if refs[c.ID] {
					return true
				}
			}
		}
	}
	for _, s := range f.Board.Stack {
		if refs[s.ID] || refs[s.Source] {
			return true
		}
	}
	return false
}

// TestFeedObservesAWholeCommanderGame drives bot-vs-bot Commander from the
// first decision to the game's end (or the 20000-step cap): seat 0's feed
// captures every decision of both players without stopping, and the stream
// carries a frame in which a commander has left the command zone.
func TestFeedObservesAWholeCommanderGame(t *testing.T) {
	cfg := commanderRootConfig(t, "foundations-tramplesaurus-rex", "foundations-wretched-ranks")
	e := rules.New(cfg)
	e.Advance()
	feed := ss.NewFeed(0)
	board := botpolicy.NewBoard(2)
	rngs := searchprobe.BotRandoms(cfg.Seed, len(cfg.Names))
	commanders := commanderNames(t, e)
	refs := make(map[uint32]bool)
	identities, outside := false, false
	steps := 0
	for ; steps < 20000 && !e.G.Over; steps++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision at step %d", steps)
		}
		f, ok := feed.Observe(e)
		if !ok {
			t.Fatalf("feed stopped at step %d: %s", steps, feed.StopReason())
		}
		identities = commanderRefs(refs, f, commanders) || identities
		outside = outside || frameShowsCommanderOutsideCommand(f, refs)
		in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
		if d.Player == 0 {
			if err := feed.RecordAnswer(d, in); err != nil {
				t.Fatalf("record answer at step %d: %v", steps, err)
			}
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("submit at step %d: %v", steps, err)
		}
	}
	if !feed.Live() {
		t.Fatalf("feed stopped: %s", feed.StopReason())
	}
	if !identities {
		t.Fatal("no frame introduced a commander; the outside-command check never had a subject")
	}
	if !outside {
		t.Fatalf("no frame showed a commander outside the command zone after %d steps (%d frames, turn %d, over %v)", steps, feed.Frames(), e.G.Turn, e.G.Over)
	}
	if _, err := feed.Known(); err != nil {
		t.Fatalf("known-card projection: %v", err)
	}
	t.Logf("observed %d decisions over %d turns (over=%v)", feed.Frames(), e.G.Turn, e.G.Over)
}

// TestHonestRootKeepsACommanderObservation is TestHonestRootKeepsTheObservation
// at a Commander table, plus the commander bookkeeping the CR 903.8 tax needs:
// every world keeps each player's commanders in the live zones and the live
// CmdCasts.
func TestHonestRootKeepsACommanderObservation(t *testing.T) {
	p := newRootPosition(t, commanderRootConfig(t, "foundations-tramplesaurus-rex", "foundations-wretched-ranks"))
	last := p.feed.LastFrame()
	if last.Decision == nil {
		t.Fatal("fixture: the feed's last frame carries no decision")
	}
	root, reason := ss.HonestRoot(p.setup, p.feed, p.e, [2]uint64{7, 11})
	if reason != "" {
		t.Fatalf("HonestRoot refused: %s", reason)
	}
	if root == nil {
		t.Fatal("HonestRoot returned a nil engine with no reason")
	}
	if root == p.e {
		t.Fatal("HonestRoot returned the live engine itself")
	}
	got, err := p.feed.Collector().Clone().Capture(root, nil)
	if err != nil {
		t.Fatalf("capture the root: %v", err)
	}
	if got.Board.Sum != last.Board.Sum {
		t.Fatal("the root's board differs from the feed's last frame")
	}
	if !reflect.DeepEqual(got.Decision, last.Decision) {
		t.Fatal("the root's decision differs from the feed's last frame")
	}
	if sameHiddenWorld(t, root, p.e) {
		t.Fatal("the root kept every real hidden card; the observation check is vacuous")
	}
	for pl := state.PlayerID(0); int(pl) < len(p.e.G.Players); pl++ {
		live, redealt := p.e.G.Players[pl], root.G.Players[pl]
		if len(live.Commanders) == 0 {
			t.Fatalf("fixture: player %d has no commanders", pl)
		}
		if !reflect.DeepEqual(redealt.Commanders, live.Commanders) {
			t.Fatalf("player %d commanders %v, want %v", pl, redealt.Commanders, live.Commanders)
		}
		if !reflect.DeepEqual(redealt.CmdCasts, live.CmdCasts) {
			t.Fatalf("player %d CmdCasts %v, want %v", pl, redealt.CmdCasts, live.CmdCasts)
		}
		for _, id := range live.Commanders {
			lo, ro := p.e.G.Obj(id), root.G.Obj(id)
			if lo == nil || ro == nil {
				t.Fatalf("player %d commander %d is missing from an engine", pl, id)
			}
			if ro.Zone != lo.Zone {
				t.Fatalf("player %d commander %d is in zone %v, want %v", pl, id, ro.Zone, lo.Zone)
			}
		}
	}
}

// TestHonestRootClosedInACommanderGame takes the same Commander position
// through setup.Closed: the root keeps the opponent's live commander in its
// live zone while every opponent hand and library card it deals comes from
// the belief.
func TestHonestRootClosedInACommanderGame(t *testing.T) {
	cfg := commanderRootConfig(t, "foundations-tramplesaurus-rex", "foundations-wretched-ranks")
	p := newRootPosition(t, cfg)
	reg := testutil.CorpusRegistry(t)
	belief := commanderBelief(t, reg, cfg.Names[0])
	if belief.Decks() < 2 {
		t.Fatalf("fixture: the belief is the opponent's list alone (%d deck)", belief.Decks())
	}
	setup := p.setup.Closed(0, belief)
	root, reason := ss.HonestRoot(setup, p.feed, p.e, [2]uint64{13, 17})
	if reason != "" {
		t.Fatalf("HonestRoot refused a closed Commander position: %s", reason)
	}
	if root == nil {
		t.Fatal("HonestRoot returned a nil engine with no reason")
	}
	liveCmd := p.e.G.Players[1].Commanders
	if len(liveCmd) == 0 {
		t.Fatal("fixture: the opponent has no commanders")
	}
	if !reflect.DeepEqual(root.G.Players[1].Commanders, liveCmd) {
		t.Fatalf("opponent commanders %v, want the live %v", root.G.Players[1].Commanders, liveCmd)
	}
	for _, id := range liveCmd {
		lo, ro := p.e.G.Obj(id), root.G.Obj(id)
		if lo == nil || ro == nil {
			t.Fatalf("opponent commander %d is missing from an engine", id)
		}
		if ro.Zone != lo.Zone {
			t.Fatalf("opponent commander %d is in zone %v, want the live %v", id, ro.Zone, lo.Zone)
		}
		if ro.Card == nil || lo.Card == nil || ro.Card.Faces[0].Name != lo.Card.Faces[0].Name {
			t.Fatalf("opponent commander %d is not the live commander", id)
		}
	}
	hidden := append(rootNames(t, root, root.G.Zone(state.ZHand, 1)), rootNames(t, root, root.G.Zone(state.ZLibrary, 1))...)
	if len(hidden) == 0 {
		t.Fatal("fixture: the root has no opponent hidden cards to check")
	}
	for _, name := range hidden {
		if _, ok := belief.Ord(name); !ok {
			t.Fatalf("opponent hidden card %q is not in the belief", name)
		}
	}
}
