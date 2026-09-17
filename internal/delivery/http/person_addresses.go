package httpserver

import (
	"net/http"

	"github.com/pchkauu/want-brief/internal/domain"
)

func decodePersonAddress(r *http.Request) (domain.PersonAddressDraft, error) {
	var body struct {
		Kind  string `json:"kind"`
		Label string `json:"label"`
		Value string `json:"value"`
		Note  string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return domain.PersonAddressDraft{}, err
	}
	return domain.PersonAddressDraft{Kind: body.Kind, Label: body.Label, Value: body.Value, Note: body.Note}, nil
}

func (s *Server) createPersonAddress(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonAddress(r)
	if err != nil {
		writeError(w, err)
		return
	}
	address, err := s.App.CreatePersonAddress(r.Context(), id, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, address)
}

func (s *Server) patchPersonAddress(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	addressID, err := parseNamedID(r, "addressId")
	if err != nil {
		writeError(w, err)
		return
	}
	draft, err := decodePersonAddress(r)
	if err != nil {
		writeError(w, err)
		return
	}
	address, err := s.App.ReplacePersonAddress(r.Context(), id, addressID, draft)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, address)
}

func (s *Server) deletePersonAddress(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	addressID, err := parseNamedID(r, "addressId")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.App.DeletePersonAddress(r.Context(), id, addressID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
