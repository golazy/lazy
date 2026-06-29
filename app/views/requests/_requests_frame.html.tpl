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
      <span class="toolbar-count">{{.requests.RequestCountText}}</span>
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
        <input type="hidden" name="framework" value="{{.requests.FrameworkValue}}">
        <input class="filter-input network-filter" type="search" name="q" placeholder="Filter" value="{{.requests.Query}}">
      </form>
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
          {{range .requests.Rows}}
            <tr aria-selected="{{.Selected}}">
              <td><span class="request-status-dot" data-status-class="{{.Trace.StatusClass}}"></span></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.PathText}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.MethodText}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.StatusText}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.DomainText}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.TypeText}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.InitiatorText}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.SizeText}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.DurationText}}</a></td>
            </tr>
          {{else}}
            <tr>
              <td colspan="9" class="empty-cell">{{if .requests.Error}}{{.requests.Error}}{{else}}No request details recorded yet.{{end}}</td>
            </tr>
          {{end}}
        </tbody>
      </table>
    </section>

    <div class="split-resize-handle" data-panel-resize-target="handle" data-action="pointerdown->panel-resize#start keydown->panel-resize#nudge" aria-label="Resize request details pane"></div>

    <aside class="details-pane">
      {{if .requests.HasSelected}}
        <div class="request-detail-pane">
          <div class="request-detail-header">
            <strong>{{.requests.Selected.PathText}}</strong>
            <span>{{.requests.Selected.MethodText}}</span>
            <span>{{.requests.Selected.StatusText}}</span>
            <span>{{.requests.Selected.DurationText}}</span>
          </div>
          <nav class="request-detail-tabs" aria-label="Request detail tabs">
            <a href="{{.requests.TabURL "headers"}}" data-turbo-frame="_top" aria-current="{{if .requests.HeadersTab}}page{{end}}">Headers</a>
            <a href="{{.requests.TabURL "tracing"}}" data-turbo-frame="_top" aria-current="{{if .requests.TracingTab}}page{{end}}">Tracing</a>
            <a href="{{.requests.TabURL "logs"}}" data-turbo-frame="_top" aria-current="{{if .requests.LogsTab}}page{{end}}">Logs</a>
          </nav>
          <div class="request-detail-body">
            {{if .requests.HeadersTab}}
              <section class="runtime-pane request-summary-pane">
                <h2>General</h2>
                <dl class="detail-list">
                  <dt>Path</dt>
                  <dd>{{.requests.Selected.PathText}}</dd>
                  <dt>Method</dt>
                  <dd>{{.requests.Selected.MethodText}}</dd>
                  <dt>Status</dt>
                  <dd>{{.requests.Selected.StatusText}}</dd>
                  <dt>Duration</dt>
                  <dd>{{.requests.Selected.DurationText}}</dd>
                  <dt>Allocated</dt>
                  <dd>{{.requests.Selected.MemoryText}}</dd>
                  <dt>Runtime</dt>
                  <dd>{{.requests.Selected.RuntimeText}}</dd>
                </dl>
              </section>
            {{else if .requests.TracingTab}}
              <div class="request-trace-grid">
                <section class="runtime-pane request-trace-timeline-pane">
                  <div class="section-heading">
                    <h2>Timeline</h2>
                    <span class="toolbar-count">{{.requests.SpanCountText}}</span>
                    <a class="toolbar-button" href="{{.requests.FrameworkToggleURL}}" data-turbo-frame="_top">{{.requests.FrameworkToggleText}}</a>
                  </div>
                  <div class="trace-timeline" data-trace-timeline>
                    {{range .requests.TimelineRows}}
                      <div class="trace-timeline-row" data-framework="{{.Span.Framework}}" data-selected="{{.Selected}}">
                        <a class="trace-timeline-label" href="{{.URL}}" data-turbo-frame="_top" style="padding-left: {{.LabelPadding}}">{{.Span.Name}}</a>
                        <a class="trace-timeline-track" href="{{.URL}}" data-turbo-frame="_top">
                          <span class="trace-timeline-bar{{if eq .Span.Name "http.server.request"}} trace-task-bar{{end}}" style="left: {{.LeftPercent}}; width: {{.WidthPercent}}"></span>
                        </a>
                        <span class="trace-timeline-time">{{.Span.DurationText}}</span>
                      </div>
                    {{else}}
                      <div class="empty-state">No trace regions recorded for this request.</div>
                    {{end}}
                  </div>
                </section>

                <section class="runtime-pane request-trace-region-pane">
                  <h2>Selected Region</h2>
                  <dl class="detail-list">
                    <dt>Name</dt>
                    <dd>{{.requests.SelectedSpan.Name}}</dd>
                    <dt>Duration</dt>
                    <dd>{{.requests.SelectedSpan.DurationSummaryText}}</dd>
                    <dt>Self time</dt>
                    <dd>{{.requests.SelectedSpan.SelfDurationText}}</dd>
                    <dt>Allocated</dt>
                    <dd>{{.requests.SelectedSpan.AllocationSummaryText}}</dd>
                    <dt>Mallocs</dt>
                    <dd>{{.requests.SelectedSpan.MallocsSummaryText}}</dd>
                    <dt>Frees</dt>
                    <dd>{{.requests.SelectedSpan.FreesSummaryText}}</dd>
                    <dt>Trace file</dt>
                    <dd><code>{{.requests.Selected.TraceFile}}</code></dd>
                  </dl>
                </section>

                <section class="runtime-pane request-trace-flame-pane">
                  <h2>Chronological Flamegraph</h2>
                  <div class="trace-flamegraph" data-trace-flamegraph>
                    {{range .requests.FlameRows}}
                      <a class="trace-flame-row" href="{{.URL}}" data-turbo-frame="_top" data-selected="{{.Selected}}" style="margin-left: {{.FlameMargin}}">
                        <span class="trace-flame-bar" style="margin-left: {{.LeftPercent}}; width: {{.WidthPercent}}"></span>
                        <span class="trace-flame-label">{{.Span.FlameLabel}}</span>
                      </a>
                    {{else}}
                      <div class="empty-state">Select a timeline region.</div>
                    {{end}}
                  </div>
                </section>
              </div>
            {{else if .requests.LogsTab}}
              <section class="runtime-pane request-logs-pane">
                <h2>Logs</h2>
                <table class="data-grid trace-log-grid">
                  <thead>
                    <tr>
                      <th>Time</th>
                      <th>Level</th>
                      <th>Message</th>
                      <th>Span</th>
                    </tr>
                  </thead>
                  <tbody data-request-logs>
                    {{range .requests.Logs}}
                      <tr>
                        <td>{{.TimeText}}</td>
                        <td>{{.Level}}</td>
                        <td>{{.Message}}</td>
                        <td>{{.SpanID}}</td>
                      </tr>
                    {{else}}
                      <tr>
                        <td colspan="4" class="empty-cell">No logs for this request.</td>
                      </tr>
                    {{end}}
                  </tbody>
                </table>
              </section>
            {{end}}
          </div>
        </div>
      {{else}}
        <div class="empty-state">Select a request to inspect headers, logs, and traces.</div>
      {{end}}
    </aside>
  </div>
</section>
