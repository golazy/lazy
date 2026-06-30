<section id="pwa" class="tool-view is-active" data-view="pwa">
  <div>
    <div class="section-heading">
      <h2>PWA</h2>
      {{if .pwa_error}}
        <span class="toolbar-count">Unavailable</span>
      {{else if .pwa_state.Enabled}}
        <span class="toolbar-count">Enabled</span>
      {{else}}
        <span class="toolbar-count">Disabled</span>
      {{end}}
    </div>
    {{if .pwa_error}}
      <p class="muted">{{.pwa_error}}</p>
    {{else}}
      <table class="data-grid">
        <tbody>
          <tr><th>Version</th><td>{{if .pwa_state.Version}}<code>{{.pwa_state.Version}}</code>{{else}}-{{end}}</td></tr>
          <tr><th>Manifest</th><td>{{if .pwa_state.ManifestPath}}<code>{{.pwa_state.ManifestPath}}</code>{{else}}-{{end}}</td></tr>
          <tr><th>Service Worker</th><td>{{if .pwa_state.ServiceWorkerPath}}<code>{{.pwa_state.ServiceWorkerName}}</code> <code>{{.pwa_state.ServiceWorkerPath}}</code>{{else}}-{{end}}</td></tr>
          <tr><th>Offline</th><td>{{if .pwa_state.Offline.Enabled}}{{.pwa_state.Offline.URLs}} URLs{{if .pwa_state.Offline.Fallback}} with fallback <code>{{.pwa_state.Offline.Fallback}}</code>{{end}}{{else}}Disabled{{end}}</td></tr>
          <tr><th>Push</th><td>{{if .pwa_state.Push.Enabled}}Enabled{{else}}Disabled{{end}}</td></tr>
        </tbody>
      </table>
    {{end}}
  </div>

  <div class="filter-toolbar">
    <form method="get" action="{{path_for "pwa"}}" class="inline-form" data-controller="debounced-form" data-action="input->debounced-form#queue submit->debounced-form#submit" data-debounced-form-count-source-value="[data-pwa-workers-frame-count]" data-debounced-form-count-target-value="[data-pwa-workers-count]" data-debounced-form-delay-value="250" data-debounced-form-frame-value="pwa_workers_table" data-turbo-frame="pwa_workers_table">
      <input class="filter-input" type="search" name="q" placeholder="Filter workers" value="{{.pwa_query}}">
    </form>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-pwa-workers-count>{{.workers_count_text}}</span>
  </div>
  <turbo-frame id="pwa_workers_table" class="assets-table-frame">
    {{partial "workers_table" .}}
  </turbo-frame>
</section>
