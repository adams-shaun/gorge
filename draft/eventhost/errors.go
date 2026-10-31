package eventhost

import "fmt"

// Error is a refusal with an HTTP status and a stable code; httpapi reads
// Status and Code through an interface, so this package never imports it.
type Error struct {
	status int
	code   string
	msg    string
}

func (e *Error) Error() string { return e.msg }
func (e *Error) Status() int   { return e.status }
func (e *Error) Code() string  { return e.code }

// The stable codes with their HTTP statuses.
const (
	codeNotFound     = "not_found"     // 404
	codeUnauthorized = "unauthorized"  // 401
	codeBadRequest   = "bad_request"   // 400
	codeNotInPack    = "not_in_pack"   // 400
	codeNotYourTurn  = "not_your_turn" // 409
	codeStalePick    = "stale_pick"    // 409
	codeConflict     = "conflict"      // 409
	codeCapacity     = "capacity"      // 409
	codeInvalidDeck  = "invalid_deck"  // 422
	codeUnavailable  = "unavailable"   // 503
)

func errNotFound(format string, args ...any) *Error {
	return &Error{status: 404, code: codeNotFound, msg: fmt.Sprintf(format, args...)}
}

func errUnauthorized(format string, args ...any) *Error {
	return &Error{status: 401, code: codeUnauthorized, msg: fmt.Sprintf(format, args...)}
}

func errBadRequest(format string, args ...any) *Error {
	return &Error{status: 400, code: codeBadRequest, msg: fmt.Sprintf(format, args...)}
}

func errNotInPack(format string, args ...any) *Error {
	return &Error{status: 400, code: codeNotInPack, msg: fmt.Sprintf(format, args...)}
}

func errNotYourTurn(format string, args ...any) *Error {
	return &Error{status: 409, code: codeNotYourTurn, msg: fmt.Sprintf(format, args...)}
}

func errStalePick(format string, args ...any) *Error {
	return &Error{status: 409, code: codeStalePick, msg: fmt.Sprintf(format, args...)}
}

func errConflict(format string, args ...any) *Error {
	return &Error{status: 409, code: codeConflict, msg: fmt.Sprintf(format, args...)}
}

func errCapacity(format string, args ...any) *Error {
	return &Error{status: 409, code: codeCapacity, msg: fmt.Sprintf(format, args...)}
}

func errInvalidDeck(format string, args ...any) *Error {
	return &Error{status: 422, code: codeInvalidDeck, msg: fmt.Sprintf(format, args...)}
}

func errUnavailable(format string, args ...any) *Error {
	return &Error{status: 503, code: codeUnavailable, msg: fmt.Sprintf(format, args...)}
}
