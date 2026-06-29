<section id="buildinfo" class="tool-view is-active buildinfo-view" data-view="buildinfo" data-buildinfo-panel>
  <div class="filter-toolbar">
    <span>BuildInfo</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{.buildinfo.StatusText}}</span>
  </div>

  {{if .buildinfo.Error}}
    <div class="empty-state">BuildInfo unavailable: {{.buildinfo.Error}}</div>
  {{else if not .buildinfo.Available}}
    <div class="empty-state">The running app did not report Go module build information.</div>
  {{else}}
    <div class="buildinfo-layout">
      <section class="runtime-pane buildinfo-summary-pane">
        <div class="section-heading">
          <h2>Command</h2>
        </div>
        <dl class="detail-list">
          <dt>Path</dt>
          <dd><code>{{.buildinfo.Path}}</code></dd>
          <dt>Go</dt>
          <dd>{{.buildinfo.GoVersion}}</dd>
          <dt>Main</dt>
          <dd><code>{{.buildinfo.Main.Path}}</code></dd>
          <dt>Version</dt>
          <dd>{{.buildinfo.Main.VersionText}}</dd>
          <dt>Sum</dt>
          <dd>{{.buildinfo.Main.SumText}}</dd>
          {{if .buildinfo.Main.Replace}}
            <dt>Replace</dt>
            <dd><code>{{.buildinfo.Main.ReplaceText}}</code></dd>
          {{end}}
        </dl>
      </section>

      <section class="runtime-pane buildinfo-table-pane buildinfo-settings-pane">
        <h2>Settings</h2>
        <table class="data-grid buildinfo-settings-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>Key</th>
              <th>Value</th>
            </tr>
          </thead>
          <tbody>
            {{range .buildinfo.Settings}}
              <tr>
                <td><code>{{.Key}}</code></td>
                <td><code>{{.Value}}</code></td>
              </tr>
            {{else}}
              <tr>
                <td colspan="2" class="empty-cell">No build settings reported.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      </section>

      <section class="runtime-pane buildinfo-table-pane buildinfo-deps-pane">
        <h2>Dependencies</h2>
        <table class="data-grid buildinfo-deps-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>Module</th>
              <th>Version</th>
              <th>Replace</th>
              <th>Sum</th>
            </tr>
          </thead>
          <tbody>
            {{range .buildinfo.Deps}}
              <tr>
                <td><code>{{.Path}}</code></td>
                <td>{{.VersionText}}</td>
                <td>{{if .Replace}}<code>{{.ReplaceText}}</code>{{else}}-{{end}}</td>
                <td><code>{{.SumText}}</code></td>
              </tr>
            {{else}}
              <tr>
                <td colspan="4" class="empty-cell">No module dependencies reported.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      </section>
    </div>
  {{end}}
</section>
