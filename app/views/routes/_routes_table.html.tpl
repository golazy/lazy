<span hidden data-routes-frame-count>{{.routes_count_text}}</span>
{{if .routes_error}}
  <div class="empty-state">Route table unavailable: {{.routes_error}}</div>
{{else}}
  <div class="details-pane">
    <table class="data-grid routes-grid" data-controller="table-resize">
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
