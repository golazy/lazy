<section id="requests" class="tool-view is-active" data-view="requests">
  <div class="network-toolbar network-toolbar-container">
    <div class="network-controls">
      <form method="post" action="{{if .monitoring.Enabled}}/_golazy/request-monitoring/off{{else}}/_golazy/request-monitoring/on{{end}}">
        <button type="submit" class="icon-button record-button" title="{{if .monitoring.Enabled}}Disable detailed request monitoring{{else}}Enable detailed request monitoring{{end}}" aria-pressed="{{.monitoring.Enabled}}">
          <span class="record-dot"></span>
        </button>
      </form>
      <button type="button" class="icon-button clear-button" disabled title="Clear request log"></button>
      <span class="toolbar-divider"></span>
      <span class="toolbar-count">{{.monitoring.StatusText}}</span>
      <span class="toolbar-divider"></span>
      <label class="inline-check">
        <input type="checkbox" checked disabled>
        <span>Preserve log</span>
      </label>
      <span class="toolbar-divider"></span>
      <label class="inline-check">
        <input type="checkbox" disabled>
        <span>Disable cache</span>
      </label>
      <button type="button" class="select-button" disabled>No throttling</button>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count">0 requests</span>
    </div>
    <div class="filter-row">
      <input class="filter-input network-filter" type="search" placeholder="Filter" disabled>
      <div class="scope-filter" aria-label="Request scope filters">
        <button type="button" aria-pressed="true" disabled>App</button>
        <button type="button" aria-pressed="false" disabled>Assets</button>
        <button type="button" aria-pressed="false" disabled>All</button>
      </div>
      <label class="inline-check">
        <input type="checkbox" disabled>
        <span>Invert</span>
      </label>
      <span class="toolbar-spacer"></span>
      <button type="button" class="more-filters" disabled>More filters</button>
    </div>
    <div class="type-filter" aria-label="Request type filters">
      <button type="button" aria-pressed="true" disabled>All</button>
      <button type="button" disabled>Fetch/XHR</button>
      <button type="button" disabled>Doc</button>
      <button type="button" disabled>CSS</button>
      <button type="button" disabled>JS</button>
      <button type="button" disabled>Font</button>
      <button type="button" disabled>Img</button>
      <button type="button" disabled>Media</button>
      <button type="button" disabled>Manifest</button>
      <button type="button" disabled>Socket</button>
      <button type="button" disabled>Wasm</button>
      <button type="button" disabled>Other</button>
    </div>
  </div>

  <div class="split-view" data-controller="panel-resize" data-panel-resize-direction-value="right" data-panel-resize-min-value="240px" data-panel-resize-max-value="70%">
    <section class="request-table-pane" aria-label="Request log" data-panel-resize-target="primary">
      <table class="data-grid network-log-grid">
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
        <tbody>
          <tr>
            <td colspan="9" class="empty-cell">No request details recorded yet.</td>
          </tr>
        </tbody>
      </table>
    </section>

    <div class="split-resize-handle" data-panel-resize-target="handle" data-action="pointerdown->panel-resize#start keydown->panel-resize#nudge" aria-label="Resize request details pane"></div>

    <aside class="details-pane">
      <div class="empty-state">Select a request to inspect headers, preview, response, logs, cache, and traces.</div>
    </aside>
  </div>
</section>
