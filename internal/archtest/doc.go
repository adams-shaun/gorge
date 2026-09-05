// Package archtest holds no code, only tests that walk the module's import
// graph and fail when a dependency-order or determinism constraint from the
// engine spec is broken: time outside the host, effects importing rules,
// the wire types importing the engine, the host importing test fixtures.
package archtest
