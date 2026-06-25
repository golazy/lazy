<main class="panel-shell" data-panel>
  <header class="panel-header">
    <div>
      <p class="eyebrow">GoLazy</p>
      <h1>Development Panel</h1>
    </div>
    <div class="status-pill" data-panel-state>{{.state.State}}</div>
  </header>

  <section class="toolbar">
    <form method="post" action="/_golazy/rebuild">
      <button type="submit">Rebuild</button>
    </form>
    <form method="post" action="/_golazy/restart">
      <button type="submit">Restart</button>
    </form>
    <a href="/">Open app</a>
  </section>

  <section class="summary-grid">
    <article>
      <h2>Status</h2>
      <dl>
        <dt>Message</dt>
        <dd data-panel-message>{{.state.Message}}</dd>
        <dt>Build</dt>
        <dd data-panel-build>{{.state.BuildCount}}</dd>
        <dt>Duration</dt>
        <dd data-panel-duration>{{.state.Duration}}</dd>
      </dl>
    </article>
    <article>
      <h2>Application</h2>
      <dl>
        <dt>Command</dt>
        <dd><code>{{.state.CommandPath}}</code></dd>
        <dt>Watched root</dt>
        <dd><code>{{.state.WatchedRoot}}</code></dd>
        <dt>App address</dt>
        <dd><code data-panel-app-addr>{{.state.AppAddr}}</code></dd>
        <dt>Control plane</dt>
        <dd><code data-panel-control-addr>{{.state.ControlPlaneAddr}}</code></dd>
      </dl>
    </article>
  </section>

  <section class="panel-section" data-cache-panel>
    <div class="section-heading">
      <h2>View Cache</h2>
      <div class="cache-actions">
        <button type="button" data-cache-action="/_golazy/cache/on">On</button>
        <button type="button" data-cache-action="/_golazy/cache/off">Off</button>
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
    <ul class="cache-keys" data-cache-keys>
      <li class="muted">No keys.</li>
    </ul>
  </section>

  <section class="panel-section">
    <h2>Changed Files</h2>
    <ul data-panel-changes>
      {{range .state.Changed}}
        <li><code>{{.}}</code></li>
      {{else}}
        <li class="muted">No recent changes.</li>
      {{end}}
    </ul>
  </section>

  <section class="panel-section">
    <h2>Latest Output</h2>
    <pre data-panel-output>{{.state.Output}}</pre>
  </section>

  <section class="panel-section">
    <h2>Events</h2>
    <ol class="event-list" data-panel-events>
      {{range .state.Events}}
        <li><span>{{.Time.Format "15:04:05"}}</span> <strong>{{.Type}}</strong> {{.Message}}</li>
      {{else}}
        <li class="muted">Waiting for development events.</li>
      {{end}}
    </ol>
  </section>
</main>
