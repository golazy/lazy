<span hidden data-assets-frame-count>{{.assets_count_text}}</span>
<table class="data-grid assets-grid" data-controller="table-resize">
  <thead>
    <tr>
      <th>Public Path</th>
      <th>Permanent Path</th>
      <th>Type</th>
      <th>Size</th>
      <th>Source</th>
      <th>Kind</th>
      <th>Status</th>
    </tr>
  </thead>
  <tbody data-assets-list>
    {{if .defer_panel_lists}}
    {{else}}
      {{partial "asset_rows" .}}
    {{end}}
  </tbody>
</table>
