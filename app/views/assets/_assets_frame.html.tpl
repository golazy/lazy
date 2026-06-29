<section id="assets" class="tool-view is-active" data-view="assets">
  <div class="filter-toolbar">
    <form method="get" action="{{path_for "assets"}}" class="inline-form">
      <input class="filter-input" type="search" name="q" placeholder="Filter assets" value="{{.assets_query}}">
    </form>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-assets-count>{{if .assets_error}}Assets unavailable{{else}}{{.assets_visible}} / {{.assets_total}} assets{{end}}</span>
  </div>
  <table class="data-grid assets-grid">
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
</section>
