// regenerate.go carries the go:generate directive that refreshes the
// embedded set specs. Run from draft/limited/setdata:
//
//	go generate ./
package setdata

//go:generate go run ./internal/gen -sets FDN,BLB
