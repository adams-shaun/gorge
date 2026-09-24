package effects

import "github.com/adams-shaun/gorge/state"

// Test-only seams for the compiled-filter equivalence test, which lives in
// the external effects_test package so it can play real games through rules.

// MatchesObjectTextOracle is the textual filter evaluator the compiled form
// replaced on the hot path.
func MatchesObjectTextOracle(g *state.Game, spec string, o *state.Object, sc SpecContext) bool {
	return matchesObjectText(g, spec, o, sc)
}

// CompiledSpecForTest compiles spec without the process cache and returns
// its ordinary and off-battlefield zone matchers.
func CompiledSpecForTest(spec string) (
	match func(*state.Game, *state.Object, SpecContext) bool,
	zone func(*state.Game, *state.Object, SpecContext, state.Zone) bool) {
	cs := compileSpec(spec)
	return func(g *state.Game, o *state.Object, sc SpecContext) bool { return compiledMatch(cs, g, o, &sc) },
		func(g *state.Game, o *state.Object, sc SpecContext, z state.Zone) bool {
			return compiledMatchZone(cs, g, o, &sc, z)
		}
}

// MatchesObjectCompiledCached evaluates spec through the process cache.

func MatchesObjectCompiledCached(g *state.Game, spec string, o *state.Object, sc SpecContext) bool {
	return compiledMatch(compiledSpecFor(spec), g, o, &sc)
}

// MatchesZoneTextOracle is matchesZoneSpecCtx's textual off-battlefield
// alternative loop.
func MatchesZoneTextOracle(g *state.Game, spec string, o *state.Object, sc SpecContext, zone state.Zone) bool {
	return matchesZoneSpecText(g, spec, o, sc, zone)
}
