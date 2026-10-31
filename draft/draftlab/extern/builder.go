package extern

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// Builder wraps the agent as a draftlab.NamedBuilder: its adapter turns a
// finished pool into a deck over the wire, falling back to the lab's
// best23@1 builder on every refusal (and, under FailFallback, on every
// session failure). The agent's hello must serve the "build" kind; ErrRole
// otherwise.
func (a *Agent) Builder(l *draftlab.Lab) (draftlab.NamedBuilder, error) {
	if !servesKind(a.hello, wire.KindBuild) {
		return draftlab.NamedBuilder{}, fmt.Errorf("%w: agent %q serves %v, not %q",
			ErrRole, a.hello.Agent, a.hello.Kinds, wire.KindBuild)
	}
	fb, err := l.Builder(draftlab.AgentRef{Name: "best23", Version: "1"})
	if err != nil {
		return draftlab.NamedBuilder{}, fmt.Errorf("extern: the build fallback builder: %w", err)
	}
	return draftlab.NamedBuilder{
		Ref: a.Ref(),
		Builder: &extBuilder{
			a: a, lab: l,
			fallback: fb.Builder,
		},
	}, nil
}

// extBuilder is one Agent's build adapter. It implements pickbot.Builder
// and draftlab.FallibleBuilder; buildNamed answers through BuildErr.
type extBuilder struct {
	a        *Agent
	lab      *draftlab.Lab
	fallback pickbot.Builder
}

// Build turns a finished pool into a deck; an error (a refusal under
// StrictRefusals, or a session failure under FailError) is answered with
// the fallback builder's deck.
func (b *extBuilder) Build(set string, pool []limited.CardRef) build.Deck {
	d, err := b.BuildErr(set, pool)
	if err != nil {
		return b.fallback.Build(set, pool)
	}
	return d
}

// BuildErr turns a finished pool into a deck over the wire: the item
// carries the seat's pool as DraftCards. A deck build.Validate refuses, an
// error answer, a session failure (OnFailure) or — under StrictRefusals —
// any refusal is the error; otherwise the fallback builder answers.
func (b *extBuilder) BuildErr(set string, pool []limited.CardRef) (build.Deck, error) {
	item := wire.Item{
		Kind:     wire.KindBuild,
		Seed:     wire.Seed53(buildSeed(set, pool)),
		Set:      set,
		Pool:     draftCards(b.lab, pool),
		DeckSize: 40,
	}
	ans, err := b.a.call(context.Background(), nil, item)
	if err != nil {
		var out build.Deck
		err = b.a.failed(err, wire.KindBuild, func() { out = b.fallback.Build(set, pool) })
		return out, err
	}
	if ans.Error != nil {
		var out build.Deck
		if err := b.a.refused(wire.KindBuild, itemErrorReason(ans.Error), func() { out = b.fallback.Build(set, pool) }); err != nil {
			return build.Deck{}, err
		}
		return out, nil
	}
	if ans.Deck == nil {
		var out build.Deck
		if err := b.a.refused(wire.KindBuild, "the answer is not a deck", func() { out = b.fallback.Build(set, pool) }); err != nil {
			return build.Deck{}, err
		}
		return out, nil
	}
	d := build.Deck{Main: ans.Deck.Main, Basics: ans.Deck.Basics}
	if err := build.Validate(pool, d); err != nil {
		var out build.Deck
		if err := b.a.refused(wire.KindBuild, err.Error(), func() { out = b.fallback.Build(set, pool) }); err != nil {
			return build.Deck{}, err
		}
		return out, nil
	}
	return d, nil
}

// buildSeed folds the set and the pool's ids into one rng word, so the
// same pool always reaches the agent with the same seed.
func buildSeed(set string, pool []limited.CardRef) uint64 {
	ids := make([]string, len(pool))
	for i, c := range pool {
		ids[i] = c.ID
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(set))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.Join(ids, "\x00")))
	return h.Sum64()
}
