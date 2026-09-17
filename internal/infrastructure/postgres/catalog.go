package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) (domain.User, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, password_hash, created_at)
		VALUES ($1, $2, $3)
		RETURNING id, password_hash, created_at
	`, user.ID, user.PasswordHash, user.CreatedAt).Scan(&user.ID, &user.PasswordHash, &user.CreatedAt)
	return user, err
}

func (s *Store) FirstUser(ctx context.Context) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, password_hash, created_at FROM users ORDER BY created_at LIMIT 1
	`).Scan(&user.ID, &user.PasswordHash, &user.CreatedAt)
	return user, mapErr(err)
}

func (s *Store) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.CreatedAt)
	return err
}

func (s *Store) GetByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	var session domain.Session
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM sessions WHERE token_hash = $1
	`, tokenHash).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.ExpiresAt, &session.CreatedAt)
	return session, mapErr(err)
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	return err
}

type UserRepo struct{ *Store }

func (r UserRepo) Count(ctx context.Context) (int, error) { return r.Store.Count(ctx) }
func (r UserRepo) Create(ctx context.Context, user domain.User) (domain.User, error) {
	return r.Store.CreateUser(ctx, user)
}
func (r UserRepo) First(ctx context.Context) (domain.User, error) { return r.Store.FirstUser(ctx) }

type SessionRepo struct{ *Store }

func (r SessionRepo) Create(ctx context.Context, session domain.Session) error {
	return r.Store.CreateSession(ctx, session)
}
func (r SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	return r.Store.GetByTokenHash(ctx, tokenHash)
}
func (r SessionRepo) Delete(ctx context.Context, tokenHash string) error {
	return r.Store.DeleteSession(ctx, tokenHash)
}
func (r SessionRepo) DeleteExpired(ctx context.Context, now time.Time) error {
	return r.Store.DeleteExpired(ctx, now)
}

const projectCols = `id, name, color, description, monthly_income_usd, monthly_income_rub, target_hours_day, target_hours_week, links, archived_at, created_at, updated_at`

func encodeProjectLinks(links []domain.ProjectLink) ([]byte, error) {
	if links == nil {
		links = []domain.ProjectLink{}
	}
	return json.Marshal(links)
}

func decodeProjectLinks(raw []byte) ([]domain.ProjectLink, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []domain.ProjectLink{}, nil
	}
	var links []domain.ProjectLink
	if err := json.Unmarshal(raw, &links); err != nil {
		return nil, err
	}
	if links == nil {
		return []domain.ProjectLink{}, nil
	}
	return links, nil
}

func scanProject(scan func(dest ...any) error) (domain.Project, error) {
	var p domain.Project
	var raw []byte
	err := scan(
		&p.ID,
		&p.Name,
		&p.Color,
		&p.Description,
		&p.MonthlyIncomeUSD,
		&p.MonthlyIncomeRUB,
		&p.TargetHoursDay,
		&p.TargetHoursWeek,
		&raw,
		&p.ArchivedAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return p, err
	}
	p.Links, err = decodeProjectLinks(raw)
	return p, err
}

func (s *Store) attachProjectLinks(ctx context.Context, rows []domain.Project) error {
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, p := range rows {
		ids[i] = p.ID
		index[p.ID] = i
		rows[i].People = []domain.PersonRel{}
		rows[i].Companies = []domain.PersonRel{}
	}
	if err := s.attachRels(ctx, "person_projects", "project_id", "person_id", ids, func(id uuid.UUID, people []domain.PersonRel) {
		rows[index[id]].People = people
	}); err != nil {
		return err
	}
	return s.attachRels(ctx, "company_projects", "project_id", "company_id", ids, func(id uuid.UUID, companies []domain.PersonRel) {
		rows[index[id]].Companies = companies
	})
}

func (s *Store) GetProject(ctx context.Context, id uuid.UUID) (domain.Project, error) {
	p, err := scanProject(s.pool.QueryRow(ctx, `SELECT `+projectCols+` FROM projects WHERE id = $1`, id).Scan)
	if err != nil {
		return p, mapErr(err)
	}
	out := []domain.Project{p}
	if err := s.attachProjectLinks(ctx, out); err != nil {
		return domain.Project{}, err
	}
	return out[0], nil
}

func (s *Store) ListProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+projectCols+` FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachProjectLinks(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CreateProject(ctx context.Context, p domain.Project) (domain.Project, error) {
	raw, err := encodeProjectLinks(p.Links)
	if err != nil {
		return domain.Project{}, err
	}
	people := p.People
	companies := p.Companies
	p, err = scanProject(s.pool.QueryRow(ctx, `
		INSERT INTO projects (`+projectCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+projectCols+`
	`, p.ID, p.Name, p.Color, p.Description, p.MonthlyIncomeUSD, p.MonthlyIncomeRUB, p.TargetHoursDay, p.TargetHoursWeek, raw, p.ArchivedAt, p.CreatedAt, p.UpdatedAt).Scan)
	if err != nil {
		return domain.Project{}, err
	}
	if err := s.replaceRels(ctx, "person_projects", "project_id", "person_id", p.ID, people); err != nil {
		return domain.Project{}, err
	}
	if err := s.replaceRels(ctx, "company_projects", "project_id", "company_id", p.ID, companies); err != nil {
		return domain.Project{}, err
	}
	return s.GetProject(ctx, p.ID)
}

func (s *Store) UpdateProject(ctx context.Context, p domain.Project) (domain.Project, error) {
	raw, err := encodeProjectLinks(p.Links)
	if err != nil {
		return domain.Project{}, err
	}
	people := p.People
	companies := p.Companies
	p, err = scanProject(s.pool.QueryRow(ctx, `
		UPDATE projects
		SET name=$2, color=$3, description=$4, monthly_income_usd=$5, monthly_income_rub=$6, target_hours_day=$7, target_hours_week=$8, links=$9, archived_at=$10, updated_at=$11
		WHERE id=$1
		RETURNING `+projectCols+`
	`, p.ID, p.Name, p.Color, p.Description, p.MonthlyIncomeUSD, p.MonthlyIncomeRUB, p.TargetHoursDay, p.TargetHoursWeek, raw, p.ArchivedAt, p.UpdatedAt).Scan)
	if err != nil {
		return domain.Project{}, mapErr(err)
	}
	if err := s.replaceRels(ctx, "person_projects", "project_id", "person_id", p.ID, people); err != nil {
		return domain.Project{}, err
	}
	if err := s.replaceRels(ctx, "company_projects", "project_id", "company_id", p.ID, companies); err != nil {
		return domain.Project{}, err
	}
	return s.GetProject(ctx, p.ID)
}

func (s *Store) DeleteProject(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type ProjectRepo struct{ *Store }

func (r ProjectRepo) Get(ctx context.Context, id uuid.UUID) (domain.Project, error) {
	return r.Store.GetProject(ctx, id)
}
func (r ProjectRepo) List(ctx context.Context) ([]domain.Project, error) {
	return r.Store.ListProjects(ctx)
}
func (r ProjectRepo) Create(ctx context.Context, project domain.Project) (domain.Project, error) {
	return r.Store.CreateProject(ctx, project)
}
func (r ProjectRepo) Update(ctx context.Context, project domain.Project) (domain.Project, error) {
	return r.Store.UpdateProject(ctx, project)
}
func (r ProjectRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteProject(ctx, id)
}

const sourceCols = `id, kind, name, email, base_url, token_sealed, query_filter, last_sync_at, project_id, connected, last_error, insecure_tls, created_at, updated_at`

func scanSource(row interface{ Scan(dest ...any) error }, src *domain.Source) error {
	var kind string
	err := row.Scan(
		&src.ID, &kind, &src.Name, &src.Email, &src.BaseURL, &src.TokenSealed, &src.QueryFilter, &src.LastSyncAt,
		&src.ProjectID, &src.Connected, &src.LastError, &src.InsecureTLS, &src.CreatedAt, &src.UpdatedAt,
	)
	src.Kind = domain.SourceKind(kind)
	src.HasToken = src.TokenSealed != ""
	return err
}

func (s *Store) GetSource(ctx context.Context, id uuid.UUID) (domain.Source, error) {
	var src domain.Source
	err := scanSource(s.pool.QueryRow(ctx, `SELECT `+sourceCols+` FROM sources WHERE id=$1`, id), &src)
	return src, mapErr(err)
}

func (s *Store) ListSources(ctx context.Context) ([]domain.Source, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+sourceCols+` FROM sources ORDER BY kind, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Source
	for rows.Next() {
		var src domain.Source
		if err := scanSource(rows, &src); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) CreateSource(ctx context.Context, src domain.Source) (domain.Source, error) {
	err := scanSource(s.pool.QueryRow(ctx, `
		INSERT INTO sources (`+sourceCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING `+sourceCols+`
	`, src.ID, string(src.Kind), src.Name, src.Email, src.BaseURL, src.TokenSealed, src.QueryFilter, src.LastSyncAt,
		src.ProjectID, src.Connected, src.LastError, src.InsecureTLS, src.CreatedAt, src.UpdatedAt), &src)
	return src, err
}

func (s *Store) UpdateSource(ctx context.Context, src domain.Source) (domain.Source, error) {
	err := scanSource(s.pool.QueryRow(ctx, `
		UPDATE sources
		SET name=$2, email=$3, base_url=$4, token_sealed=$5, query_filter=$6, last_sync_at=$7,
		    project_id=$8, connected=$9, last_error=$10, insecure_tls=$11, updated_at=$12
		WHERE id=$1
		RETURNING `+sourceCols+`
	`, src.ID, src.Name, src.Email, src.BaseURL, src.TokenSealed, src.QueryFilter, src.LastSyncAt,
		src.ProjectID, src.Connected, src.LastError, src.InsecureTLS, src.UpdatedAt), &src)
	return src, mapErr(err)
}

func (s *Store) DeleteSource(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sources WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ManualSource(ctx context.Context) (domain.Source, error) {
	var src domain.Source
	err := scanSource(s.pool.QueryRow(ctx, `
		SELECT `+sourceCols+` FROM sources WHERE kind='manual' ORDER BY created_at LIMIT 1
	`), &src)
	return src, mapErr(err)
}

type SourceRepo struct{ *Store }

func (r SourceRepo) Get(ctx context.Context, id uuid.UUID) (domain.Source, error) {
	return r.Store.GetSource(ctx, id)
}
func (r SourceRepo) List(ctx context.Context) ([]domain.Source, error) {
	return r.Store.ListSources(ctx)
}
func (r SourceRepo) Create(ctx context.Context, source domain.Source) (domain.Source, error) {
	return r.Store.CreateSource(ctx, source)
}
func (r SourceRepo) Update(ctx context.Context, source domain.Source) (domain.Source, error) {
	return r.Store.UpdateSource(ctx, source)
}
func (r SourceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteSource(ctx, id)
}
func (r SourceRepo) Manual(ctx context.Context) (domain.Source, error) {
	return r.Store.ManualSource(ctx)
}

func scanItem(scanner interface {
	Scan(dest ...any) error
}) (domain.Item, error) {
	var item domain.Item
	var kind, status, sourceKind string
	var linksRaw []byte
	var occupancy string
	err := scanner.Scan(
		&item.ID, &item.SourceID, &item.ExternalKey, &item.Title, &status, &kind,
		&item.ProjectID, &item.Urgent, &item.Important, &item.Pinned, &item.Stress, &item.DueAt,
		&item.DevDueAt, &item.ReviewDueAt, &item.TestDueAt,
		&item.Description, &item.PlannedSeconds, &linksRaw, &item.TrackedSeconds,
		&item.ArchivedAt, &item.DeletedAt, &item.CreatedAt, &item.UpdatedAt, &item.SourceName, &sourceKind, &item.ProjectName, &item.ProjectColor,
		&occupancy, &item.ExternalStatus, &item.CheckTotal, &item.CheckDone, &item.PinnedAt,
	)
	if err != nil {
		return item, err
	}
	item.Status = domain.ItemStatus(status)
	item.Kind = domain.ItemKind(kind)
	item.SourceKind = domain.SourceKind(sourceKind)
	item.Occupancy, _ = domain.ParseOccupancy(occupancy)
	item.Links, err = decodeProjectLinks(linksRaw)
	if err != nil {
		return item, err
	}
	return item.WithQuadrant(), nil
}

const itemSelect = `
SELECT
	i.id, i.source_id, i.external_key, i.title, i.status, i.kind,
	i.project_id, i.urgent, i.important, i.pinned, i.stress, i.due_at,
	i.dev_due_at, i.review_due_at, i.test_due_at,
	i.description, i.planned_seconds, i.links,
	COALESCE(t.tracked_seconds, 0),
	i.archived_at, i.deleted_at, i.created_at, i.updated_at,
	s.name, s.kind,
	COALESCE(p.name, ''), COALESCE(p.color, ''),
	i.occupancy, i.external_status,
	COALESCE((SELECT COUNT(*)::int FROM item_checks c WHERE c.item_id = i.id), 0),
	COALESCE((SELECT COUNT(*)::int FROM item_checks c WHERE c.item_id = i.id AND c.done), 0),
	i.pinned_at
FROM items i
JOIN sources s ON s.id = i.source_id
LEFT JOIN projects p ON p.id = i.project_id
LEFT JOIN (
	SELECT item_id,
		EXTRACT(EPOCH FROM SUM(ended_at - started_at))::bigint AS tracked_seconds
	FROM time_intervals
	WHERE ended_at IS NOT NULL
	GROUP BY item_id
) t ON t.item_id = i.id
`

func (s *Store) attachItemPeople(ctx context.Context, rows []domain.Item) error {
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, item := range rows {
		ids[i] = item.ID
		index[item.ID] = i
		rows[i].PersonIDs = []uuid.UUID{}
	}
	return s.attachPersonIDs(ctx, "person_items", "item_id", ids, func(id uuid.UUID, people []uuid.UUID) {
		rows[index[id]].PersonIDs = people
	})
}

func (s *Store) GetItem(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	row := s.pool.QueryRow(ctx, itemSelect+` WHERE i.id=$1`, id)
	item, err := scanItem(row)
	if err != nil {
		return item, mapErr(err)
	}
	out := []domain.Item{item}
	if err := s.attachItemPeople(ctx, out); err != nil {
		return domain.Item{}, err
	}
	return out[0], nil
}

func (s *Store) ListItems(ctx context.Context, filter domain.ItemFilter) ([]domain.Item, error) {
	query := itemSelect + ` WHERE i.deleted_at IS NULL`
	args := []any{}
	n := 1
	if filter.SourceID != nil {
		query += fmt.Sprintf(` AND i.source_id=$%d`, n)
		args = append(args, *filter.SourceID)
		n++
	}
	if filter.ProjectID != nil {
		query += fmt.Sprintf(` AND i.project_id=$%d`, n)
		args = append(args, *filter.ProjectID)
		n++
	}
	if filter.Kind != nil {
		query += fmt.Sprintf(` AND i.kind=$%d`, n)
		args = append(args, string(*filter.Kind))
		n++
	}
	if filter.Status != nil {
		query += fmt.Sprintf(` AND i.status=$%d`, n)
		args = append(args, string(*filter.Status))
		n++
	}
	if filter.OpenOnly {
		query += ` AND i.status NOT IN ('done', 'cancelled')`
	}
	if filter.ArchivedOnly {
		query += ` AND i.archived_at IS NOT NULL`
	} else if !filter.IncludeArchived {
		query += ` AND i.archived_at IS NULL`
	}
	query += ` ORDER BY i.updated_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachItemPeople(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CreateItem(ctx context.Context, item domain.Item) (domain.Item, error) {
	raw, err := encodeProjectLinks(item.Links)
	if err != nil {
		return domain.Item{}, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO items (
			id, source_id, external_key, title, status, kind, project_id,
			urgent, important, pinned, stress, due_at, dev_due_at, review_due_at, test_due_at,
			description, planned_seconds, links, occupancy, external_status,
			created_at, updated_at, pinned_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
	`, item.ID, item.SourceID, item.ExternalKey, item.Title, string(item.Status), string(item.Kind),
		item.ProjectID, item.Urgent, item.Important, item.Pinned, item.Stress, item.DueAt, item.DevDueAt, item.ReviewDueAt, item.TestDueAt,
		item.Description, item.PlannedSeconds, raw, string(item.EffectiveOccupancy()), item.ExternalStatus, item.CreatedAt, item.UpdatedAt, item.PinnedAt)
	if err != nil {
		return domain.Item{}, err
	}
	if err := s.replaceLinks(ctx, "person_items", "item_id", "person_id", item.ID, item.PersonIDs); err != nil {
		return domain.Item{}, err
	}
	return s.GetItem(ctx, item.ID)
}

func (s *Store) UpdateItem(ctx context.Context, item domain.Item) (domain.Item, error) {
	raw, err := encodeProjectLinks(item.Links)
	if err != nil {
		return domain.Item{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE items SET
			title=$2, status=$3, kind=$4, project_id=$5, urgent=$6, important=$7, pinned=$8,
			stress=$9, due_at=$10, dev_due_at=$11, review_due_at=$12, test_due_at=$13,
			description=$14, planned_seconds=$15, links=$16, external_key=$17, archived_at=$18, deleted_at=$19, updated_at=$20,
			occupancy=$21, external_status=$22, pinned_at=$23
		WHERE id=$1
	`, item.ID, item.Title, string(item.Status), string(item.Kind), item.ProjectID, item.Urgent, item.Important, item.Pinned,
		item.Stress, item.DueAt, item.DevDueAt, item.ReviewDueAt, item.TestDueAt, item.Description, item.PlannedSeconds, raw,
		item.ExternalKey, item.ArchivedAt, item.DeletedAt, item.UpdatedAt, string(item.EffectiveOccupancy()), item.ExternalStatus, item.PinnedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Item{}, domain.ErrConflict
		}
		return domain.Item{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Item{}, domain.ErrNotFound
	}
	if err := s.replaceLinks(ctx, "person_items", "item_id", "person_id", item.ID, item.PersonIDs); err != nil {
		return domain.Item{}, err
	}
	return s.GetItem(ctx, item.ID)
}

func (s *Store) DeleteItem(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM items WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) UpsertSynced(ctx context.Context, item domain.Item) (domain.Item, error) {
	raw, err := encodeProjectLinks(item.Links)
	if err != nil {
		return domain.Item{}, err
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO items (
			id, source_id, external_key, title, status, kind, project_id,
			urgent, important, pinned, stress, due_at, dev_due_at, review_due_at, test_due_at,
			description, planned_seconds, links, occupancy, external_status,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		ON CONFLICT (source_id, external_key) WHERE external_key <> ''
		DO UPDATE SET
			title = CASE WHEN btrim(EXCLUDED.title) = '' THEN items.title ELSE EXCLUDED.title END,
			due_at = EXCLUDED.due_at,
			project_id = EXCLUDED.project_id,
			links = EXCLUDED.links,
			created_at = LEAST(items.created_at, EXCLUDED.created_at),
			updated_at = EXCLUDED.updated_at
		WHERE items.deleted_at IS NULL
		RETURNING id
	`, uuid.New(), item.SourceID, item.ExternalKey, item.Title, string(item.Status), string(item.Kind),
		item.ProjectID, item.Urgent, item.Important, item.Pinned, item.Stress, item.DueAt, item.DevDueAt, item.ReviewDueAt, item.TestDueAt,
		item.Description, item.PlannedSeconds, raw, string(domain.OccupancySolo), item.ExternalStatus, item.CreatedAt, item.UpdatedAt).Scan(&id)
	if err != nil {
		return domain.Item{}, err
	}
	return s.GetItem(ctx, id)
}

type ItemRepo struct{ *Store }

func (r ItemRepo) Get(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	return r.Store.GetItem(ctx, id)
}
func (r ItemRepo) List(ctx context.Context, filter domain.ItemFilter) ([]domain.Item, error) {
	return r.Store.ListItems(ctx, filter)
}
func (r ItemRepo) Create(ctx context.Context, item domain.Item) (domain.Item, error) {
	return r.Store.CreateItem(ctx, item)
}
func (r ItemRepo) Update(ctx context.Context, item domain.Item) (domain.Item, error) {
	return r.Store.UpdateItem(ctx, item)
}
func (r ItemRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteItem(ctx, id)
}
func (r ItemRepo) UpsertSynced(ctx context.Context, item domain.Item) (domain.Item, error) {
	return r.Store.UpsertSynced(ctx, item)
}
