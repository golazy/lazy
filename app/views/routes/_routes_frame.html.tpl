<section id="routes" class="tool-view is-active" data-view="routes">
  <div class="filter-toolbar">
    <form method="get" action="{{path_for "routes"}}">
      <input class="filter-input" type="search" name="q" value="{{.routes_query}}" placeholder="Filter routes">
    </form>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-routes-count>
      {{if .routes_error}}
        Routes unavailable
      {{else}}
        {{if .routes_query}}{{.routes_visible}} / {{end}}{{.routes_total}} routes
      {{end}}
    </span>
  </div>

  {{if .routes_error}}
    <div class="empty-state">Route table unavailable: {{.routes_error}}</div>
  {{else}}
    <div class="details-pane">
      <table class="data-grid routes-grid">
        <thead>
          <tr>
            <th>Method</th>
            <th>Path</th>
            <th>Name</th>
            <th>Target</th>
            <th>Params</th>
            <th>Namespace</th>
          </tr>
        </thead>
        <tbody data-routes-list>
          {{if .defer_panel_lists}}
          {{else}}
            {{partial "route_rows" .}}
          {{end}}
        </tbody>
      </table>
    </div>
  {{end}}
</section>
