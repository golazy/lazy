<section id="buildinfo" class="tool-view is-active buildinfo-view" data-view="buildinfo" data-buildinfo-panel>
  <div class="filter-toolbar">
    <span>BuildInfo</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{.buildinfo.StatusText}}</span>
    {{if .buildtrace.Available}}
      <span class="toolbar-count">Build {{.buildtrace.BuildNumber}} - {{.buildtrace.Total}}</span>
    {{end}}
  </div>

  <div class="buildinfo-body">
    {{if .buildtrace.Error}}
      <section class="buildtrace-section">
        <div class="empty-state">Build trace unavailable: {{.buildtrace.Error}}</div>
      </section>
    {{else if .buildtrace.Available}}
      <section class="buildtrace-section">
        <div class="section-heading">
          <h2>Why This Build Took Time</h2>
          <span class="muted">Elapsed {{.buildtrace.Total}}</span>
        </div>

        <div class="buildtrace-phase-grid">
          {{range .buildtrace.Phases}}
            <div class="buildtrace-phase">
              <div class="buildtrace-phase-head">
                <strong>{{.Name}}</strong>
                <span>{{.Duration}}</span>
              </div>
              <div class="buildtrace-meter" aria-hidden="true">
                <span style="width: {{.Width}}"></span>
              </div>
              <span class="muted">{{.Count}}</span>
            </div>
          {{else}}
            <div class="empty-state">No classified build trace spans.</div>
          {{end}}
        </div>

        <div class="buildtrace-detail-grid">
          <section class="runtime-pane buildtrace-table-pane">
            <h2>Slow Packages</h2>
            <table class="data-grid buildtrace-package-grid" data-controller="table-resize">
              <thead>
                <tr>
                  <th>Package</th>
                  <th>Phase</th>
                  <th>Time</th>
                  <th>Spans</th>
                </tr>
              </thead>
              <tbody>
                {{range .buildtrace.Packages}}
                  <tr>
                    <td>
                      <code>{{.Package}}</code>
                      <span class="buildtrace-row-meter" aria-hidden="true"><span style="width: {{.Width}}"></span></span>
                    </td>
                    <td>{{.Phase}}</td>
                    <td>{{.Duration}}</td>
                    <td>{{.Count}}</td>
                  </tr>
                {{else}}
                  <tr>
                    <td colspan="4" class="empty-cell">No package timing reported.</td>
                  </tr>
                {{end}}
              </tbody>
            </table>
          </section>

          <section class="runtime-pane buildtrace-table-pane">
            <h2>Slow Actions</h2>
            <table class="data-grid buildtrace-action-grid" data-controller="table-resize">
              <thead>
                <tr>
                  <th>Action</th>
                  <th>Phase</th>
                  <th>Package</th>
                  <th>Time</th>
                </tr>
              </thead>
              <tbody>
                {{range .buildtrace.Actions}}
                  <tr>
                    <td><code>{{.Name}}</code></td>
                    <td>{{.Phase}}</td>
                    <td>{{if .Package}}<code>{{.Package}}</code>{{else}}-{{end}}</td>
                    <td>{{.Duration}}</td>
                  </tr>
                {{else}}
                  <tr>
                    <td colspan="4" class="empty-cell">No build actions reported.</td>
                  </tr>
                {{end}}
              </tbody>
            </table>
          </section>
        </div>
      </section>
    {{else}}
      <section class="buildtrace-section">
        <div class="empty-state">Build timing appears after the next Go build.</div>
      </section>
    {{end}}

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
  </div>
</section>
