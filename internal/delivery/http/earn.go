package httpserver

import (
	"net/http"

	"github.com/pchkauu/want-brief/internal/domain"
)

const defaultEarnRate = 85

func (s *Server) listEarnDays(w http.ResponseWriter, r *http.Request) {
	from, err := domain.ParseYmd(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, err)
		return
	}
	to, err := domain.ParseYmd(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	list, err := s.App.ListEarnDays(r.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) putEarnDay(w http.ResponseWriter, r *http.Request) {
	day, err := domain.ParseYmd(r.PathValue("day"))
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		USD  float64 `json:"usd"`
		RUB  float64 `json:"rub"`
		Rate float64 `json:"rate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	rate := body.Rate
	if rate <= 0 {
		rate = defaultEarnRate
	}
	row, err := s.App.UpsertEarnDay(r.Context(), day, body.USD, body.RUB, rate)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}
