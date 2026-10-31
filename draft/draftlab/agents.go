package draftlab

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
)

// Role says which kind of agent an AgentRef names: the pickbot drafter or
// builder a draft seat plays, or the engine player policy a game seat
// plays.
type Role string

// The three agent roles.
const (
	RoleDrafter Role = "drafter"
	RoleBuilder Role = "builder"
	RolePlayer  Role = "player"
)

// unversioned is the effective version recorded for a built-in agent the
// lab's version table does not list. It resolves only from a ref with no
// explicit version.
const unversioned = "unversioned"

// builtinVersions is this build's version table for the built-in agents,
// keyed "role:name". It is read only after init (the map is never written
// at run time), so concurrent lookups are safe. A behaviour change to one
// of these agents bumps its version here and regenerates the
// agent_fingerprints golden (drafter/builder) — see TestBuiltinFingerprints.
var builtinVersions = map[string]string{
	"drafter:rating-colour":  "1",
	"drafter:best23":         "1",
	"builder:best23":         "1",
	"builder:rating-colour":  "1",
	"player:bot":             "1",
	"player:lethal-pressure": "1",
}

// BuiltinVersion is this build's version of a built-in agent; false when
// the table does not list it (such a name resolves only with Version "",
// recorded as "unversioned").
func BuiltinVersion(role Role, name string) (string, bool) {
	v, ok := builtinVersions[string(role)+":"+name]
	return v, ok
}

// AgentRef names an agent by registry name and frozen version ("bot@1").
// Version "" accepts whatever this build carries; the resolved ref always
// carries the effective version.
type AgentRef struct {
	Name    string
	Version string
}

// ParseAgent parses "name" or "name@version". An empty name or — where an
// @ is present — an empty version is an error, and so is a second @.
func ParseAgent(s string) (AgentRef, error) {
	name, version, hasAt := strings.Cut(s, "@")
	if hasAt && strings.Contains(version, "@") {
		return AgentRef{}, fmt.Errorf("draftlab: agent %q: more than one @", s)
	}
	if name == "" {
		return AgentRef{}, fmt.Errorf("draftlab: agent %q: empty name", s)
	}
	if hasAt && version == "" {
		return AgentRef{}, fmt.Errorf("draftlab: agent %q: empty version", s)
	}
	return AgentRef{Name: name, Version: version}, nil
}

// String spells the ref the way ParseAgent reads it back: "name@version",
// or "name" when the version is empty.
func (a AgentRef) String() string {
	if a.Version == "" {
		return a.Name
	}
	return a.Name + "@" + a.Version
}

// The agent-resolution errors. Use errors.Is on them: the resolution
// methods wrap each with the agent named and, for ErrUnknownAgent, the
// known names.
var (
	ErrUnknownAgent = errors.New("draftlab: unknown agent")
	ErrVersion      = errors.New("draftlab: agent version not available in this build")
	ErrDecklistBot  = errors.New("draftlab: player reads every seat's real decklist")
)

// NamedDrafter is one resolved drafter: its effective ref and the fresh
// pickbot drafter instance a Draft seat plays.
type NamedDrafter struct {
	Ref     AgentRef
	Drafter pickbot.Drafter
}

// NamedBuilder is one resolved builder: its effective ref and the fresh
// pickbot builder instance a Build runs.
type NamedBuilder struct {
	Ref     AgentRef
	Builder pickbot.Builder
}

// NamedPlayer is one resolved player policy. New builds a fresh seat for
// one game from the seat's own seed; it may be called from several
// goroutines at once. ReadsDecklists marks an Env player
// — refused by Player unless Options.AllowDecklistBots; with it, the game driver runs it through the
// bench Env driver.
type NamedPlayer struct {
	Ref AgentRef
	New func(seed uint64) (seat.Seat, error)
	// ReadsDecklists reports whether the policy reads every seat's real
	// decklist (bots.Info.Env). A caller-built NamedPlayer declares it, and
	// the game driver refuses a true one without the opt-in.
	ReadsDecklists bool
}

// resolveVersion applies the name@version rule for one role: a ref with no
// version resolves to the table's version (or "unversioned" when the table
// does not list the name); an explicit version must equal the table's.
func resolveVersion(role Role, name, version string) (string, error) {
	listed, listedOK := BuiltinVersion(role, name)
	switch {
	case version == "":
		if !listedOK {
			return unversioned, nil
		}
		return listed, nil
	case !listedOK:
		return "", fmt.Errorf("%w: %s %q is unversioned in this build (it accepts no explicit version)", ErrVersion, role, name)
	case version != listed:
		return "", fmt.Errorf("%w: %s %q: this build carries version %q, not %q", ErrVersion, role, name, listed, version)
	}
	return version, nil
}

