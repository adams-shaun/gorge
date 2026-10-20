package gorged

import (
	"net/http"
	"testing"
)

// TestFeedbackStripsSeatTokenFromStoredURL proves the server, not the client,
// removes a seat's credential from the stored report URL. The web form sends
// location.href verbatim, so a seated player's join URL arrives with ?token=
// (and some flows use seat_token); neither may reach disk.
func TestFeedbackStripsSeatTokenFromStoredURL(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"token", "http://localhost:8080/t/t1?seat=0&token=abc", "http://localhost:8080/t/t1?seat=0"},
		{"seat_token", "http://localhost:8080/t/t1?seat_token=abc&seat=1", "http://localhost:8080/t/t1?seat=1"},
		{"both", "http://localhost:8080/t/t1?token=abc&seat_token=def&seq=3", "http://localhost:8080/t/t1?seq=3"},
		{"no credential", "http://localhost:8080/t/t1?seat=0", "http://localhost:8080/t/t1?seat=0"},
		{"unparseable query", "http://localhost:8080/t/t1?token=%zz", "http://localhost:8080/t/t1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newTestStore(t)
			rec := postFeedback(t, fs, "the token leaked", tc.in, nil)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
			}
			got := readReport(t, storedReports(t, fs)[0])
			if got.URL != tc.want {
				t.Fatalf("stored url = %q, want %q", got.URL, tc.want)
			}
		})
	}
}
