// Package sbv2http serves a human seat's SpellBench v2 decisions over HTTP:
// GET .../state, GET .../stream (SSE) and POST .../send. It is the v2 front of
// a host.HumanSeat tapped with a server/spellbench/humanseat.Seat, mounted by
// server/gorged only behind -sbv2 (server/gorged/sbv2.go). With the flag off
// nothing is mounted and its paths keep httpapi's JSON 404, exactly as the
// ManaBrew adapter does.
//
// The seat is the bearer token's claim and never the request body or a path
// component: server/host/httpapi.SeatClaim is the one fence shared with the
// native wire, so a token works on both and neither can disagree about which
// seat it names.
package sbv2http
