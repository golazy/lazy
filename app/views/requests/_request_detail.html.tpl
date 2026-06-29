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
        <div class="request-tracing-layout" data-controller="panel-resize" data-panel-resize-direction-value="bottom" data-panel-resize-min-value="170px" data-panel-resize-max-value="72%">
          <div class="request-trace-status-row">
            <div class="request-trace-status">
              <strong>{{.requests.TraceStatusText}}</strong>
              {{if .requests.FrameworkStatusText}}
                <span>{{.requests.FrameworkStatusText}}</span>
              {{end}}
            </div>
            <form method="get" action="{{path_for "requests"}}" class="request-framework-toggle">
              <input type="hidden" name="request" value="{{.requests.SelectedRequestID}}">
              {{if .requests.HasSelectedSpan}}<input type="hidden" name="span" value="{{.requests.SelectedSpanID}}">{{end}}
              <input type="hidden" name="tab" value="tracing">
              <input type="hidden" name="type" value="{{.requests.TypeValue}}">
              <input type="hidden" name="sort" value="{{.requests.SortValue}}">
              {{if .requests.Query}}<input type="hidden" name="q" value="{{.requests.Query}}">{{end}}
              <label><input type="checkbox" name="framework" value="1" {{if .requests.Framework}}checked{{end}} onchange="this.form.requestSubmit()"> Include golazy</label>
            </form>
          </div>

          <section class="runtime-pane request-trace-table-pane" data-panel-resize-target="primary">
            <table class="data-grid request-region-grid" data-controller="table-resize">
              <thead>
                <tr>
                  <th rowspan="2"></th>
                  <th rowspan="2">Region</th>
                  <th colspan="2">Time</th>
                  <th colspan="2">Allocations</th>
                  <th colspan="2">Memory</th>
                </tr>
                <tr>
                  <th><a href="{{.requests.SortURL "time-total"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.SortSelected "time-total"}}true{{end}}">Total</a></th>
                  <th><a href="{{.requests.SortURL "time-self"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.SortSelected "time-self"}}true{{end}}">Self</a></th>
                  <th><a href="{{.requests.SortURL "alloc-total"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.SortSelected "alloc-total"}}true{{end}}">Total</a></th>
                  <th><a href="{{.requests.SortURL "alloc-self"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.SortSelected "alloc-self"}}true{{end}}">Self</a></th>
                  <th><a href="{{.requests.SortURL "memory-total"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.SortSelected "memory-total"}}true{{end}}">Total</a></th>
                  <th><a href="{{.requests.SortURL "memory-self"}}" data-turbo-frame="request_detail" aria-current="{{if .requests.SortSelected "memory-self"}}true{{end}}">Self</a></th>
                </tr>
              </thead>
              <tbody>
                {{range .requests.RegionRows}}
                  <tr aria-selected="{{.Selected}}" data-framework="{{.Span.Framework}}">
                    <td><span class="request-region-meter"><span style="width: {{.BarPercent}}"></span></span></td>
                    <td><a href="{{.URL}}" data-turbo-frame="request_detail" style="padding-left: {{.LabelPadding}}">{{.Span.Name}}</a></td>
                    <td>{{.Span.TotalTimeText}}</td>
                    <td>{{.Span.SelfDurationText}}</td>
                    <td>{{.Span.TotalAllocText}}</td>
                    <td>{{.Span.SelfAllocText}}</td>
                    <td>{{.Span.TotalMemoryText}}</td>
                    <td>{{.Span.SelfMemoryText}}</td>
                  </tr>
                {{else}}
                  <tr>
                    <td colspan="8" class="empty-cell">No trace regions recorded for this request.</td>
                  </tr>
                {{end}}
              </tbody>
            </table>
          </section>

          <div class="split-resize-handle" data-panel-resize-target="handle" data-action="pointerdown->panel-resize#start keydown->panel-resize#nudge" aria-label="Resize trace rows"></div>

          <section class="runtime-pane request-trace-flame-pane">
            <div class="request-flame-axis">
              <span>{{.requests.FlameAxisText}}</span>
            </div>
            <div class="trace-flamegraph request-flamegraph" data-trace-flamegraph>
              {{range .requests.FlameRows}}
                <a class="trace-flame-row" href="{{.URL}}" data-turbo-frame="request_detail" data-selected="{{.Selected}}" data-goroutine-change="{{.GoroutineChanged}}" style="margin-left: {{.FlameMargin}}">
                  <span class="trace-flame-bar" style="margin-left: {{.LeftPercent}}; width: {{.WidthPercent}}"></span>
                  <span class="trace-flame-label">{{.Span.FlameLabel}}</span>
                </a>
              {{else}}
                <div class="empty-state">No trace regions recorded for this request.</div>
              {{end}}
            </div>
          </section>
        </div>
      {{else if .requests.LogsTab}}
        <section class="runtime-pane request-logs-pane">
          <h2>Logs</h2>
          <table class="data-grid trace-log-grid" data-controller="table-resize">
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
