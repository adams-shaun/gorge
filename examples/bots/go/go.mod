// The native Go bot example lives in a module of its own, the way a bot
// author's module would: it requires gorge and, inside this repository,
// replaces it with the checkout three directories up. Outside the repo, drop
// the replace and require a gorge version (a pseudo-version of a commit).
module github.com/adams-shaun/gorge/examples/bots/go

go 1.25.8

require github.com/adams-shaun/gorge v0.0.0

replace github.com/adams-shaun/gorge => ../../..
