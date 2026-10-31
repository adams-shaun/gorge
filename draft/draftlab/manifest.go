package draftlab

import (
	"runtime/debug"
	"sort"
	"sync"
)

// manifestFormat is the Manifest Format value.
const manifestFormat = "gorge-draftlab/1"

// RoleRef pairs an agent's role with the ref that named it: the
// "role:name@version" string a manifest's Agents list carries.
type RoleRef struct {
	Role Role
	Ref  AgentRef
}

// String spells the ref "role:name@version" (or "role:name" when the
// version is empty).
func (r RoleRef) String() string {
	return string(r.Role) + ":" + r.Ref.String()
}

// Manifest is one lab run's provenance: everything a training program needs
// to reproduce the run byte-for-byte (gorge_revision, the resolved agent
// refs, the engine watchdogs and mulligan round) and to judge the numbers
// (the game, draw and stall counts). Run knobs (Workers, Verify) never
// appear: they change how a run executes, never what a result is.
type Manifest struct {
	Format        string `json:"format"`
	GorgeRevision string `json:"gorge_revision"`
	// Kind is the run shape: "games", "tournament", "gauntlet" or "event".
	Kind string `json:"kind"`
	Seed uint64 `json:"seed"`
	// Agents is every resolved agent ref the run played, spelled
	// "role:name@version", sorted unique.
	Agents []string `json:"agents"`
	// AllowDecklistBots mirrors Options.AllowDecklistBots (omitted false).
	AllowDecklistBots bool `json:"allow_decklist_bots,omitempty"`
	// MaxTurns, MaxIntents and Mulligans are the effective engine watchdogs
	// and London mulligan round (the defaults filled in).
	MaxTurns   int `json:"max_turns"`
	MaxIntents int `json:"max_intents"`
	Mulligans  int `json:"mulligans"`
	// Games, Draws and Stalls count the run's played games: Draws is the
	// games with no winner (a watchdog stall counts as a draw) and Stalls
	// is the games that ended at a turn or intent cap.
	Games  int `json:"games"`
	Draws  int `json:"draws"`
	Stalls int `json:"stalls"`
	// The event fields; a run that is not an event leaves them unset.
	Set       string `json:"set,omitempty"`
	Collation string `json:"collation,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Structure string `json:"structure,omitempty"`
	Seats     int    `json:"seats,omitempty"`
}

// NewManifest fills a manifest for kind ("games", "tournament",
// "gauntlet" or "event") over seed, agents and games. The event fields are
// left unset: the callers that are events fill them after.
func (l *Lab) NewManifest(kind string, seed uint64, agents []RoleRef, games []GameResult) Manifest {
	o := l.Options()
	m := Manifest{
		Format:        manifestFormat,
		GorgeRevision: gorgeRevision(),
		Kind:          kind,
		Seed:          seed,
		Agents:        roleRefStrings(agents),
		MaxTurns:      o.MaxTurns,
		MaxIntents:    o.MaxIntents,
		Mulligans:     o.Mulligans,
		Games:         len(games),
	}
	if o.AllowDecklistBots {
		m.AllowDecklistBots = true
	}
	for _, g := range games {
		if g.Winner < 0 {
			m.Draws++
		}
		if g.Stall != "" {
			m.Stalls++
		}
	}
	return m
}

// roleRefStrings spells RoleRefs "role:name@version", sorted unique; the
// slice is never nil.
func roleRefStrings(agents []RoleRef) []string {
	seen := make(map[string]bool, len(agents))
	out := make([]string, 0, len(agents))
	for _, a := range agents {
		s := a.String()
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// playerRoleRefs lists each entrant's player as a RoleRef, in entrant
// order (roleRefStrings sorts and dedupes).
func playerRoleRefs(entrants []Entrant) []RoleRef {
	out := make([]RoleRef, len(entrants))
	for i, e := range entrants {
		out[i] = RoleRef{Role: RolePlayer, Ref: e.Player.Ref}
	}
	return out
}

// gorgeRevision is the revision string a manifest's GorgeRevision carries:
// the build's vcs.revision ("+dirty" when the build carried uncommitted
// changes), the module version when the build named one, else "devel".
// It is computed once.
var gorgeRevision = sync.OnceValue(func() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		rev, dirty := "", false
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if rev != "" {
			if dirty {
				rev += "+dirty"
			}
			return rev
		}
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "devel"
})
