package todoist

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pchkauu/want-brief/internal/domain"
)

func TestProbeOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v2/projects" || r.Method != http.MethodGet {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("auth %s", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	t.Cleanup(server.Close)
	g := New()
	g.Client = server.Client()
	if err := g.Probe(t.Context(), domain.Source{BaseURL: server.URL}, "tok"); err != nil {
		t.Fatal(err)
	}
}

func TestPullMapsTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v2/tasks" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("auth %s", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "42", "content": "Call Alex", "is_completed": false, "priority": 4},
		})
	}))
	t.Cleanup(server.Close)

	g := New()
	g.Client = server.Client()
	items, err := g.Pull(t.Context(), domain.Source{BaseURL: server.URL}, "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ExternalKey != "42" || !items[0].HintUrgent {
		t.Fatalf("%+v", items)
	}
}
