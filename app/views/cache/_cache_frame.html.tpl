<section id="cache" class="tool-view is-active" data-view="cache" data-cache-panel>
  <div class="network-toolbar network-toolbar-container cache-toolbar">
    <div class="network-controls cache-summary-bar">
      <span>Size: <span data-cache-size>{{.cache.SizeText}}</span></span>
      <span>Usage: <span data-cache-usage>{{.cache.UsageText}}</span></span>
      <span>Keys: <span data-cache-key-count>{{.cache.KeyCount}}</span></span>
      <span>Hits: <span data-cache-hits>{{.cache.Snapshot.Stats.Hits}}</span></span>
      <span>Misses: <span data-cache-misses>{{.cache.Snapshot.Stats.Misses}}</span></span>
      <span>Sets: <span data-cache-sets>{{.cache.Snapshot.Stats.Sets}}</span></span>
      <span>Evictions: <span data-cache-evictions>{{.cache.Snapshot.Stats.Evictions}}</span></span>
      <span>Cache/Hit: <span data-cache-hit-rate>{{.cache.HitRateText}}</span></span>
      <span class="toolbar-spacer"></span>
      <form method="post" action="{{if .cache.Enabled}}/_golazy/cache/off{{else}}/_golazy/cache/on{{end}}">
        <input type="hidden" name="redirect" value="{{.cache.CurrentURL}}">
        <button type="submit" class="toolbar-button">{{if .cache.Enabled}}Disable cache{{else}}Enable cache{{end}}</button>
      </form>
    </div>
    <div class="filter-row">
      <form method="get" action="{{.cache.SearchURL}}" class="inline-form">
        {{if .cache.HasSelected}}
          <input type="hidden" name="key" value="{{.cache.SelectedKey}}">
        {{end}}
        <input class="filter-input network-filter" type="search" name="q" placeholder="Search cache keys" value="{{.cache.Query}}">
      </form>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count" data-cache-status>{{.cache.StatusText}}</span>
    </div>
  </div>

  <div class="split-view cache-split-view" data-controller="panel-resize" data-panel-resize-direction-value="right" data-panel-resize-min-value="260px" data-panel-resize-max-value="100%" data-panel-resize-size-value="{{.cache.PanelSizeValue}}">
    <section class="cache-table-pane" aria-label="Cache keys" data-panel-resize-target="primary">
      <table class="data-grid cache-key-grid" data-controller="table-resize">
        <thead>
          <tr>
            <th data-table-resize-min-width-value="180">Key</th>
            <th data-table-resize-min-width-value="64">Age</th>
            <th data-table-resize-min-width-value="64">Size</th>
            <th data-table-resize-min-width-value="48">Hits</th>
            <th data-table-resize-min-width-value="48">Sets</th>
          </tr>
        </thead>
        <tbody data-cache-list>
          {{if .cache.Defer}}
          {{else}}
            {{partial "cache_rows" .}}
          {{end}}
        </tbody>
      </table>
    </section>

    <div class="split-resize-handle" data-panel-resize-target="handle" data-action="pointerdown->panel-resize#start keydown->panel-resize#nudge" aria-label="Resize cache details pane"></div>

    <aside class="details-pane cache-detail-pane" aria-label="Cache entry detail">
      {{if .cache.HasSelected}}
        <div class="detail-header">
          <h2>{{.cache.SelectedKey}}</h2>
          <span>{{.cache.SelectedSizeText}}</span>
        </div>
        {{if .cache.SelectedError}}
          <div class="empty-state">{{.cache.SelectedError}}</div>
        {{else}}
          <pre class="cache-entry-content">{{.cache.Selected.Content}}</pre>
        {{end}}
      {{else}}
        <div class="empty-state">Select a cache key to inspect its content.</div>
      {{end}}
    </aside>
  </div>
</section>
