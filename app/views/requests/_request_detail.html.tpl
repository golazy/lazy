{{if .requests.HasSelected}}
  <div class="request-detail-pane">
    <div class="request-detail-header">
      <strong>{{.requests.Selected.PathText}}</strong>
      <span>{{.requests.Selected.MethodText}}</span>
      <span>{{.requests.Selected.StatusText}}</span>
      <span>{{.requests.Selected.DurationText}}</span>
    </div>
    <nav class="request-detail-tabs" aria-label="Request detail tabs">
      <a href="{{.requests.TabURL "headers"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.HeadersTab}}page{{end}}">Headers</a>
      <a href="{{.requests.TabURL "tracing"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.TracingTab}}page{{end}}">Tracing</a>
      <a href="{{.requests.TabURL "logs"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.LogsTab}}page{{end}}">Logs</a>
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
            <dt>Handled by</dt>
            <dd>{{.requests.Selected.DomainText}}</dd>
            <dt>Type</dt>
            <dd>{{.requests.Selected.TypeText}}</dd>
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
              <a class="toolbar-button" href="{{.requests.FrameworkToggleURL}}" data-turbo-frame="request_detail">{{.requests.FrameworkToggleText}}</a>
            </div>
            <div class="trace-timeline" data-trace-timeline>
              {{range .requests.TimelineRows}}
                <div class="trace-timeline-row" data-framework="{{.Span.Framework}}" data-selected="{{.Selected}}">
                  <a class="trace-timeline-label" href="{{.URL}}" data-turbo-frame="request_detail" style="padding-left: {{.LabelPadding}}">{{.Span.Name}}</a>
                  <a class="trace-timeline-track" href="{{.URL}}" data-turbo-frame="request_detail">
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
                <a class="trace-flame-row" href="{{.URL}}" data-turbo-frame="request_detail" data-selected="{{.Selected}}" style="margin-left: {{.FlameMargin}}">
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
