// Package cards compiles Forge card scripts into the engine's card IR.
//
// The scripts themselves are GPL-3.0 and are never vendored here: forgec
// fetches a pinned upstream ref at build time and this package compiles it
// into a local cache. See docs/superpowers/specs/2026-09-03-mtgcore-go-engine-design.md.
package cards
