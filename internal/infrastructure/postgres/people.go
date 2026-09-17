package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const personCols = `id, name, born_on, created_at, updated_at`

func scanPerson(scan func(dest ...any) error) (domain.Person, error) {
	var p domain.Person
	err := scan(
		&p.ID, &p.Name, &p.BornOn, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return p, err
	}
	return p, nil
}

func (s *Store) GetPerson(ctx context.Context, id uuid.UUID) (domain.Person, error) {
	p, err := scanPerson(s.pool.QueryRow(ctx, `SELECT `+personCols+` FROM people WHERE id=$1`, id).Scan)
	if err != nil {
		return domain.Person{}, mapErr(err)
	}
	out := []domain.Person{p}
	if err := s.attachPersonLinks(ctx, out); err != nil {
		return domain.Person{}, err
	}
	if err := s.attachPersonDossier(ctx, &out[0]); err != nil {
		return domain.Person{}, err
	}
	return out[0].WithAge(time.Now().UTC()), nil
}

func (s *Store) ListPeople(ctx context.Context) ([]domain.Person, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+personCols+` FROM people ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Person
	for rows.Next() {
		p, err := scanPerson(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachPersonLinks(ctx, out); err != nil {
		return nil, err
	}
	if err := s.attachPersonListExtras(ctx, out); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range out {
		out[i] = out[i].WithAge(now)
	}
	if out == nil {
		return []domain.Person{}, nil
	}
	return out, nil
}

func (s *Store) CreatePerson(ctx context.Context, p domain.Person) (domain.Person, error) {
	links := p
	p, err := scanPerson(s.pool.QueryRow(ctx, `
		INSERT INTO people (`+personCols+`)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+personCols+`
	`, p.ID, p.Name, p.BornOn, p.CreatedAt, p.UpdatedAt).Scan)
	if err != nil {
		return domain.Person{}, err
	}
	p.Projects = links.Projects
	p.Events = links.Events
	p.Companies = links.Companies
	p.ItemIDs = links.ItemIDs
	if err := s.replacePersonLinks(ctx, p); err != nil {
		return domain.Person{}, err
	}
	return s.GetPerson(ctx, p.ID)
}

func (s *Store) UpdatePerson(ctx context.Context, p domain.Person) (domain.Person, error) {
	links := p
	_, err := scanPerson(s.pool.QueryRow(ctx, `
		UPDATE people
		SET name=$2, born_on=$3, updated_at=$4
		WHERE id=$1
		RETURNING `+personCols+`
	`, p.ID, p.Name, p.BornOn, p.UpdatedAt).Scan)
	if err != nil {
		return domain.Person{}, mapErr(err)
	}
	if err := s.replacePersonLinks(ctx, links); err != nil {
		return domain.Person{}, err
	}
	return s.GetPerson(ctx, links.ID)
}

func (s *Store) DeletePerson(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM people WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) attachPersonLinks(ctx context.Context, people []domain.Person) error {
	if len(people) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(people))
	index := make(map[uuid.UUID]int, len(people))
	for i, p := range people {
		ids[i] = p.ID
		index[p.ID] = i
		people[i].Projects = []domain.PersonRel{}
		people[i].Events = []domain.PersonRel{}
		people[i].Companies = []domain.PersonRel{}
		people[i].ItemIDs = []uuid.UUID{}
	}
	projects, err := s.relsByOwners(ctx, "person_projects", "person_id", "project_id", ids)
	if err != nil {
		return err
	}
	events, err := s.relsByOwners(ctx, "person_events", "person_id", "event_id", ids)
	if err != nil {
		return err
	}
	items, err := s.linksByOwners(ctx, "person_items", "person_id", "item_id", ids)
	if err != nil {
		return err
	}
	companies, err := s.relsByOwners(ctx, "company_people", "person_id", "company_id", ids)
	if err != nil {
		return err
	}
	for id, list := range projects {
		people[index[id]].Projects = list
	}
	for id, list := range events {
		people[index[id]].Events = list
	}
	for id, list := range items {
		people[index[id]].ItemIDs = list
	}
	for id, list := range companies {
		people[index[id]].Companies = list
	}
	return nil
}

func (s *Store) replacePersonLinks(ctx context.Context, p domain.Person) error {
	if err := s.replaceRels(ctx, "person_projects", "person_id", "project_id", p.ID, p.Projects); err != nil {
		return err
	}
	if err := s.replaceRels(ctx, "person_events", "person_id", "event_id", p.ID, p.Events); err != nil {
		return err
	}
	if err := s.replaceRels(ctx, "company_people", "person_id", "company_id", p.ID, p.Companies); err != nil {
		return err
	}
	return s.replaceLinks(ctx, "person_items", "person_id", "item_id", p.ID, p.ItemIDs)
}

