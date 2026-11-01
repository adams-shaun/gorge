// Command gorged serves bot tables, play-vs-bot, spectating and, with -events,
// draft events to a browser: the reference server (server/gorged). This build
// links gorge's own hosted bots, the draft-event plugin (draft/gorgedevents) and,
// for `gorged review`, the feedback-snapshot loader (review.go); a binary that
// adds extensions (more bots) wraps gorged.Main the same way with its own
// gorged.Extensions.
package main

import (
	"os"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/gorgedevents"
	"github.com/adams-shaun/gorge/server/gorged"
)

func extensions() gorged.Extensions {
	return gorged.Extensions{Review: reviewLoad, Events: gorgedevents.Plugin{}}
}

func main() { os.Exit(gorged.Main(os.Args[1:], extensions())) }
