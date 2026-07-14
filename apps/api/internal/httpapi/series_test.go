package httpapi

import (
	"net/http"
	"testing"
)

func TestParsePostSeriesFields(t *testing.T) {
	tests := []struct {
		name       string
		seriesID   rawJSON
		order      rawJSON
		wantChange bool
		wantNil    bool
		wantErr    bool
	}{
		{name: "omitted", wantNil: true},
		{name: "set", seriesID: rawJSON(`"10000000-0000-0000-0000-000000000001"`), order: rawJSON(`2`), wantChange: true},
		{name: "clear with nulls", seriesID: rawJSON(`null`), order: rawJSON(`null`), wantChange: true, wantNil: true},
		{name: "clear with empty id", seriesID: rawJSON(`""`), order: rawJSON(`null`), wantChange: true, wantNil: true},
		{name: "missing order", seriesID: rawJSON(`"10000000-0000-0000-0000-000000000001"`), wantErr: true},
		{name: "empty id with positive order", seriesID: rawJSON(`""`), order: rawJSON(`1`), wantErr: true},
		{name: "nonpositive order", seriesID: rawJSON(`"10000000-0000-0000-0000-000000000001"`), order: rawJSON(`0`), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed, seriesID, order, err := parsePostSeriesFields(test.seriesID, test.order)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
			if changed != test.wantChange {
				t.Fatalf("changed = %v, want %v", changed, test.wantChange)
			}
			if test.wantNil && (seriesID != nil || order != nil) {
				t.Fatalf("series fields = %v/%v, want nil/nil", seriesID, order)
			}
		})
	}
}

func TestSeriesRBACMapping(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/v1/admin/series", "taxonomy:read"},
		{http.MethodGet, "/api/v1/admin/series/:id", "taxonomy:read"},
		{http.MethodPost, "/api/v1/admin/series", "taxonomy:manage"},
		{http.MethodPatch, "/api/v1/admin/series/:id", "taxonomy:manage"},
		{http.MethodDelete, "/api/v1/admin/series/:id", "taxonomy:manage"},
	} {
		if got := permissionForRoute(test.method, test.path); got != test.want {
			t.Fatalf("%s %s permission = %q, want %q", test.method, test.path, got, test.want)
		}
	}
}
