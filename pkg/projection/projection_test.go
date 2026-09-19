package projection

import (
	"encoding/json"
	"testing"
)

type head struct {
	Ref    string `json:"ref"`
	Sha    string `json:"sha"`
	Repo   any    `json:"repo,omitempty"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author,omitempty"`
}

type cr struct {
	Number string `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Head   head   `json:"head"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func TestProject(t *testing.T) {
	v := cr{
		Number: "42", Title: "fix: things", State: "open",
		Head: head{Ref: "feature/x", Sha: "abc123"},
		Labels: []struct {
			Name string `json:"name"`
		}{{Name: "bug"}, {Name: "p1"}},
	}

	tests := []struct {
		name    string
		fields  []string
		want    string // JSON of the projection
		wantErr bool
	}{
		{
			name:   "top-level fields",
			fields: []string{"number", "state"},
			want:   `{"number":"42","state":"open"}`,
		},
		{
			name:   "nested path",
			fields: []string{"number", "head.ref"},
			want:   `{"head":{"ref":"feature/x"},"number":"42"}`,
		},
		{
			name:   "array traversed element-wise",
			fields: []string{"labels.name"},
			want:   `{"labels":[{"name":"bug"},{"name":"p1"}]}`,
		},
		{
			name:   "absent field omitted",
			fields: []string{"number", "merged_at"},
			want:   `{"number":"42"}`,
		},
		{
			name:   "wildcard returns full document",
			fields: []string{"*"},
			want:   `{"head":{"ref":"feature/x","sha":"abc123"},"labels":[{"name":"bug"},{"name":"p1"}],"number":"42","state":"open","title":"fix: things"}`,
		},
		{
			name:    "empty field list is an error",
			wantErr: true,
		},
		{
			name:    "invalid path is an error",
			fields:  []string{".head"},
			wantErr: true,
		},
		{
			name:    "segment-level wildcard is an error",
			fields:  []string{"labels.*"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Project(v, tt.fields...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Project() err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Project() err = %v", err)
			}
			raw, _ := json.Marshal(got)
			if string(raw) != tt.want {
				t.Fatalf("Project() = %s, want %s", raw, tt.want)
			}
		})
	}
}

func TestProjectPreservesNumbers(t *testing.T) {
	v := map[string]any{"id": 12345, "ratio": 0.5}
	got, err := Project(v, "id", "ratio")
	if err != nil {
		t.Fatalf("Project() err = %v", err)
	}
	if id := got["id"].(json.Number).String(); id != "12345" {
		t.Fatalf("id = %v, want json.Number 12345", id)
	}
}

func TestProjectList(t *testing.T) {
	items := []cr{{Number: "1", Title: "a"}, {Number: "2", Title: "b"}}
	got, err := ProjectList(items, "number")
	if err != nil {
		t.Fatalf("ProjectList() err = %v", err)
	}
	if len(got) != 2 || got[0]["number"] != "1" || got[1]["number"] != "2" {
		t.Fatalf("ProjectList() = %v", got)
	}
}
