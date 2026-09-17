package jira

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pchkauu/want-brief/internal/domain"
)

const maxSearchPages = 50

type Gateway struct {
	Client *http.Client
}

func New() *Gateway {
	return &Gateway{Client: &http.Client{
		Timeout: 25 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

type searchRequest struct {
	JQL           string   `json:"jql"`
	Fields        []string `json:"fields"`
	MaxResults    int      `json:"maxResults"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

type jiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		DueDate     string          `json:"duedate"`
		Created     string          `json:"created"`
		Priority    *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Status struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		Comment *struct {
			Comments []struct {
				ID     string          `json:"id"`
				Body   json.RawMessage `json:"body"`
				Author *struct {
					DisplayName string `json:"displayName"`
				} `json:"author"`
				Created string `json:"created"`
			} `json:"comments"`
		} `json:"comment"`
	} `json:"fields"`
}

type searchResponse struct {
	Issues        []jiraIssue `json:"issues"`
	NextPageToken string      `json:"nextPageToken"`
	IsLast        bool        `json:"isLast"`
	StartAt       int         `json:"startAt"`
	Total         int         `json:"total"`
}

type serverSearchRequest struct {
	JQL        string   `json:"jql"`
	Fields     []string `json:"fields"`
	StartAt    int      `json:"startAt"`
	MaxResults int      `json:"maxResults"`
}

var jiraFields = []string{"summary", "status", "priority", "duedate", "created"}

func (g *Gateway) Probe(ctx context.Context, source domain.Source, token string) error {
	base, err := jiraBase(source)
	if err != nil {
		return err
	}
	path := "/rest/api/2/myself"
	if isJiraCloud(source.BaseURL) {
		path = "/rest/api/3/myself"
	}
	status, raw, err := g.do(ctx, source, http.MethodGet, base+path, token, nil)
	if err != nil {
		return fmt.Errorf("jira probe: %w", err)
	}
	if status >= 300 {
		return jiraHTTPError("jira probe", status, raw)
	}
	if looksLikeHTML(raw) {
		return fmt.Errorf("jira probe: HTML response")
	}
	return nil
}

func (g *Gateway) Pull(ctx context.Context, source domain.Source, token string) ([]domain.RemoteItem, error) {
	base, err := jiraBase(source)
	if err != nil {
		return nil, err
	}
	if isJiraCloud(source.BaseURL) {
		return g.pullCloud(ctx, source, token, base)
	}
	return g.pullServer(ctx, source, token, base)
}

func (g *Gateway) Fetch(ctx context.Context, source domain.Source, token, externalKey string) (domain.RemoteItem, error) {
	key := strings.TrimSpace(externalKey)
	if key == "" {
		return domain.RemoteItem{}, fmt.Errorf("%w: external key", domain.ErrInvalid)
	}
	base, err := jiraBase(source)
	if err != nil {
		return domain.RemoteItem{}, err
	}
	api := "/rest/api/2/issue/"
	if isJiraCloud(source.BaseURL) {
		api = "/rest/api/3/issue/"
	}
	fields := url.Values{"fields": []string{strings.Join(append(jiraFields, "description", "comment"), ",")}}
	status, raw, err := g.do(ctx, source, http.MethodGet, base+api+url.PathEscape(key)+"?"+fields.Encode(), token, nil)
	if err != nil {
		return domain.RemoteItem{}, fmt.Errorf("jira issue: %w", err)
	}
	if status >= 300 {
		return domain.RemoteItem{}, jiraHTTPError("jira issue", status, raw)
	}
	if looksLikeHTML(raw) {
		return domain.RemoteItem{}, fmt.Errorf("jira issue: HTML response")
	}
	var issue jiraIssue
	if err := json.Unmarshal(raw, &issue); err != nil {
		return domain.RemoteItem{}, fmt.Errorf("jira issue decode: %w", err)
	}
	return mapIssueDetailed(base, issue), nil
}

func (g *Gateway) pullCloud(ctx context.Context, source domain.Source, token, base string) ([]domain.RemoteItem, error) {
	var out []domain.RemoteItem
	pageToken := ""
	// ponytail: 50 pages * 100 issues; raise if a personal inbox exceeds 5000 open tickets
	for range maxSearchPages {
		parsed, err := g.searchCloud(ctx, source, token, pageToken)
		if err != nil {
			return nil, err
		}
		for _, issue := range parsed.Issues {
			out = append(out, mapIssue(base, issue))
		}
		if parsed.IsLast || parsed.NextPageToken == "" {
			return out, nil
		}
		pageToken = parsed.NextPageToken
	}
	return out, nil
}

func (g *Gateway) pullServer(ctx context.Context, source domain.Source, token, base string) ([]domain.RemoteItem, error) {
	var out []domain.RemoteItem
	startAt := 0
	for range maxSearchPages {
		parsed, err := g.searchServer(ctx, source, token, startAt)
		if err != nil {
			return nil, err
		}
		for _, issue := range parsed.Issues {
			out = append(out, mapIssue(base, issue))
		}
		startAt += len(parsed.Issues)
		if len(parsed.Issues) == 0 || startAt >= parsed.Total {
			return out, nil
		}
	}
	return out, nil
}

func (g *Gateway) searchCloud(ctx context.Context, source domain.Source, token, pageToken string) (searchResponse, error) {
	base, err := jiraBase(source)
	if err != nil {
		return searchResponse{}, err
	}
	body, err := json.Marshal(searchRequest{
		JQL:           domain.DefaultQuery(domain.SourceJira),
		Fields:        jiraFields,
		MaxResults:    100,
		NextPageToken: pageToken,
	})
	if err != nil {
		return searchResponse{}, err
	}
	return g.decodeSearch(ctx, source, token, base+"/rest/api/3/search/jql", body)
}

func (g *Gateway) searchServer(ctx context.Context, source domain.Source, token string, startAt int) (searchResponse, error) {
	base, err := jiraBase(source)
	if err != nil {
		return searchResponse{}, err
	}
	body, err := json.Marshal(serverSearchRequest{
		JQL:        domain.DefaultQuery(domain.SourceJira),
		Fields:     jiraFields,
		StartAt:    startAt,
		MaxResults: 100,
	})
	if err != nil {
		return searchResponse{}, err
	}
	return g.decodeSearch(ctx, source, token, base+"/rest/api/2/search", body)
}

func (g *Gateway) decodeSearch(ctx context.Context, source domain.Source, token, url string, body []byte) (searchResponse, error) {
	status, raw, err := g.do(ctx, source, http.MethodPost, url, token, body)
	if err != nil {
		return searchResponse{}, fmt.Errorf("jira search: %w", err)
	}
	if status >= 300 {
		return searchResponse{}, jiraHTTPError("jira search", status, raw)
	}
	if looksLikeHTML(raw) {
		return searchResponse{}, fmt.Errorf("jira decode: HTML from search")
	}
	var parsed searchResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return searchResponse{}, fmt.Errorf("jira decode: %w", err)
	}
	return parsed, nil
}

func (g *Gateway) do(ctx context.Context, source domain.Source, method, url, token string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := applyAuth(req, source, token); err != nil {
		return 0, nil, err
	}
	resp, err := g.httpClient(source).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, raw, nil
}

func (g *Gateway) httpClient(source domain.Source) *http.Client {
	if g.Client == nil || !source.InsecureTLS {
		return g.Client
	}
	clone := *g.Client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	clone.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // ponytail: per-source flag for broken corporate CA
	}
	return &clone
}

func looksLikeHTML(raw []byte) bool {
	trim := bytes.TrimSpace(raw)
	return bytes.HasPrefix(trim, []byte("<"))
}

func mapIssue(base string, issue jiraIssue) domain.RemoteItem {
	item := domain.RemoteItem{
		ExternalKey: issue.Key,
		Title:       issue.Fields.Summary,
		Status:      domain.StatusBacklog,
		URL:         base + "/browse/" + issue.Key,
	}
	if issue.Fields.Priority != nil {
		name := strings.ToLower(issue.Fields.Priority.Name)
		item.HintImportant = name == "highest" || name == "high"
		item.HintUrgent = name == "highest" || name == "blocker"
	}
	if issue.Fields.DueDate != "" {
		if due, err := time.Parse("2006-01-02", issue.Fields.DueDate); err == nil {
			item.DueAt = &due
			if !due.After(time.Now().UTC()) {
				item.HintUrgent = true
			}
		}
	}
	if created, err := parseJiraTime(issue.Fields.Created); err == nil {
		item.CreatedAt = &created
	}
	return item
}

func mapIssueDetailed(base string, issue jiraIssue) domain.RemoteItem {
	item := mapIssue(base, issue)
	item.ExternalStatus = strings.TrimSpace(issue.Fields.Status.Name)
	item.Description = jiraText(issue.Fields.Description)
	if issue.Fields.Comment == nil {
		return item
	}
	for _, comment := range issue.Fields.Comment.Comments {
		body := jiraText(comment.Body)
		if body == "" {
			continue
		}
		mapped := domain.RemoteComment{ExternalID: strings.TrimSpace(comment.ID), Body: body}
		if comment.Author != nil {
			mapped.Author = strings.TrimSpace(comment.Author.DisplayName)
		}
		if mapped.ExternalID != "" {
			mapped.URL = item.URL + "?focusedCommentId=" + url.QueryEscape(mapped.ExternalID)
		}
		if created, err := parseJiraTime(comment.Created); err == nil {
			mapped.CreatedAt = &created
		}
		item.Comments = append(item.Comments, mapped)
	}
	return item
}

func jiraText(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return strings.TrimSpace(plain)
	}
	var node map[string]any
	if json.Unmarshal(raw, &node) != nil {
		return ""
	}
	return strings.TrimSpace(walkADF(node))
}

func walkADF(node map[string]any) string {
	if text, ok := node["text"].(string); ok {
		return text
	}
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, child := range content {
		next, ok := child.(map[string]any)
		if !ok {
			continue
		}
		if piece := walkADF(next); piece != "" {
			parts = append(parts, piece)
		}
	}
	return strings.Join(parts, "\n")
}

func jiraBase(source domain.Source) (string, error) {
	base := strings.TrimRight(source.BaseURL, "/")
	if base == "" {
		return "", fmt.Errorf("%w: jira base url", domain.ErrInvalid)
	}
	return base, nil
}

func applyAuth(req *http.Request, source domain.Source, token string) error {
	if isJiraCloud(source.BaseURL) {
		email := strings.TrimSpace(source.Email)
		if email == "" {
			return fmt.Errorf("%w: jira email", domain.ErrInvalid)
		}
		req.SetBasicAuth(email, token)
		return nil
	}
	if email, rest, ok := strings.Cut(token, ":"); ok && email != "" && rest != "" {
		req.SetBasicAuth(email, rest)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func isJiraCloud(base string) bool {
	return strings.Contains(strings.ToLower(base), ".atlassian.net")
}

func jiraHTTPError(op string, status int, raw []byte) error {
	msg := fmt.Sprintf("%s: HTTP %d", op, status)
	var parsed struct {
		ErrorMessages []string `json:"errorMessages"`
	}
	if json.Unmarshal(raw, &parsed) != nil || len(parsed.ErrorMessages) == 0 {
		return fmt.Errorf("%s", msg)
	}
	detail := strings.TrimSpace(parsed.ErrorMessages[0])
	if detail == "" {
		return fmt.Errorf("%s", msg)
	}
	if len(detail) > 200 {
		detail = detail[:200]
	}
	return fmt.Errorf("%s (%s)", msg, detail)
}

func parseJiraTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.999-0700",
		"2006-01-02T15:04:05-0700",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("jira created")
}
