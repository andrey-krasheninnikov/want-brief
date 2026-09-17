package httpserver

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/application"
	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Server) listCompanies(w http.ResponseWriter, r *http.Request) {
	rows, err := s.App.ListCompanies(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) createCompany(w http.ResponseWriter, r *http.Request) {
	write, err := decodeCompanyWrite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	company, err := s.App.CreateCompany(r.Context(), write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, company)
}

func (s *Server) getCompany(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	company, err := s.App.GetCompany(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (s *Server) patchCompany(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	write, err := decodeCompanyWrite(r)
	if err != nil {
		writeError(w, err)
		return
	}
	company, err := s.App.ReplaceCompany(r.Context(), id, write)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (s *Server) deleteCompany(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteCompany(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createCompanyTitle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Title     string  `json:"title"`
		StartedOn *string `json:"startedOn"`
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
	company, err := s.App.AddCompanyTitle(r.Context(), id, body.Title, startedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (s *Server) createCompanySalary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Currency  string  `json:"currency"`
		Amount    float64 `json:"amount"`
		Comment   string  `json:"comment"`
		StartedOn *string `json:"startedOn"`
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
	company, err := s.App.AddCompanySalary(r.Context(), id, body.Currency, body.Amount, startedOn, body.Comment)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (s *Server) putCompanyManager(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		PersonID  *string `json:"personId"`
		StartedOn *string `json:"startedOn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	personID, err := parseOptionalOtherID(body.PersonID)
	if err != nil {
		writeError(w, err)
		return
	}
	startedOn, err := parseOptionalDate(body.StartedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	company, err := s.App.SetCompanyManager(r.Context(), id, personID, startedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (s *Server) createCompanyReport(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		PersonID  string  `json:"personId"`
		StartedOn *string `json:"startedOn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	personID, err := parseNamedRawID(body.PersonID)
	if err != nil {
		writeError(w, err)
		return
	}
	startedOn, err := parseOptionalDate(body.StartedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	company, err := s.App.AddCompanyReport(r.Context(), id, personID, startedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, company)
}

func (s *Server) deleteCompanyReport(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	reportID, err := parseNamedID(r, "reportId")
	if err != nil {
		writeError(w, err)
		return
	}
	company, err := s.App.EndCompanyReport(r.Context(), id, reportID, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (s *Server) createCompanyContract(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		PersonID  *string `json:"personId"`
		Kind      string  `json:"kind"`
		StartedOn *string `json:"startedOn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	personID, err := parseOptionalOtherID(body.PersonID)
	if err != nil {
		writeError(w, err)
		return
	}
	startedOn, err := parseOptionalDate(body.StartedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	company, err := s.App.SetCompanyContract(r.Context(), id, personID, body.Kind, startedOn)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (s *Server) listCompanyNotes(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	notes, err := s.App.ListCompanyNotes(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) createCompanyNote(w http.ResponseWriter, r *http.Request) {
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
	note, err := s.App.CreateCompanyNote(r.Context(), id, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (s *Server) patchCompanyNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	noteID, err := parseNamedID(r, "noteId")
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
	note, err := s.App.ReplaceCompanyNote(r.Context(), id, noteID, body.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) deleteCompanyNote(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	noteID, err := parseNamedID(r, "noteId")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeleteCompanyNote(r.Context(), id, noteID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeCompanyWrite(r *http.Request) (application.CompanyWrite, error) {
	var body struct {
		Name        string               `json:"name"`
		Description string               `json:"description"`
		Links       []domain.ProjectLink `json:"links"`
		Projects    []domain.PersonRel   `json:"projects"`
		Events      []domain.PersonRel   `json:"events"`
		People      []domain.PersonRel   `json:"people"`
		StartedOn   *string              `json:"startedOn"`
		EndedOn     *string              `json:"endedOn"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return application.CompanyWrite{}, err
	}
	startedOn, err := parseOptionalDate(body.StartedOn)
	if err != nil {
		return application.CompanyWrite{}, err
	}
	endedOn, err := parseOptionalDate(body.EndedOn)
	if err != nil {
		return application.CompanyWrite{}, err
	}
	projects, err := domain.NormalizePersonRels(body.Projects)
	if err != nil {
		return application.CompanyWrite{}, err
	}
	events, err := domain.NormalizePersonRels(body.Events)
	if err != nil {
		return application.CompanyWrite{}, err
	}
	return application.CompanyWrite{
		Name:        body.Name,
		Description: body.Description,
		Links:       body.Links,
		Projects:    projects,
		Events:      events,
		People:      domain.NormalizeOptionalRels(body.People),
		StartedOn:   startedOn,
		EndedOn:     endedOn,
	}, nil
}

func parseNamedRawID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, domain.ErrInvalid
	}
	return id, nil
}
