package httpapi

import (
	"net/http"
	"testing"
)

func TestDraftIfMatchHeaderValidation(t *testing.T) {
	tests := []struct {
		name string
		values []string
		expected string
		wantError bool
	}{
		{"legacy without header", nil, "", false},
		{"quoted bootstrap", []string{`"draft-next"`}, "draft-next", false},
		{"quoted nanosecond", []string{`"draft-20261118-155758.123456789"`}, "draft-20261118-155758.123456789", false},
		{"missing quotes", []string{"draft-next"}, "", true},
		{"wildcard", []string{"*"}, "", true},
		{"empty", []string{`""`}, "", true},
		{"comma list", []string{`"draft-old","draft-new"`}, "", true},
		{"embedded quote", []string{`"draft-old\"evil"`}, "", true},
		{"multiple header values", []string{`"draft-old"`, `"draft-new"`}, "", true},
		{"non-draft generation", []string{`"gen-active"`}, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodPut, "http://127.0.0.1/api/v1/settings/draft", nil)
			if err != nil { t.Fatal(err) }
			for _, value := range tc.values { r.Header.Add("If-Match", value) }
			got, err := draftIfMatch(r)
			if (err != nil) != tc.wantError || got != tc.expected {
				t.Fatalf("got generation %q error %v; expected %q error=%v", got, err, tc.expected, tc.wantError)
			}
		})
	}
}
