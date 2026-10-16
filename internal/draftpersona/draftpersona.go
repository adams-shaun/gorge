// Package draftpersona turns a server's drafter flag (cmd/gorged
// -event-drafter) into a registered pickbot persona that draft-event bot
// seats draft with.
//
// A spec is one of:
//
//	rating-colour        the stock 17Lands-rated drafter (registered already)
//	net:WEIGHTS.dpkw[@TABLE.json]
//	                     the draft-RL policy network (limited/draftnet), argmax;
//	                     the table defaults to table.json beside the weights
//	                     and must be the one the weights were trained against
//	ovr:VALUES.json#FIELD
//	                     rating-colour scoring with a ratings override (a
//	                     cmd/cardval value table), the draft-RL S0 drafter
//
// A trained drafter knows one set (the net's table set; FDN for an
// override). On any other set's pod the persona drafts with rating-colour,
// so an event on another set still drafts sensibly. Every persona builds
// with best23 on the embedded ratings, the builder the gate held fixed.
//
// Privacy: a pickbot Drafter is handed only its own seat's PickView (the
// pod's legal view: its pack, its picks, the packs it saw), and these
// drafters are pure functions of that view and static set data.
package draftpersona

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/limited/draftfeat"
	"github.com/adams-shaun/gorge/limited/draftnet"
	"github.com/adams-shaun/gorge/limited/pickbot"
)

// Default is the stock persona.
const Default = "rating-colour"

// Persona names a registered trained drafter takes.
const (
	NetPersona = "draft-net"
	OvrPersona = "draft-values"
)

// setGuard drafts with d on set's pods and with fallback elsewhere.
type setGuard struct {
	set         string
	d, fallback pickbot.Drafter
}

func (g *setGuard) Pick(v pickbot.PickView, r *rand.Rand) string {
	if v.Set == g.set {
		return g.d.Pick(v, r)
	}
	return g.fallback.Pick(v, r)
}

// Register parses spec, loads what it names and registers it as a pickbot
// persona, returning the persona name to hand eventhost as its default
// drafter. "" and "rating-colour" register nothing.
func Register(spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	switch {
	case spec == "" || spec == Default:
		return Default, nil
	case strings.HasPrefix(spec, "net:"):
		n, t, err := LoadNet(strings.TrimPrefix(spec, "net:"))
		if err != nil {
			return "", err
		}
		RegisterNet(NetPersona, n, t)
		return NetPersona, nil
	case strings.HasPrefix(spec, "ovr:"):
		path, field, ok := strings.Cut(strings.TrimPrefix(spec, "ovr:"), "#")
		if !ok || path == "" || field == "" {
			return "", fmt.Errorf("drafter %q: want ovr:VALUES.json#FIELD", spec)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		const set = "FDN"
		o, err := pickbot.ParseRatingsOverride(b, path, field, set)
		if err != nil {
			return "", err
		}
		pickbot.Register(OvrPersona,
			func(l pickbot.CardLookup) pickbot.Drafter {
				return &setGuard{set: set, d: &pickbot.RatingColour{Lookup: l, Ratings: o}, fallback: &pickbot.RatingColour{Lookup: l}}
			},
			func(l pickbot.CardLookup) pickbot.Builder { return &pickbot.Best23{Lookup: l} })
		return OvrPersona, nil
	}
	return "", fmt.Errorf("drafter %q: want rating-colour, net:WEIGHTS.dpkw[@TABLE.json] or ovr:VALUES.json#FIELD", spec)
}

// LoadNet reads WEIGHTS[@TABLE] and checks the weights against the table.
func LoadNet(arg string) (*draftnet.Net, *draftfeat.SetTable, error) {
	wpath, tpath, ok := strings.Cut(arg, "@")
	if !ok {
		tpath = filepath.Join(filepath.Dir(wpath), "table.json")
	}
	wb, err := os.ReadFile(wpath)
	if err != nil {
		return nil, nil, err
	}
	n, err := draftnet.Load(wb)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", wpath, err)
	}
	tb, err := os.ReadFile(tpath)
	if err != nil {
		return nil, nil, err
	}
	t, err := draftfeat.DecodeSetTable(tb)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", tpath, err)
	}
	if err := n.CheckTable(t); err != nil {
		return nil, nil, fmt.Errorf("%s with %s: %w", wpath, tpath, err)
	}
	return n, t, nil
}

// RegisterNet registers name as the net drafter (argmax) on t's set, with
// rating-colour elsewhere and best23 building. The net is immutable and
// shared; each persona instance gets its own NetDrafter.
func RegisterNet(name string, n *draftnet.Net, t *draftfeat.SetTable) {
	pickbot.Register(name,
		func(l pickbot.CardLookup) pickbot.Drafter {
			return &setGuard{set: t.Set, d: &pickbot.NetDrafter{Net: n, Table: t}, fallback: &pickbot.RatingColour{Lookup: l}}
		},
		func(l pickbot.CardLookup) pickbot.Builder { return &pickbot.Best23{Lookup: l} })
}
