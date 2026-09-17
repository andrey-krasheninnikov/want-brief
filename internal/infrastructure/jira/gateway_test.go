package jira

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pchkauu/want-brief/internal/domain"
)

func TestPullMapsIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/search" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer pat-1" {
			t.Fatalf("auth %s", got)
		}
		raw, _ := io.ReadAll(r.Body)
		var body serverSearchRequest
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		if body.JQL != domain.DefaultQuery(domain.SourceJira) {
			t.Fatalf("jql %q", body.JQL)
		}
		if body.StartAt != 0 {
			t.Fatalf("start %d", body.StartAt)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"startAt": 0,
			"total":   1,
			"issues": []map[string]any{
				{
					"key": "WB-1",
					"fields": map[string]any{
						"summary":  "Ship inbox",
						"created":  "2020-03-15T09:30:00.000+0000",
						"duedate":  "2020-01-01",
						"priority": map[string]string{"name": "High"},
						"status": map[string]any{
							"statusCategory": map[string]string{"key": "indeterminate"},
						},
					},
				},
			},
		})
	}))
	t.Cleanup(server.Close)

	g := New()
	g.Client = server.Client()
	items, err := g.Pull(t.Context(), domain.Source{BaseURL: server.URL, QueryFilter: "project = NOPE"}, "pat-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ExternalKey != "WB-1" {
		t.Fatalf("%+v", items)
	}
	if items[0].Status != domain.StatusBacklog {
		t.Fatalf("status %s", items[0].Status)
	}
	if items[0].URL != server.URL+"/browse/WB-1" {
		t.Fatalf("url %s", items[0].URL)
	}
	if !items[0].HintUrgent || !items[0].HintImportant {
		t.Fatalf("hints %+v", items[0])
	}
	if items[0].CreatedAt == nil || !items[0].CreatedAt.Equal(time.Date(2020, 3, 15, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("created %+v", items[0].CreatedAt)
	}
}

func TestPullPagesUntilLast(t *testing.T) {
	var seen []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/search" {
			t.Fatalf("path %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var body serverSearchRequest
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		seen = append(seen, body.StartAt)
		if body.StartAt == 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"startAt": 0,
				"total":   2,
				"issues": []map[string]any{
					{"key": "WB-1", "fields": map[string]any{"summary": "One"}},
				},
			})
			return
		}
		if body.StartAt != 1 {
			t.Fatalf("start %d", body.StartAt)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"startAt": 1,
			"total":   2,
			"issues": []map[string]any{
				{"key": "WB-2", "fields": map[string]any{"summary": "Two"}},
			},
		})
	}))
	t.Cleanup(server.Close)

	g := New()
	g.Client = server.Client()
	items, err := g.Pull(t.Context(), domain.Source{BaseURL: server.URL}, "pat-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ExternalKey != "WB-1" || items[1].ExternalKey != "WB-2" {
		t.Fatalf("%+v", items)
	}
	if items[1].URL != server.URL+"/browse/WB-2" {
		t.Fatalf("url %s", items[1].URL)
	}
	if len(seen) != 2 || seen[0] != 0 || seen[1] != 1 {
		t.Fatalf("pages %+v", seen)
	}
}

func TestFetchMapsComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/WB-7" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key": "WB-7",
			"fields": map[string]any{
				"summary": "Ship inbox",
				"status":  map[string]any{"name": "In Review"},
				"comment": map[string]any{
					"comments": []map[string]any{
						{
							"id":      "10501",
							"body":    "Looks good",
							"author":  map[string]string{"displayName": "Ada Lovelace"},
							"created": "2020-03-15T09:30:00.000+0000",
						},
						{"id": "10502", "body": "   "},
					},
				},
			},
		})
	}))
	t.Cleanup(server.Close)

	g := New()
	g.Client = server.Client()
	item, err := g.Fetch(t.Context(), domain.Source{BaseURL: server.URL}, "pat-1", "WB-7")
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Comments) != 1 {
		t.Fatalf("comments %+v", item.Comments)
	}
	got := item.Comments[0]
	if got.ExternalID != "10501" || got.Author != "Ada Lovelace" || got.Body != "Looks good" {
		t.Fatalf("comment %+v", got)
	}
	if got.URL != server.URL+"/browse/WB-7?focusedCommentId=10501" {
		t.Fatalf("url %s", got.URL)
	}
	if got.CreatedAt == nil || !got.CreatedAt.Equal(time.Date(2020, 3, 15, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("created %+v", got.CreatedAt)
	}
}

func TestProbeOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/myself" || r.Method != http.MethodGet {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accountId":"1"}`))
	}))
	t.Cleanup(server.Close)
	g := New()
	g.Client = server.Client()
	if err := g.Probe(t.Context(), domain.Source{BaseURL: server.URL}, "pat-1"); err != nil {
		t.Fatal(err)
	}
}

func TestProbeUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	t.Cleanup(server.Close)
	g := New()
	g.Client = server.Client()
	err := g.Probe(t.Context(), domain.Source{BaseURL: server.URL}, "bad")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(err.Error(), "nope") {
		t.Fatalf("leaked body %v", err)
	}
}

func TestProbeIncludesErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["You do not have permission"]}`))
	}))
	t.Cleanup(server.Close)
	g := New()
	g.Client = server.Client()
	err := g.Probe(t.Context(), domain.Source{BaseURL: server.URL}, "pat")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(err.Error(), "You do not have permission") {
		t.Fatalf("err %v", err)
	}
}

func TestCloudProbeRequiresEmail(t *testing.T) {
	err := New().Probe(t.Context(), domain.Source{BaseURL: "https://ex.atlassian.net"}, "pat")
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err %v", err)
	}
}

func TestCloudAuthUsesBasic(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://ex.atlassian.net/rest/api/3/myself", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyAuth(req, domain.Source{BaseURL: "https://ex.atlassian.net", Email: "dev@ex.com"}, "pat-1"); err != nil {
		t.Fatal(err)
	}
	user, pass, ok := req.BasicAuth()
	if !ok || user != "dev@ex.com" || pass != "pat-1" {
		t.Fatalf("basic %s %s %v", user, pass, ok)
	}
}

func TestProbeRejectsEmptyBase(t *testing.T) {
	err := New().Probe(t.Context(), domain.Source{}, "pat")
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err %v", err)
	}
}

func TestProbeSkipsTLSWhenFlagged(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/myself" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	g := New()
	src := domain.Source{BaseURL: server.URL}
	if err := g.Probe(t.Context(), src, "pat"); err == nil {
		t.Fatal("want tls error")
	}
	src.InsecureTLS = true
	if err := g.Probe(t.Context(), src, "pat"); err != nil {
		t.Fatal(err)
	}
}

func TestPullRejectsHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>login</html>"))
	}))
	t.Cleanup(server.Close)
	g := New()
	g.Client = server.Client()
	_, err := g.Pull(t.Context(), domain.Source{BaseURL: server.URL}, "pat")
	if err == nil || !strings.Contains(err.Error(), "HTML") {
		t.Fatalf("err %v", err)
	}
}
