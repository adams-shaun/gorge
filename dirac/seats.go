// MaxSeats, the length of every per-seat array in State and Config, is set
// per size class: seats_default.go (two seats) and seats_mp.go (build tag
// dirac_mp, four seats). The seat count is still two everywhere (seat loops,
// opponent arithmetic and the target codes); the constant only sizes the
// arrays.

package dirac
