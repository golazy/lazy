<main class="devtools-panel" data-panel data-state="{{.state.State}}">
  <header class="top-toolbar tabbed-pane-header">
    <nav class="panel-tabs" aria-label="GoLazy panel sections">
      <button type="button" data-tab="requests" aria-selected="false">Requests</button>
      <button type="button" data-tab="console" aria-selected="false">Console</button>
      <button type="button" data-tab="logs" aria-selected="true">App Logs</button>
      <button type="button" data-tab="traces" aria-selected="false">Traces</button>
      <button type="button" data-tab="routes" aria-selected="false">Routes</button>
      <button type="button" data-tab="jobs" aria-selected="false">Jobs</button>
      <button type="button" data-tab="assets" aria-selected="false">Assets</button>
      <button type="button" data-tab="actions" aria-selected="false">Actions</button>
    </nav>
    <button type="button" class="panel-close-button" data-panel-close hidden aria-label="Close GoLazy development panel" title="Close GoLazy development panel"></button>
  </header>

  <section class="tool-view" data-view="requests">
    <div class="network-toolbar network-toolbar-container">
      <div class="network-controls">
        <button type="button" class="icon-button record-button" disabled title="Request capture is not wired yet">
          <span class="record-dot"></span>
        </button>
        <button type="button" class="icon-button clear-button" disabled title="Clear request log"></button>
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

    <div class="split-view" data-controller="panel-resize" data-panel-resize-axis-value="horizontal" data-panel-resize-min-value="240">
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
              <td colspan="9" class="empty-cell">Request capture will use browser network entries and GoLazy telemetry in a later slice.</td>
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

  <section class="tool-view" data-view="console">
    <div class="filter-toolbar">
      <button type="button" class="toolbar-button" disabled>Clear</button>
      <input class="filter-input" type="search" placeholder="Filter console" disabled>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count">Not connected</span>
    </div>
    <div class="empty-state">Browser console capture is not wired yet. It will use the development-only injected browser client.</div>
  </section>

  <section class="tool-view is-active" data-view="logs">
    <div class="filter-toolbar">
      <span class="state-chip" data-state-chip data-state-value="{{.state.State}}">
        <span class="state-dot"></span>
        <span data-panel-state>{{.state.State}}</span>
      </span>
      <span class="toolbar-divider"></span>
      <span data-panel-message>{{.state.Message}}</span>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count">Build <span data-panel-build>{{.state.BuildCount}}</span></span>
    </div>

    <div class="runtime-grid">
      <section class="runtime-pane">
        <h2>Status</h2>
        <dl class="detail-list">
          <dt>Message</dt>
          <dd data-panel-message>{{.state.Message}}</dd>
          <dt>Build</dt>
          <dd data-panel-build>{{.state.BuildCount}}</dd>
          <dt>Duration</dt>
          <dd data-panel-duration>{{.state.Duration}}</dd>
          <dt>Command</dt>
          <dd><code>{{.state.CommandPath}}</code></dd>
          <dt>Watched root</dt>
          <dd><code>{{.state.WatchedRoot}}</code></dd>
          <dt>App address</dt>
          <dd><code data-panel-app-addr>{{.state.AppAddr}}</code></dd>
          <dt>Control plane</dt>
          <dd><code data-panel-control-addr>{{.state.ControlPlaneAddr}}</code></dd>
        </dl>
      </section>

      <section class="runtime-pane">
        <h2>Changed Files</h2>
        <ul class="compact-list" data-panel-changes>
          {{range .state.Changed}}
            <li><code>{{.}}</code></li>
          {{else}}
            <li class="muted">No recent changes.</li>
          {{end}}
        </ul>
      </section>

      <section class="runtime-pane output-pane">
        <h2>Latest Output</h2>
        <pre class="panel-output" data-panel-output>{{.state.Output}}</pre>
      </section>

      <section class="runtime-pane events-pane">
        <h2>Events</h2>
        <ol class="event-list" data-panel-events>
          {{range .state.Events}}
            <li><span>{{.Time.Format "15:04:05"}}</span> <strong>{{.Type}}</strong> {{.Message}}</li>
          {{else}}
            <li class="muted">Waiting for development events.</li>
          {{end}}
        </ol>
      </section>
    </div>
  </section>

  <section class="tool-view" data-view="traces">
    <div class="filter-toolbar">
      <button type="button" class="toolbar-button" disabled>Capture trace</button>
      <input class="filter-input" type="search" placeholder="Filter traces" disabled>
    </div>
    <div class="empty-state">Trace capture will appear here after the telemetry/runtime trace API is implemented.</div>
  </section>

  <section class="tool-view" data-view="routes">
    <div class="filter-toolbar">
      <input class="filter-input" type="search" placeholder="Filter routes" disabled>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count">Not connected</span>
    </div>
    <div class="empty-state">Route inspection will use the lazydev route table endpoint in a later slice.</div>
  </section>

  <section class="tool-view" data-view="jobs">
    <div class="filter-toolbar">
      <input class="filter-input" type="search" placeholder="Filter jobs" disabled>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count" data-jobs-state>Jobs unavailable</span>
    </div>

    <div class="runtime-grid" data-jobs-panel>
      <section class="runtime-pane">
        <h2>State</h2>
        <dl class="detail-list">
          <dt>Runner</dt>
          <dd data-jobs-running>Unknown</dd>
          <dt>Total</dt>
          <dd data-jobs-total>0</dd>
          <dt>Pending</dt>
          <dd data-jobs-pending>0</dd>
          <dt>Running</dt>
          <dd data-jobs-count-running>0</dd>
          <dt>Retrying</dt>
          <dd data-jobs-retrying>0</dd>
          <dt>Succeeded</dt>
          <dd data-jobs-succeeded>0</dd>
          <dt>Discarded</dt>
          <dd data-jobs-discarded>0</dd>
        </dl>
      </section>

      <section class="runtime-pane output-pane">
        <h2>Definitions</h2>
        <ul class="compact-list" data-job-definitions>
          <li class="muted">No job definitions.</li>
        </ul>
      </section>

      <section class="runtime-pane output-pane">
        <h2>Recent Jobs</h2>
        <table class="data-grid">
          <thead>
            <tr>
              <th>ID</th>
              <th>Kind</th>
              <th>Queue</th>
              <th>State</th>
              <th>Attempt</th>
              <th>Run At</th>
              <th>Error</th>
            </tr>
          </thead>
          <tbody data-jobs-recent>
            <tr>
              <td colspan="7" class="empty-cell">No recent jobs.</td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>
  </section>

  <section class="tool-view" data-view="assets">
    <div class="filter-toolbar">
      <label class="filter-toggle">
        <input type="checkbox" checked disabled>
        <span>Generated</span>
      </label>
      <label class="filter-toggle">
        <input type="checkbox" checked disabled>
        <span>Public</span>
      </label>
      <input class="filter-input" type="search" placeholder="Filter assets" disabled>
    </div>
    <div class="empty-state">Asset inspection will show public and generated asset metadata once the panel API exists.</div>
  </section>

  <section class="tool-view" data-view="actions">
    <div class="filter-toolbar">
      <form method="post" action="/_golazy/rebuild">
        <button type="submit" class="toolbar-button">Rebuild</button>
      </form>
      <form method="post" action="/_golazy/restart">
        <button type="submit" class="toolbar-button">Restart</button>
      </form>
      <a class="toolbar-button" href="/">Open app</a>
      <span class="toolbar-spacer"></span>
      <span class="toolbar-count">Development actions</span>
    </div>

    <div class="action-layout">
      <section class="runtime-pane" data-cache-panel>
        <div class="section-heading">
          <h2>View Cache</h2>
          <div class="cache-actions">
            <button type="button" class="toolbar-button" data-cache-action="/_golazy/cache/on">On</button>
            <button type="button" class="toolbar-button" data-cache-action="/_golazy/cache/off">Off</button>
          </div>
        </div>
        <dl class="cache-stats">
          <dt>Status</dt>
          <dd data-cache-enabled>Unknown</dd>
          <dt>Entries</dt>
          <dd data-cache-entries>0</dd>
          <dt>Hits</dt>
          <dd data-cache-hits>0</dd>
          <dt>Misses</dt>
          <dd data-cache-misses>0</dd>
          <dt>Sets</dt>
          <dd data-cache-sets>0</dd>
          <dt>Evictions</dt>
          <dd data-cache-evictions>0</dd>
        </dl>
        <ul class="cache-keys compact-list" data-cache-keys>
          <li class="muted">No keys.</li>
        </ul>
      </section>

      <section class="runtime-pane">
        <h2>Action Notes</h2>
        <ul class="compact-list">
          <li>Rebuild recompiles the app and restarts the child process.</li>
          <li>Restart starts the latest successful build without recompiling.</li>
          <li>Cache controls proxy to the app lazydev control plane.</li>
        </ul>
      </section>
    </div>
  </section>

  <footer class="status-bar">
    <span>State: <strong data-panel-state>{{.state.State}}</strong></span>
    <span>Build <span data-panel-build>{{.state.BuildCount}}</span></span>
    <span data-panel-duration>{{.state.Duration}}</span>
    <span>App <code data-panel-app-addr>{{.state.AppAddr}}</code></span>
    <span>Control <code data-panel-control-addr>{{.state.ControlPlaneAddr}}</code></span>
    <span data-cache-state>Cache unknown</span>
  </footer>
</main>
