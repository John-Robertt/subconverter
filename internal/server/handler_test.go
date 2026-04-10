package server

import (
	"net/url"
	"testing"
)

func TestResolveFilename(t *testing.T) {
	tests := []struct {
		name    string
		query   url.Values
		format  string
		want    string
		wantErr string
	}{
		{
			name:   "default clash filename",
			query:  url.Values{},
			format: "clash",
			want:   "clash.yaml",
		},
		{
			name:   "basename gets default extension",
			query:  url.Values{"filename": {"my-profile"}},
			format: "surge",
			want:   "my-profile.conf",
		},
		{
			name:   "uppercase extension allowed",
			query:  url.Values{"filename": {"PROFILE.YAML"}},
			format: "clash",
			want:   "PROFILE.YAML",
		},
		{
			name:    "unicode rejected",
			query:   url.Values{"filename": {"配置"}},
			format:  "clash",
			wantErr: "invalid filename parameter: only ASCII letters, digits, dot, dash, and underscore are allowed",
		},
		{
			name:    "space rejected",
			query:   url.Values{"filename": {"my profile"}},
			format:  "clash",
			wantErr: "invalid filename parameter: only ASCII letters, digits, dot, dash, and underscore are allowed",
		},
		{
			name:    "quote rejected",
			query:   url.Values{"filename": {`a"b`}},
			format:  "clash",
			wantErr: "invalid filename parameter: only ASCII letters, digits, dot, dash, and underscore are allowed",
		},
		{
			name:    "basename required",
			query:   url.Values{"filename": {".yaml"}},
			format:  "clash",
			wantErr: "invalid filename parameter: basename is required",
		},
		{
			name:    "wrong extension rejected",
			query:   url.Values{"filename": {"profile.yaml"}},
			format:  "surge",
			wantErr: "invalid filename parameter: surge files must use .conf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveFilename(tc.query, tc.format)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveFilename() error = nil, want %q", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("resolveFilename() error = %q, want %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveFilename() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolveFilename() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestContentDispositionValue(t *testing.T) {
	got := contentDispositionValue("my-profile.yaml")
	want := `attachment; filename="my-profile.yaml"; filename*=UTF-8''my-profile.yaml`
	if got != want {
		t.Fatalf("contentDispositionValue() = %q, want %q", got, want)
	}
}
