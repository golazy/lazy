<section id="app" class="tool-view is-active app-view" data-view="app" data-app-panel>
  <div class="filter-toolbar">
    <span>App</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{.state.State}}</span>
    <span class="toolbar-spacer"></span>
    <form method="post" action="/_golazy/rebuild">
      <button type="submit" class="toolbar-button">Rebuild</button>
    </form>
    <form method="post" action="/_golazy/restart">
      <button type="submit" class="toolbar-button">Restart</button>
    </form>
    {{if .state.AppAddr}}
      <a class="toolbar-button" href="http://{{.state.AppAddr}}" target="_blank" rel="noreferrer">Open App</a>
    {{else}}
      <span class="toolbar-button is-disabled" aria-disabled="true">Open App</span>
    {{end}}
  </div>

  <div class="app-layout">
    <section class="runtime-pane app-services-pane">
      <div class="section-heading">
        <h2>Services</h2>
      </div>
      <table class="data-grid app-services-grid">
        <thead>
          <tr>
            <th>service</th>
            <th>status</th>
            <th>message</th>
          </tr>
        </thead>
        <tbody>
          {{range .service_rows}}
            <tr data-service-state="{{.State}}">
              <td>{{.Name}}</td>
              <td>{{.State}}</td>
              <td>{{.Message}}</td>
            </tr>
          {{else}}
            <tr>
              <td colspan="3" class="empty-cell">No services discovered.</td>
            </tr>
          {{end}}
        </tbody>
      </table>
    </section>

    <section class="runtime-pane app-log-pane">
      <div class="section-heading">
        <h2>Lazy Logs</h2>
      </div>
      <table class="data-grid app-log-grid">
        <thead>
          <tr>
            <th>time</th>
            <th>source</th>
            <th>event</th>
            <th>message</th>
            <th>details</th>
          </tr>
        </thead>
        <tbody>
          {{range .app_log_rows}}
            <tr>
              <td>{{.Time}}</td>
              <td>{{.Source}}</td>
              <td>{{.Event}}</td>
              <td>{{.Message}}</td>
              <td>{{.Details}}</td>
            </tr>
          {{else}}
            <tr>
              <td colspan="5" class="empty-cell">No lazy events yet.</td>
            </tr>
          {{end}}
        </tbody>
      </table>
    </section>

    <section class="runtime-pane app-changes-pane">
      <div class="section-heading">
        <h2>Changed Files</h2>
      </div>
      <div class="app-change-list">
        {{range .change_groups}}
          <article class="app-change-group">
            <div class="app-change-summary">
              <span>{{.Time}}</span>
              <span>{{.Build}}</span>
              {{if .Duration}}<span>{{.Duration}}</span>{{end}}
              <strong>{{.Message}}</strong>
            </div>
            <ul>
              {{range .Files}}
                <li><code>{{.}}</code></li>
              {{end}}
            </ul>
          </article>
        {{else}}
          <div class="empty-state">No changed files recorded yet.</div>
        {{end}}
      </div>
    </section>
  </div>
</section>
