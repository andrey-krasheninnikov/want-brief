package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const personBondCols = `id, person_a_id, person_b_id, kind, comment, started_on, changed_on, ended_on, created_at, updated_at`

func scanPersonBond(scan func(dest ...any) error) (domain.PersonBond, error) {
	var b domain.PersonBond
	err := scan(
		&b.ID, &b.PersonAID, &b.PersonBID, &b.Kind, &b.Comment,
		&b.StartedOn, &b.ChangedOn, &b.EndedOn, &b.CreatedAt, &b.UpdatedAt,
	)
	if b.Events == nil {
		b.Events = []domain.PersonBondEvent{}
	}
	return b, err
}

func (s *Store) GetPersonBond(ctx context.Context, id uuid.UUID) (domain.PersonBond, error) {
	bond, err := scanPersonBond(s.pool.QueryRow(ctx, `SELECT `+personBondCols+` FROM person_bonds WHERE id=$1`, id).Scan)
	if err != nil {
		return domain.PersonBond{}, mapErr(err)
	}
	events, err := s.listBondEvents(ctx, []uuid.UUID{bond.ID})
	if err != nil {
		return domain.PersonBond{}, err
	}
	bond.Events = events[bond.ID]
	if bond.Events == nil {
		bond.Events = []domain.PersonBondEvent{}
	}
	return bond, nil
}

func (s *Store) ListPersonBonds(ctx context.Context, personID uuid.UUID) ([]domain.PersonBond, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personBondCols+`
		FROM person_bonds
		WHERE person_b_id = $1 OR person_a_id = $1
		ORDER BY ended_on NULLS FIRST, changed_on DESC, created_at DESC
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PersonBond{}
	for rows.Next() {
		bond, err := scanPersonBond(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, bond)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.finishPersonBonds(ctx, personID, out)
}

func (s *Store) finishPersonBonds(ctx context.Context, personID uuid.UUID, bonds []domain.PersonBond) ([]domain.PersonBond, error) {
	if len(bonds) == 0 {
		return []domain.PersonBond{}, nil
	}
	ids := make([]uuid.UUID, len(bonds))
	for i, bond := range bonds {
		ids[i] = bond.ID
	}
	events, err := s.listBondEvents(ctx, ids)
	if err != nil {
		return nil, err
	}
	names, err := s.bondOtherNames(ctx, bonds)
	if err != nil {
		return nil, err
	}
	for i, bond := range bonds {
		bond.Events = events[bond.ID]
		if bond.Events == nil {
			bond.Events = []domain.PersonBondEvent{}
		}
		bond = bond.ViewedFrom(personID)
		if bond.OtherID != nil {
			bond.OtherName = names[*bond.OtherID]
		}
		bonds[i] = bond
	}
	return bonds, nil
}

func (s *Store) bondOtherNames(ctx context.Context, bonds []domain.PersonBond) (map[uuid.UUID]string, error) {
	ids := make([]uuid.UUID, 0, len(bonds)*2)
	seen := map[uuid.UUID]struct{}{}
	for _, bond := range bonds {
		if bond.PersonAID != nil {
			if _, ok := seen[*bond.PersonAID]; !ok {
				seen[*bond.PersonAID] = struct{}{}
				ids = append(ids, *bond.PersonAID)
			}
		}
		if _, ok := seen[bond.PersonBID]; !ok {
			seen[bond.PersonBID] = struct{}{}
			ids = append(ids, bond.PersonBID)
		}
	}
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM people WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func (s *Store) listBondEvents(ctx context.Context, bondIDs []uuid.UUID) (map[uuid.UUID][]domain.PersonBondEvent, error) {
	out := map[uuid.UUID][]domain.PersonBondEvent{}
	if len(bondIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, bond_id, action, kind, comment, started_on, changed_on, ended_on, at
		FROM person_bond_events
		WHERE bond_id = ANY($1)
		ORDER BY at, id
	`, bondIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e domain.PersonBondEvent
		if err := rows.Scan(&e.ID, &e.BondID, &e.Action, &e.Kind, &e.Comment, &e.StartedOn, &e.ChangedOn, &e.EndedOn, &e.At); err != nil {
			return nil, err
		}
		out[e.BondID] = append(out[e.BondID], e)
	}
	return out, rows.Err()
}

