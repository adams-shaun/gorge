// Command gorged serves bot tables, play-vs-bot and spectating to a browser:
// the reference server (server/gorged). This build links gorge's own hosted
// bots and, for `gorged review`, the feedback-snapshot loader (review.go); a
// binary that adds extensions (draft events, more bots) wraps gorged.Main the
// same way with its own gorged.Extensions.
package main

import (
	"os"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/server/gorged"
)

func main() { os.Exit(gorged.Main(os.Args[1:], gorged.Extensions{Review: reviewLoad})) }
