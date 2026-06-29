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
        <h2>Build</h2>
        {{if .buildtrace.Available}}<span>{{.buildtrace.Total}}</span>{{else}}<span>{{.buildinfo.StatusText}}</span>{{end}}
      </div>
      <dl class="detail-list buildinfo-facts">
        <dt>Build time</dt>
        <dd>{{if .buildtrace.Available}}{{.buildtrace.Total}}{{else}}Pending{{end}}</dd>
        {{if .buildinfo.Available}}
          <dt>Go</dt>
          <dd>{{.buildinfo.GoVersion}}</dd>
          <dt>Command</dt>
          <dd><code class="buildinfo-value" title="{{.buildinfo.Path}}">{{.buildinfo.Path}}</code></dd>
          <dt>Main</dt>
          <dd><code class="buildinfo-value" title="{{.buildinfo.Main.Path}}">{{.buildinfo.Main.Path}}</code></dd>
          <dt>Version</dt>
          <dd>{{.buildinfo.Main.VersionText}}</dd>
        {{else if .buildinfo.Error}}
          <dt>Runtime</dt>
          <dd>{{.buildinfo.Error}}</dd>
        {{else}}
          <dt>Runtime</dt>
          <dd>No module build info</dd>
        {{end}}
      </dl>

      <div class="detail-header buildinfo-section-header">
        <h2>Build Phases</h2>
        {{if .buildtrace.Available}}<span>{{len .buildtrace.Phases}} phases</span>{{end}}
      </div>
      {{if .buildtrace.Error}}
        <div class="empty-state">Build trace unavailable: {{.buildtrace.Error}}</div>
      {{else if .buildtrace.Available}}
        <div class="buildtrace-phase-list">
          {{range .buildtrace.Phases}}
            <div class="buildtrace-phase-row">
              <strong class="buildtrace-phase-name" title="{{.Name}}">{{.Name}}</strong>
              <span class="buildtrace-duration">{{.Duration}}</span>
              <span class="buildtrace-meter" aria-hidden="true">
                <span style="width: {{.Width}}"></span>
              </span>
            </div>
          {{end}}
        </div>
      {{else}}
        <div class="empty-state">Build timing appears after the next Go build.</div>
      {{end}}

      <div class="detail-header buildinfo-section-header">
        <h2>Top Packages</h2>
        <span>Top 5</span>
      </div>
      <table class="data-grid buildtrace-top-package-grid">
        <thead>
          <tr>
            <th>Package</th>
            <th>Phase</th>
            <th>Time</th>
          </tr>
        </thead>
        <tbody>
          {{if .buildtrace.Available}}
            {{range .buildtrace.Packages}}
              <tr>
                <td><code class="buildtrace-package-name" title="{{.Package}}">{{.Package}}</code></td>
                <td>{{.Phase}}</td>
                <td>{{.Duration}}</td>
              </tr>
            {{else}}
              <tr>
                <td colspan="3" class="empty-cell">No package timing reported.</td>
              </tr>
            {{end}}
          {{else}}
            <tr>
              <td colspan="3" class="empty-cell">Package timing appears after the next Go build.</td>
            </tr>
          {{end}}
        </tbody>
      </table>
    </aside>

    <section class="buildinfo-detail-pane">
      <nav class="request-detail-tabs buildinfo-detail-tabs" aria-label="BuildInfo detail tabs">
        <a href="{{.buildtabs.TabURL "runtime"}}" data-turbo-frame="_top" aria-current="{{if .buildtabs.RuntimeTab}}page{{end}}">Runtime Details</a>
        <a href="{{.buildtabs.TabURL "settings"}}" data-turbo-frame="_top" aria-current="{{if .buildtabs.SettingsTab}}page{{end}}">Settings</a>
        <a href="{{.buildtabs.TabURL "dependencies"}}" data-turbo-frame="_top" aria-current="{{if .buildtabs.DependenciesTab}}page{{end}}">Dependencies</a>
      </nav>

      <div class="buildinfo-tab-panel">
        {{if .buildtabs.SettingsTab}}
          <table class="data-grid buildinfo-runtime-grid" data-controller="table-resize">
            <thead>
              <tr>
                <th>Setting</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              {{if .buildinfo.Available}}
                {{range .buildinfo.Settings}}
                  <tr>
                    <td><code>{{.Key}}</code></td>
                    <td><code>{{.Value}}</code></td>
                  </tr>
                {{else}}
                  <tr>
                    <td colspan="2" class="empty-cell">No recorded build settings.</td>
                  </tr>
                {{end}}
              {{else if .buildinfo.Error}}
                <tr>
                  <td colspan="2" class="empty-cell">BuildInfo unavailable: {{.buildinfo.Error}}</td>
                </tr>
              {{else}}
                <tr>
                  <td colspan="2" class="empty-cell">The running app did not report Go module build information.</td>
                </tr>
              {{end}}
            </tbody>
          </table>
        {{else if .buildtabs.DependenciesTab}}
          <table class="data-grid buildinfo-runtime-grid" data-controller="table-resize">
            <thead>
              <tr>
                <th>Module</th>
                <th>Version</th>
                <th>Replacement or Sum</th>
              </tr>
            </thead>
            <tbody>
              {{if .buildinfo.Available}}
                {{range .buildinfo.Deps}}
                  <tr>
                    <td><code>{{.Path}}</code></td>
                    <td>{{.VersionText}}</td>
                    <td>{{if .Replace}}<code>{{.ReplaceText}}</code>{{else}}<code>{{.SumText}}</code>{{end}}</td>
                  </tr>
                {{else}}
                  <tr>
                    <td colspan="3" class="empty-cell">No module dependencies reported.</td>
                  </tr>
                {{end}}
              {{else if .buildinfo.Error}}
                <tr>
                  <td colspan="3" class="empty-cell">BuildInfo unavailable: {{.buildinfo.Error}}</td>
                </tr>
              {{else}}
                <tr>
                  <td colspan="3" class="empty-cell">The running app did not report Go module build information.</td>
                </tr>
              {{end}}
            </tbody>
          </table>
        {{else}}
          <section class="runtime-pane buildinfo-runtime-pane">
            <dl class="detail-list buildinfo-runtime-details">
              {{if .buildinfo.Available}}
                <dt>Command</dt>
                <dd><code>{{.buildinfo.Path}}</code></dd>
                <dt>Go</dt>
                <dd>{{.buildinfo.GoVersion}}</dd>
                <dt>Main module</dt>
                <dd><code>{{.buildinfo.Main.Path}}</code></dd>
                <dt>Main version</dt>
                <dd>{{.buildinfo.Main.VersionText}}</dd>
                <dt>Main sum</dt>
                <dd><code>{{.buildinfo.Main.SumText}}</code></dd>
                <dt>Dependencies</dt>
                <dd>{{.buildinfo.StatusText}}</dd>
              {{else if .buildinfo.Error}}
                <dt>Status</dt>
                <dd>BuildInfo unavailable: {{.buildinfo.Error}}</dd>
              {{else}}
                <dt>Status</dt>
                <dd>The running app did not report Go module build information.</dd>
              {{end}}
            </dl>
          </section>
        {{end}}
      </div>
    </section>
  </div>
</section>
