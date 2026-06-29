package routes

import (
	"testing"

	"golazy.dev/lazyroutes"
)

func TestRouteRowsSortsParams(t *testing.T) {
	rows := routeRows(lazyroutes.RouteTable{
		{
			Method: "GET",
			Path:   "/posts",
		},
		{
			Method: "GET",
			Path:   "/teams/{team_id}/posts/{post_id}",
			NamedParams: map[string]bool{
				"post_id": true,
				"team_id": true,
			},
		},
	})
	if got, want := rows[1].Params, "post_id, team_id"; got != want {
		t.Fatalf("Params = %q, want %q", got, want)
	}
	if !rows[0].Linkable() || rows[0].Link != "/posts" {
		t.Fatalf("static GET row link = %#v, want /posts", rows[0])
	}
	if rows[1].Linkable() {
		t.Fatalf("parameterized GET row is linkable: %#v", rows[1])
	}
}

func TestRouteRowsOnlyLinkSafeGetPaths(t *testing.T) {
	rows := routeRows(lazyroutes.RouteTable{
		{Method: "POST", Path: "/posts"},
		{Method: "GET", Path: "/posts/{post_id}"},
		{Method: "GET", Path: "/about"},
	})
	if rows[0].Linkable() || rows[1].Linkable() {
		t.Fatalf("unsafe rows are linkable: %#v", rows)
	}
	if !rows[2].Linkable() || rows[2].Link != "/about" {
		t.Fatalf("GET static row = %#v, want /about link", rows[2])
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
