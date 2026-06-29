<section id="dependencies" class="tool-view is-active dependencies-view" data-view="dependencies" data-dependencies-panel>
  <div class="filter-toolbar">
    <span>Dependency Graph</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{.dependencies.StatusText}}</span>
  </div>

  {{if .dependencies.Error}}
    <div class="empty-state">Dependency graph unavailable: {{.dependencies.Error}}</div>
  {{else}}
    <div class="dependencies-layout">
      <section class="runtime-pane dependencies-summary-pane">
        <div class="section-heading">
          <h2>Graph</h2>
        </div>
        <dl class="detail-list">
          <dt>Services</dt>
          <dd>{{.dependency_counts.Services}}</dd>
          <dt>Edges</dt>
          <dd>{{.dependency_counts.Edges}}</dd>
          <dt>Root</dt>
          <dd><code>app</code></dd>
        </dl>
      </section>

      <section class="runtime-pane dependencies-table-pane">
        <h2>Services</h2>
        <table class="data-grid dependencies-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>Service</th>
              <th>Depends On</th>
              <th>Used By</th>
            </tr>
          </thead>
          <tbody>
            {{range .dependency_nodes}}
              <tr>
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
