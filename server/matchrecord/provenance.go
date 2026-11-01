package matchrecord

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
)

const module = "github.com/adams-shaun/gorge"

// GorgeCommit names the gorge build this binary was linked from, read from
// the Go toolchain's build stamp (runtime/debug.ReadBuildInfo):
//
//   - in a binary built from the gorge module itself, the vcs.revision
//     stamp, suffixed "+dirty" when vcs.modified says the tree had local
//     changes;
//   - in a binary of another module that depends on gorge, the gorge dependency's module version, whose
//     pseudo-version carries the pinned commit.
//
// It is empty when no stamp exists (`go test` binaries carry no vcs
// settings). The stamp is the toolchain's: a binary built inside a git
// worktree nested in another checkout (gorge's .worktrees/<id>) is stamped
// with the OUTER checkout's revision, because the go command finds the
// outer .git directory first. Provenance only; nothing replays from it.
func GorgeCommit() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if bi.Main.Path == module {
		rev, dirty := "", false
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if rev != "" && dirty {
			rev += "+dirty"
		}
		return rev
	}
	for _, d := range bi.Deps {
		if d.Path != module {
			continue
		}
		if d.Replace != nil {
			return d.Replace.Path + "@" + d.Replace.Version
		}
		return d.Version
	}
	return ""
}

// ForgeRef reads the card-corpus pin from the corpus directory's
// cards.lock (the `.cards` directory `forgec fetch` wrote): the fetched
// commit, or the requested ref when the lock recorded no commit. Empty when
// the directory has no readable lock.
func ForgeRef(cardsDir string) string {
	l, err := cards.ReadLock(cardsDir)
	if err != nil || l == nil {
		return ""
	}
	if l.Commit != "" {
		return l.Commit
	}
	return l.Ref
}

// Sink is where a game driver sends the record of each game it finishes
// (testkit/bench's Hooks.Record). Where records land and which games are
// sampled are the caller's concern: Put decides both.
type Sink struct {
	// Producer is stamped into every record. An empty GorgeCommit is filled
	// with GorgeCommit().
	Producer Producer
	// Put receives each record. The record references the driver's live
	// log, so Put must finish with it (write it, or copy it) before it
	// returns.
	Put func(r *Record) error
}

// Emit builds the record of the game cfg configured and l logged and hands
// it to Put. A nil Sink or Put records nothing.
func (s *Sink) Emit(cfg rules.Config, l *events.Log) error {
	return s.EmitExplain(cfg, l, nil)
}

// EmitExplain is Emit with the record's per-intent decision notes
// (Record.Explain): explain is empty, or holds one entry per logged intent,
// null where nothing was noted. Its entries' shape is bots/explain.Note,
// which this package never decodes.
func (s *Sink) EmitExplain(cfg rules.Config, l *events.Log, explain []json.RawMessage) error {
	if s == nil || s.Put == nil {
		return nil
	}
	if len(explain) != 0 && len(explain) != len(l.Intents) {
		return fmt.Errorf("matchrecord: %d explain notes for %d intents", len(explain), len(l.Intents))
	}
	p := s.Producer
	if p.GorgeCommit == "" {
		p.GorgeCommit = GorgeCommit()
	}
	p.Policies = append([]string(nil), p.Policies...)
	r, err := New(cfg, l, p)
	if err != nil {
		return err
	}
	r.Explain = explain
	return s.Put(r)
}
