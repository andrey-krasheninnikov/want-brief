package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/application"
	"github.com/pchkauu/want-brief/internal/domain"
)

const sessionCookie = "wb_session"

type Server struct {
	App        *application.Service
	CORSOrigin string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.withAuth(s.logout))
	mux.HandleFunc("GET /api/me", s.withAuth(s.me))

	mux.HandleFunc("GET /api/projects", s.withAuth(s.listProjects))
	mux.HandleFunc("POST /api/projects", s.withAuth(s.createProject))
	mux.HandleFunc("GET /api/projects/{id}", s.withAuth(s.getProject))
	mux.HandleFunc("PATCH /api/projects/{id}", s.withAuth(s.patchProject))
	mux.HandleFunc("DELETE /api/projects/{id}", s.withAuth(s.deleteProject))
	mux.HandleFunc("GET /api/projects/{id}/notes", s.withAuth(s.listProjectNotes))
	mux.HandleFunc("POST /api/projects/{id}/notes", s.withAuth(s.createProjectNote))
	mux.HandleFunc("DELETE /api/projects/{id}/notes/{noteId}", s.withAuth(s.deleteProjectNote))

	mux.HandleFunc("GET /api/sources", s.withAuth(s.listSources))
	mux.HandleFunc("POST /api/sources", s.withAuth(s.createSource))
	mux.HandleFunc("PATCH /api/sources/{id}", s.withAuth(s.patchSource))
	mux.HandleFunc("DELETE /api/sources/{id}", s.withAuth(s.deleteSource))
	mux.HandleFunc("POST /api/sources/{id}/sync", s.withAuth(s.syncSource))

	mux.HandleFunc("GET /api/items", s.withAuth(s.listItems))
	mux.HandleFunc("POST /api/items", s.withAuth(s.createItem))
	mux.HandleFunc("GET /api/items/{id}", s.withAuth(s.getItem))
	mux.HandleFunc("PATCH /api/items/{id}", s.withAuth(s.patchItem))
	mux.HandleFunc("DELETE /api/items/{id}", s.withAuth(s.deleteItem))
	mux.HandleFunc("POST /api/items/{id}/undelete", s.withAuth(s.undeleteItem))
	mux.HandleFunc("GET /api/items/{id}/notes", s.withAuth(s.listItemNotes))
	mux.HandleFunc("POST /api/items/{id}/notes", s.withAuth(s.createItemNote))
	mux.HandleFunc("DELETE /api/items/{id}/notes/{noteId}", s.withAuth(s.deleteItemNote))
	mux.HandleFunc("GET /api/items/{id}/events", s.withAuth(s.listItemEvents))
	mux.HandleFunc("POST /api/items/{id}/events/annotate", s.withAuth(s.annotateItemEvent))
	mux.HandleFunc("GET /api/items/{id}/checks", s.withAuth(s.listItemChecks))
	mux.HandleFunc("POST /api/items/{id}/checks", s.withAuth(s.createItemCheck))
	mux.HandleFunc("PATCH /api/items/{id}/checks/{checkId}", s.withAuth(s.patchItemCheck))
	mux.HandleFunc("DELETE /api/items/{id}/checks/{checkId}", s.withAuth(s.deleteItemCheck))
	mux.HandleFunc("POST /api/items/sync-active", s.withAuth(s.syncActiveItems))
	mux.HandleFunc("POST /api/items/{id}/sync", s.withAuth(s.syncItem))

	mux.HandleFunc("GET /api/notes", s.withAuth(s.listNotes))
	mux.HandleFunc("POST /api/notes", s.withAuth(s.createNote))
	mux.HandleFunc("PATCH /api/notes/{id}", s.withAuth(s.patchNote))
	mux.HandleFunc("DELETE /api/notes/{id}", s.withAuth(s.deleteNote))

	mux.HandleFunc("GET /api/intervals", s.withAuth(s.listIntervals))
	mux.HandleFunc("POST /api/intervals", s.withAuth(s.startInterval))
	mux.HandleFunc("POST /api/intervals/{id}/stop", s.withAuth(s.stopInterval))

	mux.HandleFunc("POST /api/stress", s.withAuth(s.createStress))
	mux.HandleFunc("POST /api/checkins", s.withAuth(s.createCheckin))
	mux.HandleFunc("GET /api/checkins/latest", s.withAuth(s.latestCheckins))
	mux.HandleFunc("GET /api/events", s.withAuth(s.listEvents))
	mux.HandleFunc("GET /api/event-series", s.withAuth(s.listEventSeries))
	mux.HandleFunc("POST /api/events", s.withAuth(s.createEvent))
	mux.HandleFunc("GET /api/events/{id}", s.withAuth(s.getEvent))
	mux.HandleFunc("PATCH /api/events/{id}", s.withAuth(s.patchEvent))
	mux.HandleFunc("DELETE /api/events/{id}", s.withAuth(s.deleteEvent))
	mux.HandleFunc("GET /api/events/{id}/occurrence", s.withAuth(s.getEventOccurrence))
	mux.HandleFunc("PUT /api/events/{id}/occurrence", s.withAuth(s.putEventOccurrence))
	mux.HandleFunc("DELETE /api/events/{id}/occurrence", s.withAuth(s.deleteEventOccurrence))
	mux.HandleFunc("GET /api/events/{id}/notes", s.withAuth(s.listEventNotes))
	mux.HandleFunc("POST /api/events/{id}/notes", s.withAuth(s.createEventNote))
	mux.HandleFunc("DELETE /api/events/{id}/notes/{noteId}", s.withAuth(s.deleteEventNote))
	mux.HandleFunc("GET /api/companies", s.withAuth(s.listCompanies))
	mux.HandleFunc("POST /api/companies", s.withAuth(s.createCompany))
	mux.HandleFunc("GET /api/companies/{id}", s.withAuth(s.getCompany))
	mux.HandleFunc("PATCH /api/companies/{id}", s.withAuth(s.patchCompany))
	mux.HandleFunc("DELETE /api/companies/{id}", s.withAuth(s.deleteCompany))
	mux.HandleFunc("POST /api/companies/{id}/titles", s.withAuth(s.createCompanyTitle))
	mux.HandleFunc("POST /api/companies/{id}/salaries", s.withAuth(s.createCompanySalary))
	mux.HandleFunc("POST /api/companies/{id}/manager", s.withAuth(s.putCompanyManager))
	mux.HandleFunc("POST /api/companies/{id}/reports", s.withAuth(s.createCompanyReport))
	mux.HandleFunc("DELETE /api/companies/{id}/reports/{reportId}", s.withAuth(s.deleteCompanyReport))
	mux.HandleFunc("POST /api/companies/{id}/contracts", s.withAuth(s.createCompanyContract))
	mux.HandleFunc("GET /api/companies/{id}/notes", s.withAuth(s.listCompanyNotes))
	mux.HandleFunc("POST /api/companies/{id}/notes", s.withAuth(s.createCompanyNote))
	mux.HandleFunc("PATCH /api/companies/{id}/notes/{noteId}", s.withAuth(s.patchCompanyNote))
	mux.HandleFunc("DELETE /api/companies/{id}/notes/{noteId}", s.withAuth(s.deleteCompanyNote))
	mux.HandleFunc("GET /api/people", s.withAuth(s.listPeople))
	mux.HandleFunc("POST /api/people", s.withAuth(s.createPerson))
	mux.HandleFunc("GET /api/people/{id}", s.withAuth(s.getPerson))
	mux.HandleFunc("PATCH /api/people/{id}", s.withAuth(s.patchPerson))
	mux.HandleFunc("DELETE /api/people/{id}", s.withAuth(s.deletePerson))
	mux.HandleFunc("GET /api/people/{id}/notes", s.withAuth(s.listPersonNotes))
	mux.HandleFunc("POST /api/people/{id}/notes", s.withAuth(s.createPersonNote))
	mux.HandleFunc("PATCH /api/people/{id}/notes/{noteId}", s.withAuth(s.patchPersonNote))
	mux.HandleFunc("DELETE /api/people/{id}/notes/{noteId}", s.withAuth(s.deletePersonNote))
	mux.HandleFunc("POST /api/people/{id}/contacts", s.withAuth(s.createPersonContact))
	mux.HandleFunc("PATCH /api/people/{id}/contacts/{contactId}", s.withAuth(s.patchPersonContact))
	mux.HandleFunc("DELETE /api/people/{id}/contacts/{contactId}", s.withAuth(s.deletePersonContact))
	mux.HandleFunc("POST /api/people/{id}/professions", s.withAuth(s.createPersonProfession))
	mux.HandleFunc("PATCH /api/people/{id}/professions/{professionId}", s.withAuth(s.patchPersonProfession))
	mux.HandleFunc("DELETE /api/people/{id}/professions/{professionId}", s.withAuth(s.deletePersonProfession))
	mux.HandleFunc("POST /api/people/{id}/sites", s.withAuth(s.createPersonSite))
	mux.HandleFunc("PATCH /api/people/{id}/sites/{siteId}", s.withAuth(s.patchPersonSite))
	mux.HandleFunc("DELETE /api/people/{id}/sites/{siteId}", s.withAuth(s.deletePersonSite))
	mux.HandleFunc("POST /api/people/{id}/bonds", s.withAuth(s.createPersonBond))
	mux.HandleFunc("PATCH /api/people/{id}/bonds/{bondId}", s.withAuth(s.patchPersonBond))
	mux.HandleFunc("POST /api/people/{id}/bonds/{bondId}/end", s.withAuth(s.endPersonBond))
	mux.HandleFunc("POST /api/people/{id}/absences", s.withAuth(s.createPersonAbsence))
	mux.HandleFunc("PATCH /api/people/{id}/absences/{absenceId}", s.withAuth(s.patchPersonAbsence))
	mux.HandleFunc("DELETE /api/people/{id}/absences/{absenceId}", s.withAuth(s.deletePersonAbsence))
	mux.HandleFunc("GET /api/load", s.withAuth(s.load))
	mux.HandleFunc("GET /api/schedule", s.withAuth(s.schedule))
	mux.HandleFunc("GET /api/schedule/settings", s.withAuth(s.getScheduleSettings))
	mux.HandleFunc("PUT /api/schedule/settings", s.withAuth(s.putScheduleSettings))
	mux.HandleFunc("GET /api/schedule/days", s.withAuth(s.listDayOverrides))
	mux.HandleFunc("PUT /api/schedule/days/{date}", s.withAuth(s.putDayOverride))
	mux.HandleFunc("DELETE /api/schedule/days/{date}", s.withAuth(s.deleteDayOverride))
	mux.HandleFunc("POST /api/schedule/whatif", s.withAuth(s.scheduleWhatIf))
	mux.HandleFunc("GET /api/journal", s.withAuth(s.listJournal))

	return s.cors(mux)
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := s.CORSOrigin
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, domain.ErrUnauthorized)
			return
		}
		if err := s.App.Authenticate(r.Context(), cookie.Value); err != nil {
			writeError(w, err)
			return
		}
		next(w, r)
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	token, err := s.App.Login(r.Context(), body.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil {
		_ = s.App.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.ListProjects(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	project, err := s.App.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name             string               `json:"name"`
		Color            string               `json:"color"`
		Description      string               `json:"description"`
		MonthlyIncomeUSD float64              `json:"monthlyIncomeUsd"`
		MonthlyIncomeRUB float64              `json:"monthlyIncomeRub"`
		TargetHoursDay   float64              `json:"targetHoursDay"`
		Links            []domain.ProjectLink `json:"links"`
		People           []domain.PersonRel   `json:"people"`
		Companies        []domain.PersonRel   `json:"companies"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	people, err := domain.NormalizePersonRels(body.People)
	if err != nil {
		writeError(w, err)
		return
	}
	companies, err := domain.NormalizePersonRels(body.Companies)
	if err != nil {
		writeError(w, err)
		return
	}
	project, err := s.App.CreateProject(r.Context(), application.ProjectWrite{
		Name:             body.Name,
		Color:            body.Color,
		Description:      body.Description,
		MonthlyIncomeUSD: body.MonthlyIncomeUSD,
		MonthlyIncomeRUB: body.MonthlyIncomeRUB,
		TargetHoursDay:   body.TargetHoursDay,
		Links:            body.Links,
		People:           people,
		Companies:        companies,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) patchProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name             *string               `json:"name"`
		Color            *string               `json:"color"`
		Description      *string               `json:"description"`
		MonthlyIncomeUSD *float64              `json:"monthlyIncomeUsd"`
		MonthlyIncomeRUB *float64              `json:"monthlyIncomeRub"`
		TargetHoursDay   *float64              `json:"targetHoursDay"`
		Links            *[]domain.ProjectLink `json:"links"`
		People           *[]domain.PersonRel   `json:"people"`
		Companies        *[]domain.PersonRel   `json:"companies"`
		Archived         *bool                 `json:"archived"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	people, err := parseOptionalPersonRels(body.People)
	if err != nil {
		writeError(w, err)
		return
	}
	companies, err := parseOptionalPersonRels(body.Companies)
	if err != nil {
		writeError(w, err)
		return
	}
	project, err := s.App.PatchProject(r.Context(), id, application.ProjectPatch{
		Name:             body.Name,
		Color:            body.Color,
		Description:      body.Description,
		MonthlyIncomeUSD: body.MonthlyIncomeUSD,
		MonthlyIncomeRUB: body.MonthlyIncomeRUB,
		TargetHoursDay:   body.TargetHoursDay,
		Links:            body.Links,
		People:           people,
		Companies:        companies,
		Archived:         body.Archived,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteProject(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listProjectNotes(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	notes, err := s.App.ListProjectNotes(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) createProjectNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	note, err := s.App.CreateProjectNote(r.Context(), id, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) deleteProjectNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("noteId"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	if err := s.App.DeleteProjectNote(r.Context(), id, noteID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.ListSources(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var body application.SourceInput
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	source, err := s.App.CreateSource(r.Context(), body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, source)
}

func (s *Server) patchSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body application.SourceInput
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	source, err := s.App.PatchSource(r.Context(), id, body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteSource(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	n, err := s.App.SyncSource(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"upserted": n})
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	filter := domain.ItemFilter{}
	q := r.URL.Query()
	if v := q.Get("sourceId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		filter.SourceID = &id
	}
	if v := q.Get("projectId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		filter.ProjectID = &id
	}
	if v := q.Get("kind"); v != "" {
		kind, err := domain.ParseItemKind(v)
		if err != nil {
			writeError(w, err)
			return
		}
		filter.Kind = &kind
	}
	if v := q.Get("status"); v != "" {
		status, err := domain.ParseItemStatus(v)
		if err != nil {
			writeError(w, err)
			return
		}
		filter.Status = &status
	}
	if v := q.Get("openOnly"); v == "true" || v == "1" {
		filter.OpenOnly = true
	}
	if v := q.Get("includeArchived"); v == "true" || v == "1" {
		filter.IncludeArchived = true
	}
	if v := q.Get("archivedOnly"); v == "true" || v == "1" {
		filter.ArchivedOnly = true
	}
	items, err := s.App.ListItems(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title     string  `json:"title"`
		Kind      string  `json:"kind"`
		ProjectID *string `json:"projectId"`
		Urgent    bool    `json:"urgent"`
		Important bool    `json:"important"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	var projectID *uuid.UUID
	if body.ProjectID != nil && *body.ProjectID != "" {
		id, err := uuid.Parse(*body.ProjectID)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		projectID = &id
	}
	item, err := s.App.CreateItem(r.Context(), body.Title, body.Kind, projectID, body.Urgent, body.Important)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.App.GetItem(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) patchItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Title          *string               `json:"title"`
		Status         *string               `json:"status"`
		Kind           *string               `json:"kind"`
		ProjectID      *string               `json:"projectId"`
		Urgent         *bool                 `json:"urgent"`
		Important      *bool                 `json:"important"`
		Pinned         *bool                 `json:"pinned"`
		PinnedAt       *string               `json:"pinnedAt"`
		Stress         *int                  `json:"stress"`
		DueAt          *string               `json:"dueAt"`
		DevDueAt       *string               `json:"devDueAt"`
		ReviewDueAt    *string               `json:"reviewDueAt"`
		TestDueAt      *string               `json:"testDueAt"`
		Description    *string               `json:"description"`
		PlannedSeconds *int                  `json:"plannedSeconds"`
		Links          *[]domain.ProjectLink `json:"links"`
		PersonIDs      *[]string             `json:"personIds"`
		ExternalKey    *string               `json:"externalKey"`
		Occupancy      *string               `json:"occupancy"`
		ExternalStatus *string               `json:"externalStatus"`
		Archived       *bool                 `json:"archived"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	patch := application.ItemPatch{
		Title:          body.Title,
		Status:         body.Status,
		Kind:           body.Kind,
		Urgent:         body.Urgent,
		Important:      body.Important,
		Pinned:         body.Pinned,
		Stress:         body.Stress,
		Description:    body.Description,
		PlannedSeconds: body.PlannedSeconds,
		Links:          body.Links,
		ExternalKey:    body.ExternalKey,
		Occupancy:      body.Occupancy,
		ExternalStatus: body.ExternalStatus,
		Archived:       body.Archived,
	}
	ids, err := parseOptionalIDList(body.PersonIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	patch.PersonIDs = ids
	if body.ProjectID != nil {
		if *body.ProjectID == "" {
			patch.ClearProj = true
		} else {
			pid, err := uuid.Parse(*body.ProjectID)
			if err != nil {
				writeError(w, domain.ErrInvalid)
				return
			}
			patch.ProjectID = &pid
		}
	}
	if body.Stress != nil && *body.Stress == 0 {
		patch.Stress = nil
		patch.ClearStr = true
	}
	if body.PinnedAt != nil {
		if *body.PinnedAt == "" {
			patch.ClearPinnedAt = true
		} else {
			at, err := time.Parse(time.RFC3339, *body.PinnedAt)
			if err != nil {
				writeError(w, domain.ErrInvalid)
				return
			}
			patch.PinnedAt = &at
		}
	}
	if body.DueAt != nil {
		if *body.DueAt == "" {
			patch.ClearDue = true
		} else {
			due, err := time.Parse(time.RFC3339, *body.DueAt)
			if err != nil {
				writeError(w, domain.ErrInvalid)
				return
			}
			patch.DueAt = &due
		}
	}
	if body.DevDueAt != nil {
		if *body.DevDueAt == "" {
			patch.ClearDevDue = true
		} else {
			due, err := time.Parse(time.RFC3339, *body.DevDueAt)
			if err != nil {
				writeError(w, domain.ErrInvalid)
				return
			}
			patch.DevDueAt = &due
		}
	}
	if body.ReviewDueAt != nil {
		if *body.ReviewDueAt == "" {
			patch.ClearReviewDue = true
		} else {
			due, err := time.Parse(time.RFC3339, *body.ReviewDueAt)
			if err != nil {
				writeError(w, domain.ErrInvalid)
				return
			}
			patch.ReviewDueAt = &due
		}
	}
	if body.TestDueAt != nil {
		if *body.TestDueAt == "" {
			patch.ClearTestDue = true
		} else {
			due, err := time.Parse(time.RFC3339, *body.TestDueAt)
			if err != nil {
				writeError(w, domain.ErrInvalid)
				return
			}
			patch.TestDueAt = &due
		}
	}
	item, err := s.App.PatchItem(r.Context(), id, patch)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) listItemNotes(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	notes, err := s.App.ListItemNotes(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) createItemNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	note, err := s.App.CreateItemNote(r.Context(), id, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) deleteItemNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("noteId"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	if err := s.App.DeleteItemNote(r.Context(), id, noteID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listItemEvents(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	events, err := s.App.ListItemEvents(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) annotateItemEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Kinds  []string `json:"kinds"`
		Note   string   `json:"note"`
		Stress *int     `json:"stress"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	event, err := s.App.AnnotateLatestItemEvent(r.Context(), id, body.Kinds, body.Note, body.Stress)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) listItemChecks(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	checks, err := s.App.ListItemChecks(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

func (s *Server) createItemCheck(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	check, err := s.App.CreateItemCheck(r.Context(), id, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, check)
}

func (s *Server) patchItemCheck(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	checkID, err := uuid.Parse(r.PathValue("checkId"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	var body struct {
		Body *string `json:"body"`
		Done *bool   `json:"done"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	check, err := s.App.PatchItemCheck(r.Context(), id, checkID, body.Body, body.Done)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, check)
}

func (s *Server) deleteItemCheck(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	checkID, err := uuid.Parse(r.PathValue("checkId"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	if err := s.App.DeleteItemCheck(r.Context(), id, checkID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.App.SyncItem(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) syncActiveItems(w http.ResponseWriter, r *http.Request) {
	synced, failed, err := s.App.SyncActiveItems(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"synced": synced, "failed": failed})
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteItem(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) undeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.App.UndeleteItem(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) {
	var itemID *uuid.UUID
	if v := r.URL.Query().Get("itemId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		itemID = &id
	}
	notes, err := s.App.ListNotes(r.Context(), itemID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Body   string  `json:"body"`
		ItemID *string `json:"itemId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	var itemID *uuid.UUID
	if body.ItemID != nil && *body.ItemID != "" {
		id, err := uuid.Parse(*body.ItemID)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		itemID = &id
	}
	note, err := s.App.CreateNote(r.Context(), body.Body, itemID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) patchNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	note, err := s.App.PatchNote(r.Context(), id, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteNote(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listIntervals(w http.ResponseWriter, r *http.Request) {
	items, err := s.App.ListOpenIntervals(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) startInterval(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemID    string  `json:"itemId"`
		StartedAt *string `json:"startedAt"`
		EndedAt   *string `json:"endedAt"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	id, err := uuid.Parse(body.ItemID)
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	hasStart := body.StartedAt != nil && *body.StartedAt != ""
	hasEnd := body.EndedAt != nil && *body.EndedAt != ""
	if hasStart != hasEnd {
		writeError(w, domain.ErrInvalid)
		return
	}
	if hasStart {
		started, err := time.Parse(time.RFC3339, *body.StartedAt)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		ended, err := time.Parse(time.RFC3339, *body.EndedAt)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		interval, err := s.App.LogInterval(r.Context(), id, started, ended)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, interval)
		return
	}
	interval, err := s.App.StartInterval(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, interval)
}

func (s *Server) stopInterval(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	interval, err := s.App.StopInterval(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, interval)
}

func (s *Server) createStress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Level  int     `json:"level"`
		ItemID *string `json:"itemId"`
		At     *string `json:"at"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	var itemID *uuid.UUID
	if body.ItemID != nil && *body.ItemID != "" {
		id, err := uuid.Parse(*body.ItemID)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		itemID = &id
	}
	var at *time.Time
	if body.At != nil && *body.At != "" {
		parsed, err := time.Parse(time.RFC3339, *body.At)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		at = &parsed
	}
	log, err := s.App.CreateStress(r.Context(), body.Level, itemID, at)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, log)
}

func (s *Server) createCheckin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind  string `json:"kind"`
		Level int    `json:"level"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	log, err := s.App.CreateCheckin(r.Context(), body.Kind, body.Level)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, log)
}

func (s *Server) latestCheckins(w http.ResponseWriter, r *http.Request) {
	latest, err := s.App.LatestCheckins(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, latest)
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	from, err := parseOptionalTime(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	events, err := s.App.ListEventOccurrences(r.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) listEventSeries(w http.ResponseWriter, r *http.Request) {
	events, err := s.App.ListEvents(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	event, err := s.App.GetEvent(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	write, err := decodeEventWrite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	event, err := s.App.CreateEvent(r.Context(), write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) patchEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	write, err := decodeEventWrite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	event, err := s.App.ReplaceEvent(r.Context(), id, write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteEvent(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getEventOccurrence(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	on, err := parseYmdQuery(r, "on")
	if err != nil {
		writeError(w, err)
		return
	}
	occ, err := s.App.GetEventOccurrence(r.Context(), id, on)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, occ)
}

func (s *Server) putEventOccurrence(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		OriginalOn      string  `json:"originalOn"`
		StartsAt        *string `json:"startsAt"`
		DurationSeconds *int    `json:"durationSeconds"`
		Skipped         bool    `json:"skipped"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	on, err := domain.ParseYmd(body.OriginalOn)
	if err != nil {
		writeError(w, err)
		return
	}
	var startsAt *time.Time
	if body.StartsAt != nil && *body.StartsAt != "" {
		parsed, err := time.Parse(time.RFC3339, *body.StartsAt)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
		startsAt = &parsed
	}
	occ, err := s.App.PutEventOccurrence(r.Context(), id, application.EventOccurrenceWrite{
		OriginalOn:      on,
		StartsAt:        startsAt,
		DurationSeconds: body.DurationSeconds,
		Skipped:         body.Skipped,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, occ)
}

func (s *Server) deleteEventOccurrence(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	on, err := parseYmdQuery(r, "on")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteEventOccurrence(r.Context(), id, on); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listEventNotes(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var on *domain.Ymd
	if raw := strings.TrimSpace(r.URL.Query().Get("on")); raw != "" {
		parsed, err := domain.ParseYmd(raw)
		if err != nil {
			writeError(w, err)
			return
		}
		on = &parsed
	}
	notes, err := s.App.ListEventNotes(r.Context(), id, on)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) createEventNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Body       string `json:"body"`
		OriginalOn string `json:"originalOn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	on, err := domain.ParseYmd(body.OriginalOn)
	if err != nil {
		writeError(w, err)
		return
	}
	note, err := s.App.CreateEventNote(r.Context(), id, on, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) deleteEventNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("noteId"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	if err := s.App.DeleteEventNote(r.Context(), id, noteID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseYmdQuery(r *http.Request, key string) (domain.Ymd, error) {
	return domain.ParseYmd(r.URL.Query().Get(key))
}

func (s *Server) load(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var from, to time.Time
	var err error
	if v := q.Get("from"); v != "" {
		from, err = time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
	}
	if v := q.Get("to"); v != "" {
		to, err = time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, domain.ErrInvalid)
			return
		}
	}
	report, err := s.App.Load(r.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) schedule(w http.ResponseWriter, r *http.Request) {
	from, err := parseOptionalTime(r.URL.Query().Get("from"))
	if err != nil || from.IsZero() {
		writeError(w, domain.ErrInvalid)
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"))
	if err != nil || to.IsZero() {
		writeError(w, domain.ErrInvalid)
		return
	}
	kind, err := domain.ParseScheduleKind(r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, err)
		return
	}
	report, err := s.App.Schedule(r.Context(), from, to, kind)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) listJournal(w http.ResponseWriter, r *http.Request) {
	entries, err := s.App.ListJournal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

type eventBody struct {
	Title             string               `json:"title"`
	Description       string               `json:"description"`
	Agenda            string               `json:"agenda"`
	Kind              string               `json:"kind"`
	Type              string               `json:"type"`
	ProjectID         *string              `json:"projectId"`
	StartsAt          string               `json:"startsAt"`
	DurationSeconds   int                  `json:"durationSeconds"`
	Recurrence        string               `json:"recurrence"`
	RepeatUntil       *string              `json:"repeatUntil"`
	Weekdays          []int                `json:"weekdays"`
	Links             []domain.ProjectLink `json:"links"`
	MeetURL           string               `json:"meetUrl"`
	Involvement       *int                 `json:"involvement"`
	ActiveStartOffset *int                 `json:"activeStartOffset"`
	ActiveEndOffset   *int                 `json:"activeEndOffset"`
	CanSkip           *bool                `json:"canSkip"`
	People            *[]domain.PersonRel  `json:"people"`
}

func decodeEventWrite(r *http.Request) (application.EventWrite, error) {
	var body eventBody
	if err := decodeJSON(r, &body); err != nil {
		return application.EventWrite{}, err
	}
	startsAt, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil {
		return application.EventWrite{}, domain.ErrInvalid
	}
	projectID, err := parseOptionalProjectID(body.ProjectID)
	if err != nil {
		return application.EventWrite{}, err
	}
	people, err := parseOptionalPersonRels(body.People)
	if err != nil {
		return application.EventWrite{}, err
	}
	var until *domain.Ymd
	if body.RepeatUntil != nil && strings.TrimSpace(*body.RepeatUntil) != "" {
		parsed, err := domain.ParseYmd(*body.RepeatUntil)
		if err != nil {
			return application.EventWrite{}, err
		}
		until = &parsed
	}
	return application.EventWrite{
		Title:             body.Title,
		Description:       body.Description,
		Agenda:            body.Agenda,
		Kind:              body.Kind,
		Type:              body.Type,
		ProjectID:         projectID,
		StartsAt:          startsAt,
		DurationSeconds:   body.DurationSeconds,
		Recurrence:        body.Recurrence,
		RepeatUntil:       until,
		Weekdays:          body.Weekdays,
		Links:             body.Links,
		MeetURL:           body.MeetURL,
		Involvement:       body.Involvement,
		ActiveStartOffset: body.ActiveStartOffset,
		ActiveEndOffset:   body.ActiveEndOffset,
		CanSkip:           body.CanSkip,
		People:            people,
	}, nil
}

func (s *Server) listPeople(w http.ResponseWriter, r *http.Request) {
	people, err := s.App.ListPeople(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, people)
}

func (s *Server) getPerson(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	person, err := s.App.GetPerson(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, person)
}

func (s *Server) createPerson(w http.ResponseWriter, r *http.Request) {
	write, err := decodePersonWrite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	person, err := s.App.CreatePerson(r.Context(), write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, person)
}

func (s *Server) patchPerson(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	write, err := decodePersonWrite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	person, err := s.App.ReplacePerson(r.Context(), id, write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, person)
}

func (s *Server) deletePerson(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeletePerson(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listPersonNotes(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	notes, err := s.App.ListPersonNotes(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) createPersonNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	note, err := s.App.CreatePersonNote(r.Context(), id, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) patchPersonNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("noteId"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	note, err := s.App.ReplacePersonNote(r.Context(), id, noteID, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) deletePersonNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("noteId"))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	if err := s.App.DeletePersonNote(r.Context(), id, noteID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createPersonContact(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonContact(r)
	if err != nil {
		writeError(w, err)
		return
	}
	contact, err := s.App.CreatePersonContact(r.Context(), id, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, contact)
}

func (s *Server) patchPersonContact(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	contactID, err := parseNamedID(r, "contactId")
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonContact(r)
	if err != nil {
		writeError(w, err)
		return
	}
	contact, err := s.App.ReplacePersonContact(r.Context(), id, contactID, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contact)
}

func (s *Server) deletePersonContact(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	contactID, err := parseNamedID(r, "contactId")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeletePersonContact(r.Context(), id, contactID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createPersonProfession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	write, err := decodePersonProfession(r)
	if err != nil {
		writeError(w, err)
		return
	}
	prof, err := s.App.CreatePersonProfession(r.Context(), id, write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, prof)
}

func (s *Server) patchPersonProfession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	professionID, err := parseNamedID(r, "professionId")
	if err != nil {
		writeError(w, err)
		return
	}
	write, err := decodePersonProfession(r)
	if err != nil {
		writeError(w, err)
		return
	}
	prof, err := s.App.ReplacePersonProfession(r.Context(), id, professionID, write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prof)
}

func (s *Server) deletePersonProfession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	professionID, err := parseNamedID(r, "professionId")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeletePersonProfession(r.Context(), id, professionID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createPersonSite(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonSite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	site, err := s.App.CreatePersonSite(r.Context(), id, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, site)
}

func (s *Server) patchPersonSite(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	siteID, err := parseNamedID(r, "siteId")
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonSite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	site, err := s.App.ReplacePersonSite(r.Context(), id, siteID, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, site)
}

func (s *Server) deletePersonSite(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	siteID, err := parseNamedID(r, "siteId")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeletePersonSite(r.Context(), id, siteID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createPersonBond(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		OtherID   *string `json:"otherId"`
		Kind      string  `json:"kind"`
		Comment   string  `json:"comment"`
		StartedOn *string `json:"startedOn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	other, err := parseOptionalOtherID(body.OtherID)
	if err != nil {
		writeError(w, err)
		return
	}
	startedOn, err := parseOptionalDate(body.StartedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	bond, err := s.App.OpenBond(r.Context(), id, application.BondOpen{
		OtherID:   other,
		Kind:      body.Kind,
		Comment:   body.Comment,
		StartedOn: startedOn,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, bond)
}

func (s *Server) patchPersonBond(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	bondID, err := parseNamedID(r, "bondId")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Kind          string  `json:"kind"`
		Comment       *string `json:"comment"`
		ActionComment string  `json:"actionComment"`
		StartedOn     *string `json:"startedOn"`
		ChangedOn     *string `json:"changedOn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	startedOn, err := parseOptionalDate(body.StartedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	changedOn, err := parseOptionalDate(body.ChangedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	bond, err := s.App.ChangeBond(r.Context(), id, bondID, application.BondPatch{
		BondChange: domain.BondChange{
			Kind:      body.Kind,
			Comment:   body.Comment,
			StartedOn: startedOn,
			ChangedOn: changedOn,
		},
		ActionComment: body.ActionComment,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bond)
}

func (s *Server) endPersonBond(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	bondID, err := parseNamedID(r, "bondId")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		EndedOn *string `json:"endedOn"`
		Comment string  `json:"comment"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	endedOn, err := parseOptionalDate(body.EndedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	bond, err := s.App.EndBond(r.Context(), id, bondID, application.BondEnd{
		EndedOn: endedOn,
		Comment: body.Comment,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bond)
}

func decodePersonContact(r *http.Request) (domain.PersonContactDraft, error) {
	var body struct {
		Kind  string `json:"kind"`
		Label string `json:"label"`
		Value string `json:"value"`
		Note  string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return domain.PersonContactDraft{}, err
	}
	return domain.PersonContactDraft{Kind: body.Kind, Label: body.Label, Value: body.Value, Note: body.Note}, nil
}

func decodePersonSite(r *http.Request) (domain.PersonSiteDraft, error) {
	var body struct {
		Kind    string `json:"kind"`
		URL     string `json:"url"`
		Comment string `json:"comment"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return domain.PersonSiteDraft{}, err
	}
	return domain.PersonSiteDraft{Kind: body.Kind, URL: body.URL, Comment: body.Comment}, nil
}

func decodePersonProfession(r *http.Request) (application.ProfessionWrite, error) {
	var body struct {
		Title            string  `json:"title"`
		Comment          string  `json:"comment"`
		StartedOn        *string `json:"startedOn"`
		EndedOn          *string `json:"endedOn"`
		MonthlySalaryUSD float64 `json:"monthlySalaryUsd"`
		MonthlySalaryRUB float64 `json:"monthlySalaryRub"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return application.ProfessionWrite{}, err
	}
	startedOn, err := parseOptionalDate(body.StartedOn)
	if err != nil {
		return application.ProfessionWrite{}, err
	}
	endedOn, err := parseOptionalDate(body.EndedOn)
	if err != nil {
		return application.ProfessionWrite{}, err
	}
	return application.ProfessionWrite{
		Title:            body.Title,
		Comment:          body.Comment,
		StartedOn:        startedOn,
		EndedOn:          endedOn,
		MonthlySalaryUSD: body.MonthlySalaryUSD,
		MonthlySalaryRUB: body.MonthlySalaryRUB,
	}, nil
}

func parseOptionalOtherID(raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, domain.ErrInvalid
	}
	return &id, nil
}

func decodePersonWrite(r *http.Request) (application.PersonWrite, error) {
	var body struct {
		Name      string             `json:"name"`
		BornOn    *string            `json:"bornOn"`
		AgeYears  *int               `json:"ageYears"`
		Projects  []domain.PersonRel `json:"projects"`
		Events    []domain.PersonRel `json:"events"`
		Companies []domain.PersonRel `json:"companies"`
		ItemIDs   []string           `json:"itemIds"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return application.PersonWrite{}, err
	}
	bornOn, err := parseOptionalDate(body.BornOn)
	if err != nil {
		return application.PersonWrite{}, err
	}
	projects, err := domain.NormalizePersonRels(body.Projects)
	if err != nil {
		return application.PersonWrite{}, err
	}
	events, err := domain.NormalizePersonRels(body.Events)
	if err != nil {
		return application.PersonWrite{}, err
	}
	items, err := parseIDList(body.ItemIDs)
	if err != nil {
		return application.PersonWrite{}, err
	}
	return application.PersonWrite{
		Name:      body.Name,
		BornOn:    bornOn,
		AgeYears:  body.AgeYears,
		Projects:  projects,
		Events:    events,
		Companies: domain.NormalizeOptionalRels(body.Companies),
		ItemIDs:   items,
	}, nil
}

func parseOptionalDate(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	if day, err := time.Parse("2006-01-02", *raw); err == nil {
		return &day, nil
	}
	stamp, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil, domain.ErrInvalid
	}
	return &stamp, nil
}

func parseIDList(raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, value := range raw {
		if value == "" {
			continue
		}
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, domain.ErrInvalid
		}
		out = append(out, id)
	}
	return out, nil
}

func parseOptionalIDList(raw *[]string) (*[]uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	ids, err := parseIDList(*raw)
	if err != nil {
		return nil, err
	}
	return &ids, nil
}

func parseOptionalPersonRels(raw *[]domain.PersonRel) (*[]domain.PersonRel, error) {
	if raw == nil {
		return nil, nil
	}
	rels, err := domain.NormalizePersonRels(*raw)
	if err != nil {
		return nil, err
	}
	return &rels, nil
}

func parseOptionalProjectID(raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, domain.ErrInvalid
	}
	return &id, nil
}

func parseOptionalTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func parseID(r *http.Request) (uuid.UUID, error) {
	return parseNamedID(r, "id")
}

func parseNamedID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, domain.ErrInvalid
	}
	return id, nil
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil {
		return domain.ErrInvalid
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusUnauthorized
		msg = "unauthorized"
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		msg = "not found"
	case errors.Is(err, domain.ErrInvalid):
		status = http.StatusBadRequest
		msg = err.Error()
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrSourceReadOnly):
		status = http.StatusConflict
		msg = err.Error()
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrUnsupported), errors.Is(err, domain.ErrNoToken):
		status = http.StatusForbidden
		msg = err.Error()
	}
	if status == http.StatusInternalServerError && strings.Contains(err.Error(), "HTTP") {
		status = http.StatusBadGateway
		msg = err.Error()
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
