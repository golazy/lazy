<section id="requests" class="tool-view is-active" data-view="requests">
  <div class="network-toolbar network-toolbar-container">
    <div class="network-controls">
      <form method="post" action="{{if .monitoring.Enabled}}/_golazy/request-monitoring/off{{else}}/_golazy/request-monitoring/on{{end}}">
        <input type="hidden" name="redirect" value="{{.requests.CurrentURL}}">
        <button type="submit" class="icon-button record-button" title="Enable tracing" aria-pressed="{{.monitoring.Enabled}}">
          <span class="record-dot"></span>
        </button>
      </form>
      <form method="post" action="/_golazy/request-traces/clear">
        <input type="hidden" name="redirect" value="{{.requests.CurrentURL}}">
        <button type="submit" class="icon-button clear-button" title="Clear request log"></button>
      </form>
      <span class="toolbar-divider"></span>
      <form method="post" action="{{if .cache.Enabled}}/_golazy/cache/off{{else}}/_golazy/cache/on{{end}}">
        <input type="hidden" name="redirect" value="{{.requests.CurrentURL}}">
        <button type="submit" class="toolbar-button">{{if .cache.Enabled}}Disable cache{{else}}Enable cache{{end}}</button>
      </form>
      <button type="button" class="select-button" disabled>No throttling</button>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count" data-request-count>{{.requests.RequestCountText}}</span>
    </div>
    <div class="filter-row">
      <form method="get" action="{{path_for "requests"}}" class="inline-form">
        {{if .requests.HasSelected}}
          <input type="hidden" name="request" value="{{.requests.SelectedRequestID}}">
        {{end}}
        {{if .requests.HasSelectedSpan}}
          <input type="hidden" name="span" value="{{.requests.SelectedSpanID}}">
        {{end}}
        {{if .requests.TracingTab}}
          <input type="hidden" name="tab" value="tracing">
        {{end}}
        {{if .requests.LogsTab}}
          <input type="hidden" name="tab" value="logs">
        {{end}}
        <input type="hidden" name="type" value="{{.requests.TypeValue}}">
        <input type="hidden" name="framework" value="{{.requests.FrameworkValue}}">
        <input class="filter-input network-filter" type="search" name="q" placeholder="Filter" value="{{.requests.Query}}">
      </form>
      <span class="toolbar-spacer"></span>
      <button type="button" class="more-filters" disabled>More filters</button>
    </div>
    <div class="type-filter" aria-label="Request type filters">
      <a href="{{.requests.TypeURL "all"}}" data-turbo-frame="_top" aria-current="{{if .requests.TypeSelected "all"}}page{{end}}">All</a>
      <a href="{{.requests.TypeURL "framework"}}" data-turbo-frame="_top" aria-current="{{if .requests.TypeSelected "framework"}}page{{end}}">Framework</a>
      <a href="{{.requests.TypeURL "assets"}}" data-turbo-frame="_top" aria-current="{{if .requests.TypeSelected "assets"}}page{{end}}">Assets</a>
      <a href="{{.requests.TypeURL "other"}}" data-turbo-frame="_top" aria-current="{{if .requests.TypeSelected "other"}}page{{end}}">Other</a>
    </div>
  </div>

  <div class="split-view" data-controller="panel-resize" data-panel-resize-direction-value="right" data-panel-resize-min-value="240px" data-panel-resize-max-value="70%">
    <section class="request-table-pane" aria-label="Request log" data-panel-resize-target="primary">
      <table class="data-grid network-log-grid" data-controller="table-resize">
        <thead>
          <tr>
            <th></th>
            <th>Name</th>
            <th>Method</th>
            <th>Status</th>
            <th>Domain</th>
            <th>Type</th>
            <th>Initiator</th>
            <th>Size</th>
            <th>Time</th>
          </tr>
        </thead>
        <tbody data-request-list>
          {{if .defer_panel_lists}}
          {{else}}
            {{partial "request_rows" .}}
          {{end}}
        </tbody>
      </table>
    </section>

    <div class="split-resize-handle" data-panel-resize-target="handle" data-action="pointerdown->panel-resize#start keydown->panel-resize#nudge" aria-label="Resize request details pane"></div>

    <aside class="details-pane">
      {{if .requests.HasSelected}}
        <turbo-frame id="request_detail" src="{{.requests.DetailURL}}" loading="lazy">
          <div class="empty-state">Loading request details.</div>
        </turbo-frame>
      {{else}}
        <turbo-frame id="request_detail">
          <div class="empty-state">Select a request to inspect headers, logs, and traces.</div>
        </turbo-frame>
      {{end}}
    </aside>
  </div>
</section>
