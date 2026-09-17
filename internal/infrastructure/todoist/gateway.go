package todoist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pchkauu/want-brief/internal/domain"
)

type Gateway struct {
	Client *http.Client
}

func New() *Gateway {
	return &Gateway{Client: &http.Client{Timeout: 25 * time.Second}}
}

type task struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	Description string `json:"description"`
	IsCompleted bool   `json:"is_completed"`
	Due         *struct {
		Date string `json:"date"`
	} `json:"due"`
	Priority int `json:"priority"`
}

type comment struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	PostedAt string `json:"posted_at"`
}

func (g *Gateway) Probe(ctx context.Context, source domain.Source, token string) error {
	base := strings.TrimRight(source.BaseURL, "/")
	if base == "" {
		base = "https://api.todoist.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/rest/v2/projects", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return fmt.Errorf("todoist probe: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("todoist probe: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (g *Gateway) Pull(ctx context.Context, source domain.Source, token string) ([]domain.RemoteItem, error) {
	base := strings.TrimRight(source.BaseURL, "/")
	if base == "" {
		base = "https://api.todoist.com"
	}
	endpoint := base + "/rest/v2/tasks"
	if filter := strings.TrimSpace(source.QueryFilter); filter != "" {
		endpoint += "?filter=" + url.QueryEscape(filter)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("todoist tasks: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("todoist tasks: HTTP %d", resp.StatusCode)
	}
	var tasks []task
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return nil, fmt.Errorf("todoist decode: %w", err)
	}
	out := make([]domain.RemoteItem, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, mapTask(task))
	}
	return out, nil
}

func (g *Gateway) Fetch(ctx context.Context, source domain.Source, token, externalKey string) (domain.RemoteItem, error) {
	id := strings.TrimSpace(externalKey)
	if id == "" {
		return domain.RemoteItem{}, fmt.Errorf("%w: external key", domain.ErrInvalid)
	}
	base := strings.TrimRight(source.BaseURL, "/")
	if base == "" {
		base = "https://api.todoist.com"
	}
	raw, err := g.get(ctx, token, base+"/rest/v2/tasks/"+url.PathEscape(id))
	if err != nil {
		return domain.RemoteItem{}, err
	}
	var row task
	if err := json.Unmarshal(raw, &row); err != nil {
		return domain.RemoteItem{}, fmt.Errorf("todoist decode: %w", err)
	}
	item := mapTask(row)
	commentsRaw, err := g.get(ctx, token, base+"/rest/v2/comments?task_id="+url.QueryEscape(id))
	if err != nil {
		return domain.RemoteItem{}, err
	}
	var comments []comment
	if err := json.Unmarshal(commentsRaw, &comments); err != nil {
		return domain.RemoteItem{}, fmt.Errorf("todoist comments: %w", err)
	}
	for _, row := range comments {
		body := strings.TrimSpace(row.Content)
		if body == "" {
			continue
		}
		mapped := domain.RemoteComment{ExternalID: strings.TrimSpace(row.ID), Body: body}
		if posted, err := time.Parse(time.RFC3339, row.PostedAt); err == nil {
			mapped.CreatedAt = &posted
		}
		item.Comments = append(item.Comments, mapped)
	}
	return item, nil
}

func (g *Gateway) get(ctx context.Context, token, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("todoist get: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("todoist get: HTTP %d", resp.StatusCode)
	}
	return raw, nil
}

func mapTask(row task) domain.RemoteItem {
	item := domain.RemoteItem{
		ExternalKey:    row.ID,
		Title:          row.Content,
		Status:         domain.StatusBacklog,
		ExternalStatus: "Open",
		Description:    strings.TrimSpace(row.Description),
		HintImportant:  row.Priority >= 3,
		HintUrgent:     row.Priority == 4,
	}
	if row.IsCompleted {
		item.Status = domain.StatusDone
		item.ExternalStatus = "Completed"
	}
	if row.Due != nil && row.Due.Date != "" {
		if due, err := parseDue(row.Due.Date); err == nil {
			item.DueAt = &due
			if !due.After(time.Now().UTC()) {
				item.HintUrgent = true
			}
		}
	}
	return item
}

func parseDue(raw string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, raw)
}
