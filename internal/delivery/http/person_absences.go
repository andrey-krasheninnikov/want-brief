package httpserver

import (
	"net/http"
	"time"

	"github.com/pchkauu/want-brief/internal/domain"
)

func decodePersonAbsence(r *http.Request) (domain.PersonAbsenceDraft, error) {
	var body struct {
		StartsOn string `json:"startsOn"`
		EndsOn   string `json:"endsOn"`
		Note     string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return domain.PersonAbsenceDraft{}, err
	}
	start, err := parseDateOrTime(body.StartsOn)
	if err != nil {
		return domain.PersonAbsenceDraft{}, err
	}
	end, err := parseDateOrTime(body.EndsOn)
	if err != nil {
		return domain.PersonAbsenceDraft{}, err
	}
	return domain.PersonAbsenceDraft{StartsOn: start, EndsOn: end, Note: body.Note}, nil
}

// parseDateOrTime accepts YYYY-MM-DD or RFC3339; both map to a calendar day.
func parseDateOrTime(raw string) (time.Time, error) {
	if day, err := domain.ParseYmd(raw); err == nil {
		return day.Midnight(time.UTC), nil
	}
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, domain.ErrInvalid
	}
	return at, nil
}

func (s *Server) createPersonAbsence(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonAbsence(r)
	if err != nil {
		writeError(w, err)
		return
	}
	absence, err := s.App.CreatePersonAbsence(r.Context(), id, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, absence)
}

func (s *Server) patchPersonAbsence(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	absenceID, err := parseNamedID(r, "absenceId")
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonAbsence(r)
	if err != nil {
		writeError(w, err)
		return
	}
	absence, err := s.App.ReplacePersonAbsence(r.Context(), id, absenceID, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, absence)
}

func (s *Server) deletePersonAbsence(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	absenceID, err := parseNamedID(r, "absenceId")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeletePersonAbsence(r.Context(), id, absenceID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
