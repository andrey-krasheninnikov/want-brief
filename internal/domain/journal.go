package domain

import "time"

type NoteSource string

const (
	NoteSourceLoose   NoteSource = "loose"
	NoteSourceProject NoteSource = "project"
	NoteSourceItem    NoteSource = "item"
	NoteSourcePerson  NoteSource = "person"
)

type JournalEntry struct {
	ID        string     `json:"id"`
	Source    NoteSource `json:"source"`
	OwnerID   *string    `json:"ownerId"`
	OwnerName string     `json:"ownerName"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
}
