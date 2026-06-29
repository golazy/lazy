<section id="traces" class="tool-view is-active" data-view="traces">
  <div class="filter-toolbar">
    <form method="post" action="{{if .monitoring.Enabled}}/_golazy/request-monitoring/off{{else}}/_golazy/request-monitoring/on{{end}}">
      <button type="submit" class="toolbar-button">{{if .monitoring.Enabled}}Disable monitoring{{else}}Enable monitoring{{end}}</button>
    </form>
    <a class="toolbar-button" href="{{.traces.FrameworkToggleURL}}" data-turbo-frame="_top">{{.traces.FrameworkToggleText}}</a>
    <form method="get" action="{{path_for "traces"}}" class="inline-form">
      <input type="hidden" name="framework" value="{{.traces.FrameworkValue}}">
      {{if .traces.HasSelected}}
        <input type="hidden" name="trace" value="{{.traces.SelectedTraceID}}">
      {{end}}
      {{if .traces.HasSelectedSpan}}
        <input type="hidden" name="span" value="{{.traces.SelectedSpanID}}">
      {{end}}
      <input class="filter-input" type="search" name="q" placeholder="Filter traces" value="{{.traces.Query}}">
    </form>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count">{{.monitoring.StatusText}}</span>
    <code>{{.monitoring.Directory}}</code>
  </div>

  <div class="traces-layout" data-traces-panel>
    <section class="trace-list-pane" aria-label="Recorded request traces">
      <table class="data-grid trace-list-grid">
        <thead>
          <tr>
            <th>Method</th>
            <th>Path</th>
            <th>Status</th>
            <th>Time</th>
          </tr>
        </thead>
        <tbody data-trace-list>
          {{range .traces.TraceRows}}
            <tr aria-selected="{{.Selected}}">
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.Method}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.Path}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.Status}}</a></td>
              <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.DurationText}}</a></td>
            </tr>
          {{else}}
            <tr>
              <td colspan="4" class="empty-cell">{{if .traces.Error}}{{.traces.Error}}{{else}}No traces recorded.{{end}}</td>
            </tr>
          {{end}}
        </tbody>
      </table>
    </section>

    <section class="trace-detail-pane" aria-label="Trace details">
      <div class="trace-summary-strip">
        <strong>{{if .traces.HasSelected}}{{.traces.Selected.Title}}{{else}}Select a trace{{end}}</strong>
        {{if .traces.HasSelected}}
          <span>{{.traces.Selected.RuntimeText}}</span>
          <span>{{.traces.Selected.MemoryText}}</span>
          <code>{{.traces.Selected.TraceFile}}</code>
        {{end}}
      </div>

      <div class="trace-detail-grid">
        <section class="runtime-pane trace-timeline-pane">
          <div class="section-heading">
            <h2>Timeline</h2>
            <span class="toolbar-count">{{.traces.SpanCountText}}</span>
          </div>
          <div class="trace-timeline" data-trace-timeline>
            {{range .traces.TimelineRows}}
              <div class="trace-timeline-row" data-framework="{{.Span.Framework}}" data-selected="{{.Selected}}">
                <a class="trace-timeline-label" href="{{.URL}}" data-turbo-frame="_top" style="padding-left: {{.LabelPadding}}">{{.Span.Name}}</a>
                <a class="trace-timeline-track" href="{{.URL}}" data-turbo-frame="_top">
                  <span class="trace-timeline-bar{{if eq .Span.Name "http.server.request"}} trace-task-bar{{end}}" style="left: {{.LeftPercent}}; width: {{.WidthPercent}}"></span>
                </a>
                <span class="trace-timeline-time">{{.Span.DurationText}}</span>
              </div>
            {{else}}
              <div class="empty-state">Select a recorded request.</div>
            {{end}}
          </div>
        </section>

        <section class="runtime-pane trace-region-pane">
          <h2>Selected Region</h2>
          <dl class="detail-list">
            <dt>Name</dt>
            <dd>{{.traces.SelectedSpan.Name}}</dd>
            <dt>Duration</dt>
            <dd>{{.traces.SelectedSpan.DurationSummaryText}}</dd>
            <dt>Self time</dt>
            <dd>{{.traces.SelectedSpan.SelfDurationText}}</dd>
            <dt>Allocated</dt>
            <dd>{{.traces.SelectedSpan.AllocationSummaryText}}</dd>
            <dt>Mallocs</dt>
            <dd>{{.traces.SelectedSpan.MallocsSummaryText}}</dd>
            <dt>Frees</dt>
            <dd>{{.traces.SelectedSpan.FreesSummaryText}}</dd>
            <dt>Trace drill-down</dt>
            <dd><code>{{if .traces.HasSelected}}go tool trace {{.traces.Selected.TraceFile}}{{end}}</code></dd>
          </dl>
        </section>

        <section class="runtime-pane trace-flame-pane">
          <h2>Chronological Flamegraph</h2>
          <div class="trace-flamegraph" data-trace-flamegraph>
            {{range .traces.FlameRows}}
              <a class="trace-flame-row" href="{{.URL}}" data-turbo-frame="_top" data-selected="{{.Selected}}" style="margin-left: {{.FlameMargin}}">
                <span class="trace-flame-bar" style="margin-left: {{.LeftPercent}}; width: {{.WidthPercent}}"></span>
                <span class="trace-flame-label">{{.Span.FlameLabel}}</span>
              </a>
            {{else}}
              <div class="empty-state">Select a timeline section.</div>
            {{end}}
          </div>
        </section>

        <section class="runtime-pane trace-log-pane">
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
            <tbody data-trace-logs>
              {{range .traces.Logs}}
                <tr>
                  <td>{{.TimeText}}</td>
                  <td>{{.Level}}</td>
                  <td>{{.Message}}</td>
                  <td>{{.SpanID}}</td>
                </tr>
              {{else}}
                <tr>
                  <td colspan="4" class="empty-cell">No logs for this trace.</td>
                </tr>
              {{end}}
            </tbody>
          </table>
        </section>
      </div>
    </section>
  </div>
</section>