// lookup is the pickbot CardLookup over the lab's registry: a name the
// corpus does not carry has no facts.
func (l *Lab) lookup() pickbot.CardLookup {
	return func(name string) *cards.Card {
		c, _ := l.reg.Lookup(name)
		return c
	}
}

// Drafter resolves a pickbot drafter by ref: a fresh instance per call with
// the lab's card-facts lookup injected, carrying the effective version.
func (l *Lab) Drafter(ref AgentRef) (NamedDrafter, error) {
	if !pickbotHas(ref.Name) {
		return NamedDrafter{}, fmt.Errorf("%w: drafter %q (known: %s)", ErrUnknownAgent, ref.Name, pickbot.Names())
	}
	v, err := resolveVersion(RoleDrafter, ref.Name, ref.Version)
	if err != nil {
		return NamedDrafter{}, err
	}
	d, _, ok := pickbot.New(ref.Name, l.lookup())
	if !ok {
		return NamedDrafter{}, fmt.Errorf("%w: drafter %q (known: %s)", ErrUnknownAgent, ref.Name, pickbot.Names())
	}
	return NamedDrafter{Ref: AgentRef{Name: ref.Name, Version: v}, Drafter: d}, nil
}

// Builder resolves a pickbot builder by ref: a fresh instance per call with
// the lab's card-facts lookup injected, carrying the effective version.
func (l *Lab) Builder(ref AgentRef) (NamedBuilder, error) {
	if !pickbotHas(ref.Name) {
		return NamedBuilder{}, fmt.Errorf("%w: builder %q (known: %s)", ErrUnknownAgent, ref.Name, pickbot.Names())
	}
	v, err := resolveVersion(RoleBuilder, ref.Name, ref.Version)
	if err != nil {
		return NamedBuilder{}, err
	}
	_, b, ok := pickbot.New(ref.Name, l.lookup())
	if !ok {
		return NamedBuilder{}, fmt.Errorf("%w: builder %q (known: %s)", ErrUnknownAgent, ref.Name, pickbot.Names())
	}
	return NamedBuilder{Ref: AgentRef{Name: ref.Name, Version: v}, Builder: b}, nil
}

// Player resolves an engine bot policy by ref. The returned New builds a
// fresh seat per call — bots.New with AutoPayMana and the lab's registry,
// then the limited London mulligan heuristic for a *heuristic.Bot (the rule
// cmd/draftsim's game driver applies) — and is safe to call concurrently.
//
// An Env policy (bots.ReadsDecklists) is refused with ErrDecklistBot unless
// Options.AllowDecklistBots is set, by design. With the
// opt-in it is admitted and its NamedPlayer declares ReadsDecklists: true, so
// the game driver runs it through the bench Env driver (Hooks.EnvSeats) and
// manifests can mark the result. NewManifest sets
// Manifest.AllowDecklistBots when Options.AllowDecklistBots and any game's
// players include such a ref; the detection is bots.ReadsDecklists over each
// ref's name (strip the "@version", bots.Normalize).
func (l *Lab) Player(ref AgentRef) (NamedPlayer, error) {
	name, err := bots.Normalize(ref.Name)
	if err != nil {
		return NamedPlayer{}, fmt.Errorf("%w: %v", ErrUnknownAgent, err)
	}
	v, err := resolveVersion(RolePlayer, ref.Name, ref.Version)
	if err != nil {
		return NamedPlayer{}, err
	}
	readsDecklists := bots.ReadsDecklists(name)
	if readsDecklists && !l.opts.AllowDecklistBots {
		return NamedPlayer{}, fmt.Errorf("%w: policy %q reads every seat's real decklist (bots.Info.Env); the lab's AllowDecklistBots opt-in is off", ErrDecklistBot, name)
	}
	return NamedPlayer{
		Ref: AgentRef{Name: ref.Name, Version: v},
		New: func(seed uint64) (seat.Seat, error) {
			st, err := bots.New(name, bots.Options{
				Seed:        seed,
				AutoPayMana: true,
				Deps:        bots.Deps{Cards: l.reg},
			})
			if err != nil {
				return nil, fmt.Errorf("draftlab: seat policy %q: %w", name, err)
			}
			if b, ok := st.(*heuristic.Bot); ok {
				b.WithMulligan(botpolicy.MulliganLands)
			}
			return st, nil
		},
		ReadsDecklists: readsDecklists,
	}, nil
}

// pickbotHas reports whether name is registered in the pickbot registry.
func pickbotHas(name string) bool {
	for _, n := range pickbot.Names() {
		if n == name {
			return true
		}
	}
	return false
}
