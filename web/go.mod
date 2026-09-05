// Fence: keeps `go build ./...` from the repo root out of web/ (node_modules
// ships stray .go files). No Go code lives here.
module github.com/adams-shaun/gorge/web

go 1.25.8
