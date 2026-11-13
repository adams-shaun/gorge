package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/adams-shaun/gorge/server/protocol"
)

// EventService is the draft-event backend behind /api/events. token is the
// caller's event seat token ("" for a spectator), passed through verbatim.
// An error that implements interface{ Status() int; Code() string } picks
// the HTTP status and ErrorBody.Code; any other error is a 500 "internal".
type EventService interface {
	CreateEvent(req protocol.CreateEventRequest) (protocol.CreateEventResponse, error)
	Events() []protocol.EventState
	EventState(id, token string) (protocol.EventState, error)
	JoinEvent(id, token string, req protocol.JoinEventRequest) (protocol.EventState, error)
	DraftPack(id, token string) (protocol.DraftPack, error)
	DraftAdvice(id, token string) (protocol.DraftAdvice, error)
	Pick(id, token string, req protocol.PickRequest) (protocol.DraftPack, error)
	EventPool(id, token string) (protocol.EventPool, error)
	CheckDeck(id, token string, req protocol.DeckRequest) (protocol.DeckCheck, error)
	SaveDeck(id, token string, req protocol.DeckRequest) (protocol.DeckCheck, error)
	Ready(id, token string, req protocol.ReadyRequest) (protocol.EventState, error)
	Sideboard(id, token string, req protocol.SideboardRequest) (protocol.EventState, error)
	EventLog(id, token string) (protocol.EventLog, error)
	SubscribeEvent(id, token, session string) error
	// The two discovery endpoints.
	Sets() []protocol.EventSetInfo
	Personas() protocol.EventPersonas
}

// eventStatusCoder is the error shape the backend reports its protocol
// failures with (eventhost's statusErr); writeEventError maps it.
type eventStatusCoder interface {
	error
	Status() int
	Code() string
}

// mountEvents arms the /api/events routes when an EventService backend is
// wired. nil — the default — registers nothing, so every /api/events path
// keeps the JSON 404 the catch-all answers today.
func (h *handler) mountEvents(mux *http.ServeMux) {
	if h.opts.Events == nil {
		return
	}
	mux.HandleFunc("POST /api/events", h.createEvent)
	mux.HandleFunc("GET /api/events", h.listEvents)
	mux.HandleFunc("GET /api/events/{id}", h.eventState)
	mux.HandleFunc("POST /api/events/{id}/join", h.joinEvent)
	mux.HandleFunc("GET /api/events/{id}/pack", h.eventPack)
	mux.HandleFunc("GET /api/events/{id}/advice", h.eventAdvice)
	mux.HandleFunc("POST /api/events/{id}/pick", h.eventPick)
	mux.HandleFunc("GET /api/events/{id}/pool", h.eventPool)
	mux.HandleFunc("POST /api/events/{id}/deck/check", h.eventCheckDeck)
	mux.HandleFunc("POST /api/events/{id}/deck", h.eventSaveDeck)
	mux.HandleFunc("POST /api/events/{id}/ready", h.eventReady)
	mux.HandleFunc("POST /api/events/{id}/sideboard", h.eventSideboard)
	mux.HandleFunc("GET /api/events/{id}/log", h.eventLog)
	mux.HandleFunc("POST /api/events/{id}/subscribe", h.eventSubscribe)
	mux.HandleFunc("GET /api/events/sets", h.eventSets)
	mux.HandleFunc("GET /api/events/personas", h.eventPersonas)
	// The two literals' method twins are registered per method: a
	// method-less twin of /api/events/sets would conflict with
	// GET /api/events/{id} in the mux (more specific path, broader method
	// set). A POST twin answers 405; any other method falls through to the
	// method-less /api/events/{id} twin below, which also answers 405.
	mux.HandleFunc("POST /api/events/sets", methodNotAllowed)
	mux.HandleFunc("POST /api/events/personas", methodNotAllowed)
	// Method-less twins of every pattern above: the mux prefers the
	// method-specific pattern, so these only ever see the wrong method and
	// answer 405 in JSON.
	for _, p := range []string{
		"/api/events",
		"/api/events/{id}",
		"/api/events/{id}/join",
		"/api/events/{id}/pack",
		"/api/events/{id}/advice",
		"/api/events/{id}/pick",
		"/api/events/{id}/pool",
		"/api/events/{id}/deck/check",
		"/api/events/{id}/deck",
		"/api/events/{id}/ready",
		"/api/events/{id}/sideboard",
		"/api/events/{id}/log",
		"/api/events/{id}/subscribe",
	} {
		mux.HandleFunc(p, methodNotAllowed)
	}
}

