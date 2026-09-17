package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CompanyNote struct {
	ID        uuid.UUID `json:"id"`
	CompanyID uuid.UUID `json:"companyId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewCompanyNote(companyID uuid.UUID, body string) (CompanyNote, error) {
	body = strings.TrimSpace(body)
	if companyID == uuid.Nil {
		return CompanyNote{}, fmt.Errorf("%w: company", ErrInvalid)
	}
	if body == "" {
		return CompanyNote{}, fmt.Errorf("%w: note body", ErrInvalid)
	}
	return CompanyNote{
		ID:        uuid.New(),
		CompanyID: companyID,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (n *CompanyNote) Apply(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("%w: note body", ErrInvalid)
	}
	n.Body = body
	return nil
}
