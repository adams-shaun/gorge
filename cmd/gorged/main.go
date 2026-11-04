// Command gorged serves bot tables, play-vs-bot, spectating and, with -events,
// draft events to a browser: the reference server (server/gorged), and the
// binary the demo deploys (scripts/deploy-demo.sh). This build links gorge's
// built-in hosted bots (bots/all), the experimental search and SpellBench
// policies (bots/search, sbsearch, sbbuiltin, castprofile), the draft-event plugin (draft/gorgedevents) and, for
// `gorged review`, the feedback-snapshot loader (review.go). A binary that adds
// extensions (more bots) wraps gorged.Main the same way with its own
// gorged.Extensions.
package main

import (
	"os"

	_ "github.com/adams-shaun/gorge/bots/all"
	_ "github.com/adams-shaun/gorge/bots/castprofile"
	_ "github.com/adams-shaun/gorge/bots/sbbuiltin"
	_ "github.com/adams-shaun/gorge/bots/sbsearch"
	_ "github.com/adams-shaun/gorge/bots/search"
	"github.com/adams-shaun/gorge/draft/gorgedevents"
	"github.com/adams-shaun/gorge/server/gorged"
)

func extensions() gorged.Extensions {
	return gorged.Extensions{Review: reviewLoad, Events: gorgedevents.Plugin{}}
}

func main() { os.Exit(gorged.Main(os.Args[1:], extensions())) }