func (h *handler) createEvent(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateEventRequest
	if !decodeBody(w, r, &req) {
		return
	}
	resp, err := h.opts.Events.CreateEvent(req)
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *handler) listEvents(w http.ResponseWriter, r *http.Request) {
	states := h.opts.Events.Events()
	if states == nil {
		states = []protocol.EventState{}
	}
	writeJSON(w, http.StatusOK, states)
}

func (h *handler) eventState(w http.ResponseWriter, r *http.Request) {
	st, err := h.opts.Events.EventState(r.PathValue("id"), eventToken(r))
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *handler) joinEvent(w http.ResponseWriter, r *http.Request) {
	var req protocol.JoinEventRequest
	if !decodeBody(w, r, &req) {
		return
	}
	st, err := h.opts.Events.JoinEvent(r.PathValue("id"), eventToken(r), req)
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *handler) eventPack(w http.ResponseWriter, r *http.Request) {
	pack, err := h.opts.Events.DraftPack(r.PathValue("id"), eventToken(r))
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

func (h *handler) eventAdvice(w http.ResponseWriter, r *http.Request) {
	adv, err := h.opts.Events.DraftAdvice(r.PathValue("id"), eventToken(r))
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, adv)
}

func (h *handler) eventPick(w http.ResponseWriter, r *http.Request) {
	var req protocol.PickRequest
	if !decodeBody(w, r, &req) {
		return
	}
	pack, err := h.opts.Events.Pick(r.PathValue("id"), eventToken(r), req)
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

func (h *handler) eventPool(w http.ResponseWriter, r *http.Request) {
	pool, err := h.opts.Events.EventPool(r.PathValue("id"), eventToken(r))
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pool)
}

func (h *handler) eventCheckDeck(w http.ResponseWriter, r *http.Request) {
	var req protocol.DeckRequest
	if !decodeBody(w, r, &req) {
		return
	}
	check, err := h.opts.Events.CheckDeck(r.PathValue("id"), eventToken(r), req)
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, check)
}

func (h *handler) eventSaveDeck(w http.ResponseWriter, r *http.Request) {
	var req protocol.DeckRequest
	if !decodeBody(w, r, &req) {
		return
	}
	check, err := h.opts.Events.SaveDeck(r.PathValue("id"), eventToken(r), req)
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, check)
}

func (h *handler) eventReady(w http.ResponseWriter, r *http.Request) {
	var req protocol.ReadyRequest
	if !decodeBody(w, r, &req) {
		return
	}
	st, err := h.opts.Events.Ready(r.PathValue("id"), eventToken(r), req)
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *handler) eventSideboard(w http.ResponseWriter, r *http.Request) {
	var req protocol.SideboardRequest
	if !decodeBody(w, r, &req) {
		return
	}
	st, err := h.opts.Events.Sideboard(r.PathValue("id"), eventToken(r), req)
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *handler) eventLog(w http.ResponseWriter, r *http.Request) {
	log, err := h.opts.Events.EventLog(r.PathValue("id"), eventToken(r))
	if err != nil {
		writeEventError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (h *handler) eventSubscribe(w http.ResponseWriter, r *http.Request) {
	var req protocol.EventSubscribe
	if !decodeBody(w, r, &req) {
		return
	}
	if err := h.opts.Events.SubscribeEvent(r.PathValue("id"), eventToken(r), req.Session); err != nil {
		writeEventError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// eventSets answers GET /api/events/sets with the draftable set catalogue.
func (h *handler) eventSets(w http.ResponseWriter, r *http.Request) {
	sets := h.opts.Events.Sets()
	if sets == nil {
		sets = []protocol.EventSetInfo{}
	}
	writeJSON(w, http.StatusOK, sets)
}

// eventPersonas answers GET /api/events/personas with the pickbot persona
// names a create body's Drafter fields accept.
func (h *handler) eventPersonas(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.opts.Events.Personas())
}

// eventToken reads the event seat token the way cmd/gorged reads a seat
// token: the Authorization: Bearer header (case-insensitive scheme) first,
// then the ?token= query the browser needs. Absent everywhere it is "" — a
// spectator.
func eventToken(r *http.Request) string {
	const prefix = "Bearer "
	if v := r.Header.Get("Authorization"); len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
		return v[len(prefix):]
	}
	return r.URL.Query().Get("token")
}

// writeEventError maps a backend error onto a reply: an error carrying its
// own status and code decides both; anything else is a 500 "internal".
func writeEventError(w http.ResponseWriter, err error) {
	var sc eventStatusCoder
	if errors.As(err, &sc) {
		writeError(w, sc.Status(), sc.Code(), err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", err.Error())
}
