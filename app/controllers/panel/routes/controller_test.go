package routes

import (
	"testing"

	"golazy.dev/lazyroutes"
)

func TestRouteRowsSortsParams(t *testing.T) {
	rows := routeRows(lazyroutes.RouteTable{
		{
			Method: "GET",
			Path:   "/teams/{team_id}/posts/{post_id}",
			NamedParams: map[string]bool{
				"post_id": true,
				"team_id": true,
			},
		},
	})
	if got, want := rows[0].Params, "post_id, team_id"; got != want {
		t.Fatalf("Params = %q, want %q", got, want)
	}
}

func TestFilterRoutesMatchesRenderedFields(t *testing.T) {
	rows := []routeRow{
		{Method: "GET", Path: "/posts", Name: "posts", Target: "posts#Index"},
		{Method: "POST", Path: "/admin/posts", Name: "admin_posts", Target: "admin/posts#Create", Params: "post_id"},
	}
	filtered := filterRoutes(rows, "post_id")
	if len(filtered) != 1 || filtered[0].Name != "admin_posts" {
		t.Fatalf("filtered = %#v, want admin_posts", filtered)
	}
}
