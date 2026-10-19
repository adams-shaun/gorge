package main

import (
	"os"

	"github.com/adams-shaun/gorge/server/gorged"
)

func main() {
	os.Exit(gorged.Main(os.Args[1:], gorged.Extensions{}))
}
