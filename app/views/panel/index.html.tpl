<main class="devtools-panel" data-panel data-state="{{.state.State}}">
  <header class="top-toolbar tabbed-pane-header">
    <nav class="panel-tabs" aria-label="GoLazy panel sections">
      <button type="button" data-tab="requests" aria-selected="false">Requests</button>
      <button type="button" data-tab="console" aria-selected="false">Console</button>
      <button type="button" data-tab="logs" aria-selected="true">App Logs</button>
      <button type="button" data-tab="services" aria-selected="false">Services</button>
      <button type="button" data-tab="traces" aria-selected="false">Traces</button>
      <button type="button" data-tab="routes" aria-selected="false">Routes</button>
      <button type="button" data-tab="jobs" aria-selected="false">Jobs</button>
      <button type="button" data-tab="assets" aria-selected="false">Assets</button>
      <button type="button" data-tab="actions" aria-selected="false">Actions</button>
    </nav>
    <button type="button" class="panel-close-button" data-panel-close hidden aria-label="Close GoLazy development panel" title="Close GoLazy development panel"></button>
  </header>

  <turbo-frame id="requests" src="/_golazy/requests"></turbo-frame>
  <turbo-frame id="console" src="/_golazy/console"></turbo-frame>
  <turbo-frame id="logs" src="/_golazy/logs"></turbo-frame>
  <turbo-frame id="services" src="/_golazy/services"></turbo-frame>
  <turbo-frame id="traces" src="/_golazy/traces"></turbo-frame>
  <turbo-frame id="routes" src="/_golazy/routes"></turbo-frame>
  <turbo-frame id="jobs" src="/_golazy/jobs"></turbo-frame>
  <turbo-frame id="assets" src="/_golazy/assets"></turbo-frame>
  <turbo-frame id="actions" src="/_golazy/actions"></turbo-frame>
  <turbo-frame id="status_bar" src="/_golazy/status"></turbo-frame>
</main>
