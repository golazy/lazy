<section id="dependencies" class="tool-view is-active dependencies-view" data-view="dependencies" data-dependencies-panel>
  <div class="filter-toolbar">
    <span>Dependency Graph</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{.dependencies.StatusText}}</span>
  </div>

  {{if .dependencies.Error}}
    <div class="empty-state">Dependency graph unavailable: {{.dependencies.Error}}</div>
  {{else}}
    <div class="dependencies-layout" data-controller="depgraph" data-depgraph-events-url-value="{{.dependency_shutdown_events}}">
      <section class="runtime-pane dependencies-summary-pane">
        <div class="section-heading">
          <h2>Graph</h2>
        </div>
        <form class="dependency-shutdown-form" action="{{.dependency_shutdown_action}}" method="post">
          <label>
            <span>Seconds at {{.dependency_shutdown_load_rps}} RPS</span>
            <input type="number" name="seconds" min="0" max="120" step="1" value="5" inputmode="numeric">
          </label>
          <button type="submit">Simulate shutdown</button>
        </form>
        <dl class="detail-list">
          <dt>Services</dt>
          <dd>{{.dependency_counts.Services}}</dd>
          <dt>Edges</dt>
          <dd>{{.dependency_counts.Edges}}</dd>
          <dt>Root</dt>
          <dd><code>app</code></dd>
          <dt>Ready</dt>
          <dd data-depgraph-target="ready">{{.dependency_shutdown.ReadyTextValue}}</dd>
          <dt>Active requests</dt>
          <dd data-depgraph-target="activeRequests">{{.dependency_shutdown.ActiveRequests}}</dd>
          <dt>Active connections</dt>
          <dd data-depgraph-target="activeConnections">{{.dependency_shutdown.ActiveConnections}}</dd>
          <dt>Phase</dt>
          <dd data-depgraph-target="phase">{{.dependency_shutdown.PhaseText}}</dd>
        </dl>
        <p class="dependency-shutdown-message" data-depgraph-target="message">{{.dependency_shutdown.MessageText}}</p>
      </section>

      <section class="runtime-pane dependencies-table-pane">
        <h2>Services</h2>
        <div class="depgraph-output" data-depgraph-target="output" hidden></div>
        <table class="data-grid dependencies-grid" data-controller="table-resize" data-depgraph-target="table">
          <thead>
            <tr>
              <th>Service</th>
              <th>Depends On</th>
              <th>Used By</th>
            </tr>
          </thead>
          <tbody>
            {{range .dependency_nodes}}
              <tr data-depgraph-service data-depgraph-name="{{.Name}}" data-depgraph-depends-on="{{.DependsOn}}" data-depgraph-used-by="{{.UsedBy}}" data-depgraph-state="{{.State}}" data-controller-depgraph-name="{{.Name}}" data-controller-depgraph-depends-on="{{.DependsOn}}" data-controller-depgraph-used-by="{{.UsedBy}}" data-controller-depgraph-state="{{.State}}">
                <td><code>{{.Name}}</code></td>
                <td><code>{{.DependsOn}}</code></td>
                <td><code>{{.UsedBy}}</code></td>
              </tr>
            {{else}}
              <tr>
                <td colspan="3" class="empty-cell">No application services reported.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      </section>

      <section class="runtime-pane dependencies-table-pane">
        <h2>Edges</h2>
        <table class="data-grid dependency-edge-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>From</th>
              <th>To</th>
            </tr>
          </thead>
          <tbody>
            {{range .dependency_edges}}
              <tr>
                <td><code>{{.From}}</code></td>
                <td><code>{{.To}}</code></td>
              </tr>
            {{else}}
              <tr>
                <td colspan="2" class="empty-cell">No dependency edges reported.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      </section>
    </div>
  {{end}}
</section>
