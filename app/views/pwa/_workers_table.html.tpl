<span hidden data-pwa-workers-frame-count>{{.workers_count_text}}</span>
<table class="data-grid assets-grid" data-controller="table-resize">
  <thead>
    <tr>
      <th>Name</th>
      <th>Kind</th>
      <th>Path</th>
      <th>Scope</th>
      <th>Type</th>
      <th>Source</th>
      <th>Owner</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody data-pwa-workers-list>
    {{if .defer_panel_lists}}
    {{else}}
      {{partial "worker_rows" .}}
    {{end}}
  </tbody>
</table>
