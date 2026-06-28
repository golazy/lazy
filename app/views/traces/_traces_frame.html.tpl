<section id="traces" class="tool-view is-active" data-view="traces">
  <div class="filter-toolbar">
    <button type="button" class="toolbar-button" data-request-monitoring-action="/_golazy/request-monitoring/on" data-request-monitoring-button>Enable monitoring</button>
    <label class="inline-check">
      <input type="checkbox" data-trace-framework-toggle>
      <span>Framework</span>
    </label>
    <input class="filter-input" type="search" placeholder="Filter traces" data-trace-filter>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-request-monitoring-state>Monitoring unknown</span>
    <code data-request-monitoring-directory>.tmp/traces</code>
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
          <tr>
            <td colspan="4" class="empty-cell">No traces recorded.</td>
          </tr>
        </tbody>
      </table>
    </section>

    <section class="trace-detail-pane" aria-label="Trace details">
      <div class="trace-summary-strip">
        <strong data-trace-title>Select a trace</strong>
        <span data-trace-runtime></span>
        <span data-trace-memory></span>
        <code data-trace-file></code>
      </div>

      <div class="trace-detail-grid">
        <section class="runtime-pane trace-timeline-pane">
          <div class="section-heading">
            <h2>Timeline</h2>
            <span class="toolbar-count" data-trace-span-count>0 regions</span>
          </div>
          <div class="trace-timeline" data-trace-timeline>
            <div class="empty-state">Select a recorded request.</div>
          </div>
        </section>

        <section class="runtime-pane trace-region-pane">
          <h2>Selected Region</h2>
          <dl class="detail-list">
            <dt>Name</dt>
            <dd data-trace-region-name></dd>
            <dt>Duration</dt>
            <dd data-trace-region-duration></dd>
            <dt>Allocations</dt>
            <dd data-trace-region-allocations>Not captured per region</dd>
            <dt>Trace drill-down</dt>
            <dd><code data-trace-command></code></dd>
          </dl>
        </section>

        <section class="runtime-pane trace-flame-pane">
          <h2>Chronological Flamegraph</h2>
          <div class="trace-flamegraph" data-trace-flamegraph>
            <div class="empty-state">Select a timeline section.</div>
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
              <tr>
                <td colspan="4" class="empty-cell">No logs for this trace.</td>
              </tr>
            </tbody>
          </table>
        </section>
      </div>
    </section>
  </div>
</section>
