package httpserver

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Server) getScheduleSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.App.GetScheduleSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// putScheduleSettings overlays the body on top of defaults so partial bodies
// and older clients never blank out a knob.
func (s *Server) putScheduleSettings(w http.ResponseWriter, r *http.Request) {
	settings := domain.DefaultScheduleSettings()
	if err := decodeJSON(r, &settings); err != nil {
		writeError(w, err)
		return
	}
	saved, err := s.App.SaveScheduleSettings(r.Context(), settings)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) listDayOverrides(w http.ResponseWriter, r *http.Request) {
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
	list, err := s.App.ListDayOverrides(r.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) putDayOverride(w http.ResponseWriter, r *http.Request) {
	day, err := domain.ParseYmd(r.PathValue("date"))
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Off          bool   `json:"off"`
		WorkStartMin *int   `json:"workStartMin"`
		WorkEndMin   *int   `json:"workEndMin"`
		Note         string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	override, err := s.App.UpsertDayOverride(r.Context(), day, domain.DayOverrideDraft{
		Off:          body.Off,
		WorkStartMin: body.WorkStartMin,
		WorkEndMin:   body.WorkEndMin,
		Note:         body.Note,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, override)
}

func (s *Server) deleteDayOverride(w http.ResponseWriter, r *http.Request) {
	day, err := domain.ParseYmd(r.PathValue("date"))
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteDayOverride(r.Context(), day); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type whatIfBody struct {
	Kind    string `json:"kind"`
	From    string `json:"from"`
	To      string `json:"to"`
	MoveDue []struct {
		ItemID string `json:"itemId"`
		DueAt  string `json:"dueAt"`
	} `json:"moveDue"`
	DropItems  []string `json:"dropItems"`
	SkipEvents []string `json:"skipEvents"`
}

func (s *Server) scheduleWhatIf(w http.ResponseWriter, r *http.Request) {
	var body whatIfBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	kind, err := domain.ParseScheduleKind(body.Kind)
	if err != nil {
		writeError(w, err)
		return
	}
	from, err := parseOptionalTime(body.From)
	if err != nil || from.IsZero() {
		writeError(w, domain.ErrInvalid)
		return
	}
	to, err := parseOptionalTime(body.To)
	if err != nil || to.IsZero() {
		writeError(w, domain.ErrInvalid)
		return
	}
	scenario := domain.WhatIfScenario{}
	for _, move := range body.MoveDue {
		id, err := uuid.Parse(move.ItemID)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		due, err := time.Parse(time.RFC3339, move.DueAt)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		scenario.MoveDue = append(scenario.MoveDue, domain.WhatIfMove{ItemID: id, DueAt: due})
	}
	scenario.DropItems, err = parseIDs(body.DropItems)
	if err != nil {
		writeError(w, err)
		return
	}
	scenario.SkipEvents, err = parseIDs(body.SkipEvents)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.App.ScheduleWhatIf(r.Context(), from, to, kind, scenario)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func parseIDs(raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, value := range raw {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, domain.ErrInvalid
		}
		out = append(out, id)
	}
	return out, nil
}