func (s *Store) attachPersonListExtras(ctx context.Context, people []domain.Person) error {
	if len(people) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(people))
	index := make(map[uuid.UUID]int, len(people))
	for i, p := range people {
		ids[i] = p.ID
		index[p.ID] = i
		people[i].Contacts = []domain.PersonContact{}
		people[i].Sites = []domain.PersonSite{}
		people[i].Bonds = []domain.PersonBond{}
		people[i].Professions = []domain.PersonProfession{}
		people[i].Absences = []domain.PersonAbsence{}
	}
	contacts, err := s.contactsByPeople(ctx, ids)
	if err != nil {
		return err
	}
	absences, err := s.absencesByPeople(ctx, ids)
	if err != nil {
		return err
	}
	sites, err := s.sitesByPeople(ctx, ids)
	if err != nil {
		return err
	}
	meBonds, err := s.meBondsByPeople(ctx, ids)
	if err != nil {
		return err
	}
	notes, err := s.lastNotesByPeople(ctx, ids)
	if err != nil {
		return err
	}
	profs, err := s.professionsByPeople(ctx, ids, true)
	if err != nil {
		return err
	}
	for id, list := range contacts {
		people[index[id]].Contacts = list
	}
	for id, list := range sites {
		people[index[id]].Sites = list
	}
	for id, list := range profs {
		people[index[id]].Professions = list
	}
	for id, list := range absences {
		people[index[id]].Absences = list
	}
	for id, bond := range meBonds {
		b := bond
		people[index[id]].MeBond = &b
	}
	for id, at := range notes {
		stamp := at
		people[index[id]].LastNoteAt = &stamp
	}
	return nil
}

func (s *Store) lastNotesByPeople(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	out := map[uuid.UUID]time.Time{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT person_id, max(created_at)
		FROM person_notes
		WHERE person_id = ANY($1)
		GROUP BY person_id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = at
	}
	return out, rows.Err()
}

func (s *Store) attachPersonIDs(ctx context.Context, table, ownerCol string, owners []uuid.UUID, set func(uuid.UUID, []uuid.UUID)) error {
	by, err := s.linksByOwners(ctx, table, ownerCol, "person_id", owners)
	if err != nil {
		return err
	}
	for _, id := range owners {
		set(id, emptyIDs(by[id]))
	}
	return nil
}

func (s *Store) relsByOwners(ctx context.Context, table, ownerCol, otherCol string, owners []uuid.UUID) (map[uuid.UUID][]domain.PersonRel, error) {
	out := map[uuid.UUID][]domain.PersonRel{}
	if len(owners) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`SELECT %s, %s, comment FROM %s WHERE %s = ANY($1)`, ownerCol, otherCol, table, ownerCol)
	rows, err := s.pool.Query(ctx, q, owners)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner, other uuid.UUID
		var comment string
		if err := rows.Scan(&owner, &other, &comment); err != nil {
			return nil, err
		}
		out[owner] = append(out[owner], domain.PersonRel{ID: other, Comment: comment})
	}
	return out, rows.Err()
}

func (s *Store) replaceRels(ctx context.Context, table, ownerCol, otherCol string, owner uuid.UUID, rels []domain.PersonRel) error {
	del := fmt.Sprintf(`DELETE FROM %s WHERE %s=$1`, table, ownerCol)
	if _, err := s.pool.Exec(ctx, del, owner); err != nil {
		return err
	}
	if len(rels) == 0 {
		return nil
	}
	ins := fmt.Sprintf(`INSERT INTO %s (%s, %s, comment) VALUES ($1, $2, $3)`, table, ownerCol, otherCol)
	for _, rel := range rels {
		if _, err := s.pool.Exec(ctx, ins, owner, rel.ID, rel.Comment); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) attachRels(ctx context.Context, table, ownerCol, otherCol string, owners []uuid.UUID, set func(uuid.UUID, []domain.PersonRel)) error {
	by, err := s.relsByOwners(ctx, table, ownerCol, otherCol, owners)
	if err != nil {
		return err
	}
	for _, id := range owners {
		set(id, emptyRels(by[id]))
	}
	return nil
}

func emptyRels(rels []domain.PersonRel) []domain.PersonRel {
	if rels == nil {
		return []domain.PersonRel{}
	}
	return rels
}

func (s *Store) linksByOwners(ctx context.Context, table, ownerCol, otherCol string, owners []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := map[uuid.UUID][]uuid.UUID{}
	if len(owners) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`SELECT %s, %s FROM %s WHERE %s = ANY($1)`, ownerCol, otherCol, table, ownerCol)
	rows, err := s.pool.Query(ctx, q, owners)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner, other uuid.UUID
		if err := rows.Scan(&owner, &other); err != nil {
			return nil, err
		}
		out[owner] = append(out[owner], other)
	}
	return out, rows.Err()
}

