// Package host keeps tables running: a registry of table configurations,
// one goroutine per table that plays match after match with bot seats,
// sessions that subscribe to overview widgets or a focused table's event
// stream, turn-start snapshots that answer "view at seq N", append-only
// persistence, and crash handling that halts a table rather than hiding a
// bug. Each table runs on an engine (Options.Engines, TableConfig.Engine)
// through the game interfaces. It imports rules and seat but exposes neither: a
// client sees protocol frames and view.Views only. It is the first package allowed to
// import time, and the only clock it uses is the injected Sleep.
package host
