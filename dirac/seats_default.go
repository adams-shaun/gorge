//go:build !dirac_mp

package dirac

// The default size class: two seats. Later size knobs (MaxObj, Obj, MaxPerm)
// move into this file and seats_mp.go.
const MaxSeats = 2