func (s *Store) CreatePersonBond(ctx context.Context, bond domain.PersonBond, event domain.PersonBondEvent) (domain.PersonBond, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PersonBond{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = scanPersonBond(tx.QueryRow(ctx, `
		INSERT INTO person_bonds (`+personBondCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+personBondCols+`
	`, bond.ID, bond.PersonAID, bond.PersonBID, bond.Kind, bond.Comment, bond.StartedOn, bond.ChangedOn, bond.EndedOn, bond.CreatedAt, bond.UpdatedAt).Scan)
	if err != nil {
		return domain.PersonBond{}, mapErr(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO person_bond_events (id, bond_id, action, kind, comment, started_on, changed_on, ended_on, at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, event.ID, event.BondID, event.Action, event.Kind, event.Comment, event.StartedOn, event.ChangedOn, event.EndedOn, event.At); err != nil {
		return domain.PersonBond{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PersonBond{}, err
	}
	return s.GetPersonBond(ctx, bond.ID)
}

func (s *Store) UpdatePersonBond(ctx context.Context, bond domain.PersonBond, event domain.PersonBondEvent) (domain.PersonBond, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PersonBond{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = scanPersonBond(tx.QueryRow(ctx, `
		UPDATE person_bonds
		SET kind=$2, comment=$3, started_on=$4, changed_on=$5, ended_on=$6, updated_at=$7
		WHERE id=$1
		RETURNING `+personBondCols+`
	`, bond.ID, bond.Kind, bond.Comment, bond.StartedOn, bond.ChangedOn, bond.EndedOn, bond.UpdatedAt).Scan)
	if err != nil {
		return domain.PersonBond{}, mapErr(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO person_bond_events (id, bond_id, action, kind, comment, started_on, changed_on, ended_on, at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, event.ID, event.BondID, event.Action, event.Kind, event.Comment, event.StartedOn, event.ChangedOn, event.EndedOn, event.At); err != nil {
		return domain.PersonBond{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PersonBond{}, err
	}
	return s.GetPersonBond(ctx, bond.ID)
}

func (s *Store) attachPersonDossier(ctx context.Context, person *domain.Person) error {
	contacts, err := s.ListPersonContacts(ctx, person.ID)
	if err != nil {
		return err
	}
	sites, err := s.ListPersonSites(ctx, person.ID)
	if err != nil {
		return err
	}
	bonds, err := s.ListPersonBonds(ctx, person.ID)
	if err != nil {
		return err
	}
	profs, err := s.ListPersonProfessions(ctx, person.ID)
	if err != nil {
		return err
	}
	absences, err := s.ListPersonAbsences(ctx, person.ID)
	if err != nil {
		return err
	}
	person.Contacts = contacts
	person.Sites = sites
	person.Bonds = bonds
	person.Professions = profs
	person.Absences = absences
	for _, bond := range bonds {
		if bond.PersonAID == nil && bond.EndedOn == nil {
			person.MeBond = &domain.MeBond{ID: bond.ID, Kind: bond.Kind}
			break
		}
	}
	var last *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT max(created_at) FROM person_notes WHERE person_id=$1`, person.ID).Scan(&last); err != nil {
		return err
	}
	person.LastNoteAt = last
	return nil
}

func (s *Store) meBondsByPeople(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.MeBond, error) {
	out := map[uuid.UUID]domain.MeBond{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, person_b_id, kind
		FROM person_bonds
		WHERE person_a_id IS NULL AND ended_on IS NULL AND person_b_id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bond domain.MeBond
		var personID uuid.UUID
		if err := rows.Scan(&bond.ID, &personID, &bond.Kind); err != nil {
			return nil, err
		}
		out[personID] = bond
	}
	return out, rows.Err()
}

type PersonBondRepo struct{ *Store }

func (r PersonBondRepo) Get(ctx context.Context, id uuid.UUID) (domain.PersonBond, error) {
	return r.Store.GetPersonBond(ctx, id)
}
func (r PersonBondRepo) ListByPerson(ctx context.Context, personID uuid.UUID) ([]domain.PersonBond, error) {
	return r.Store.ListPersonBonds(ctx, personID)
}
func (r PersonBondRepo) Create(ctx context.Context, bond domain.PersonBond, event domain.PersonBondEvent) (domain.PersonBond, error) {
	return r.Store.CreatePersonBond(ctx, bond, event)
}
func (r PersonBondRepo) Update(ctx context.Context, bond domain.PersonBond, event domain.PersonBondEvent) (domain.PersonBond, error) {
	return r.Store.UpdatePersonBond(ctx, bond, event)
}
