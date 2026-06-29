<section id="buildinfo" class="tool-view is-active buildinfo-view" data-view="buildinfo" data-buildinfo-panel>
  <div class="filter-toolbar">
    <span>BuildInfo</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{.buildinfo.StatusText}}</span>
    {{if .buildtrace.Available}}
      <span class="toolbar-count">Build {{.buildtrace.BuildNumber}} - {{.buildtrace.Total}}</span>
    {{end}}
  </div>

  <div class="buildinfo-layout">
    <aside class="details-pane buildinfo-summary-pane">
      <div class="detail-header">
        <h2>Build Timing</h2>
        {{if .buildtrace.Available}}<span>{{.buildtrace.Total}}</span>{{end}}
      </div>

      {{if .buildtrace.Error}}
        <div class="empty-state">Build trace unavailable: {{.buildtrace.Error}}</div>
      {{else if .buildtrace.Available}}
        <dl class="detail-list buildinfo-facts">
          <dt>Elapsed</dt>
          <dd>{{.buildtrace.Total}}</dd>
          {{if .buildtrace.TopPhase}}
            <dt>Top phase</dt>
            <dd>{{.buildtrace.TopPhase}} <span class="muted">{{.buildtrace.TopPhaseDuration}}</span></dd>
          {{end}}
          {{if .buildtrace.TopPackage}}
            <dt>Top package</dt>
            <dd><code>{{.buildtrace.TopPackage}}</code> <span class="muted">{{.buildtrace.TopPackageDuration}}</span></dd>
          {{end}}
        </dl>

        <div class="buildtrace-phase-list">
          {{range .buildtrace.Phases}}
            <div class="buildtrace-phase-row">
              <div>
                <strong>{{.Name}}</strong>
                <span class="muted">{{.Count}}</span>
              </div>
              <span>{{.Duration}}</span>
              <div class="buildtrace-meter" aria-hidden="true">
                <span style="width: {{.Width}}"></span>
              </div>
            </div>
          {{end}}
        </div>
      {{else}}
        <div class="empty-state">Build timing appears after the next Go build.</div>
      {{end}}

      <div class="detail-header buildinfo-runtime-header">
        <h2>Runtime</h2>
        <span>{{.buildinfo.StatusText}}</span>
      </div>
      {{if .buildinfo.Error}}
        <div class="empty-state">BuildInfo unavailable: {{.buildinfo.Error}}</div>
      {{else if not .buildinfo.Available}}
        <div class="empty-state">The running app did not report Go module build information.</div>
      {{else}}
        <dl class="detail-list buildinfo-facts">
          <dt>Path</dt>
          <dd><code>{{.buildinfo.Path}}</code></dd>
          <dt>Go</dt>
          <dd>{{.buildinfo.GoVersion}}</dd>
          <dt>Main</dt>
          <dd><code>{{.buildinfo.Main.Path}}</code></dd>
          <dt>Version</dt>
          <dd>{{.buildinfo.Main.VersionText}}</dd>
        </dl>
      {{end}}
    </aside>

    <section class="buildinfo-main-pane">
      <div class="details-pane buildinfo-table-section">
        <div class="detail-header">
          <h2>Slowest Packages</h2>
          <span>Cumulative trace time</span>
        </div>
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
            {{if .buildtrace.Available}}
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
            {{else}}
              <tr>
                <td colspan="4" class="empty-cell">Package timing appears after the next Go build.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      </div>

      <div class="details-pane buildinfo-table-section">
        <div class="detail-header">
          <h2>Runtime Details</h2>
          <span>{{.buildinfo.StatusText}}</span>
        </div>
        <table class="data-grid buildinfo-runtime-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>Kind</th>
              <th>Name</th>
              <th>Value</th>
              <th>Extra</th>
            </tr>
          </thead>
          <tbody>
            {{if .buildinfo.Available}}
              <tr>
                <td>Main</td>
                <td><code>{{.buildinfo.Main.Path}}</code></td>
                <td>{{.buildinfo.Main.VersionText}}</td>
                <td><code>{{.buildinfo.Main.SumText}}</code></td>
              </tr>
              {{range .buildinfo.Settings}}
                <tr>
                  <td>Setting</td>
                  <td><code>{{.Key}}</code></td>
                  <td><code>{{.Value}}</code></td>
                  <td>-</td>
                </tr>
              {{end}}
              {{range .buildinfo.Deps}}
                <tr>
                  <td>Dependency</td>
                  <td><code>{{.Path}}</code></td>
                  <td>{{.VersionText}}</td>
                  <td>{{if .Replace}}<code>{{.ReplaceText}}</code>{{else}}<code>{{.SumText}}</code>{{end}}</td>
                </tr>
              {{else}}
                <tr>
                  <td colspan="4" class="empty-cell">No module dependencies reported.</td>
                </tr>
              {{end}}
            {{else if .buildinfo.Error}}
              <tr>
                <td colspan="4" class="empty-cell">BuildInfo unavailable: {{.buildinfo.Error}}</td>
              </tr>
            {{else}}
              <tr>
                <td colspan="4" class="empty-cell">The running app did not report Go module build information.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </section>
  </div>
</section>