func (s *Store) replaceLinks(ctx context.Context, table, ownerCol, otherCol string, owner uuid.UUID, others []uuid.UUID) error {
	del := fmt.Sprintf(`DELETE FROM %s WHERE %s=$1`, table, ownerCol)
	if _, err := s.pool.Exec(ctx, del, owner); err != nil {
		return err
	}
	others = domain.NormalizeIDs(others)
	if len(others) == 0 {
		return nil
	}
	ins := fmt.Sprintf(`INSERT INTO %s (%s, %s) VALUES ($1, $2)`, table, ownerCol, otherCol)
	for _, id := range others {
		if _, err := s.pool.Exec(ctx, ins, owner, id); err != nil {
			return err
		}
	}
	return nil
}

func emptyIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

func (s *Store) GetPersonNote(ctx context.Context, id uuid.UUID) (domain.PersonNote, error) {
	var n domain.PersonNote
	err := s.pool.QueryRow(ctx, `
		SELECT id, person_id, body, created_at FROM person_notes WHERE id=$1
	`, id).Scan(&n.ID, &n.PersonID, &n.Body, &n.CreatedAt)
	return n, mapErr(err)
}

func (s *Store) ListPersonNotes(ctx context.Context, personID uuid.UUID) ([]domain.PersonNote, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, person_id, body, created_at
		FROM person_notes WHERE person_id=$1
		ORDER BY created_at DESC
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PersonNote{}
	for rows.Next() {
		var n domain.PersonNote
		if err := rows.Scan(&n.ID, &n.PersonID, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreatePersonNote(ctx context.Context, n domain.PersonNote) (domain.PersonNote, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO person_notes (id, person_id, body, created_at)
		VALUES ($1,$2,$3,$4)
		RETURNING id, person_id, body, created_at
	`, n.ID, n.PersonID, n.Body, n.CreatedAt).
		Scan(&n.ID, &n.PersonID, &n.Body, &n.CreatedAt)
	return n, err
}

func (s *Store) UpdatePersonNote(ctx context.Context, n domain.PersonNote) (domain.PersonNote, error) {
	err := s.pool.QueryRow(ctx, `
		UPDATE person_notes SET body=$2 WHERE id=$1
		RETURNING id, person_id, body, created_at
	`, n.ID, n.Body).Scan(&n.ID, &n.PersonID, &n.Body, &n.CreatedAt)
	return n, mapErr(err)
}

func (s *Store) DeletePersonNote(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM person_notes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type PersonRepo struct{ *Store }

func (r PersonRepo) Get(ctx context.Context, id uuid.UUID) (domain.Person, error) {
	return r.Store.GetPerson(ctx, id)
}
func (r PersonRepo) List(ctx context.Context) ([]domain.Person, error) {
	return r.Store.ListPeople(ctx)
}
func (r PersonRepo) Create(ctx context.Context, person domain.Person) (domain.Person, error) {
	return r.Store.CreatePerson(ctx, person)
}
func (r PersonRepo) Update(ctx context.Context, person domain.Person) (domain.Person, error) {
	return r.Store.UpdatePerson(ctx, person)
}
func (r PersonRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeletePerson(ctx, id)
}

type PersonNoteRepo struct{ *Store }

func (r PersonNoteRepo) Get(ctx context.Context, id uuid.UUID) (domain.PersonNote, error) {
	return r.Store.GetPersonNote(ctx, id)
}
func (r PersonNoteRepo) ListByPerson(ctx context.Context, personID uuid.UUID) ([]domain.PersonNote, error) {
	return r.Store.ListPersonNotes(ctx, personID)
}
func (r PersonNoteRepo) Create(ctx context.Context, note domain.PersonNote) (domain.PersonNote, error) {
	return r.Store.CreatePersonNote(ctx, note)
}
func (r PersonNoteRepo) Update(ctx context.Context, note domain.PersonNote) (domain.PersonNote, error) {
	return r.Store.UpdatePersonNote(ctx, note)
}
func (r PersonNoteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeletePersonNote(ctx, id)
}
