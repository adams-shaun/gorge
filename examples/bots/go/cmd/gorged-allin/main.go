// Command gorged-allin is gorged with the allin example policy linked in:
// gorge's own hosted bots (bots/all) plus one blank import. Every gorged flag
// works, and allin's option is set with -bot-opt allin.min-power=N.
package main

import (
	"os"

	_ "github.com/adams-shaun/gorge/bots/all"
	_ "github.com/adams-shaun/gorge/examples/bots/go/allin"
	"github.com/adams-shaun/gorge/server/gorged"
)

func main() { os.Exit(gorged.Main(os.Args[1:], gorged.Extensions{})) }
