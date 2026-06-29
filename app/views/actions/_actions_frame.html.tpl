<section id="actions" class="tool-view is-active" data-view="actions">
  <div class="filter-toolbar">
    <form method="post" action="/_golazy/rebuild">
      <button type="submit" class="toolbar-button">Rebuild</button>
    </form>
    <form method="post" action="/_golazy/restart">
      <button type="submit" class="toolbar-button">Restart</button>
    </form>
    <a class="toolbar-button" href="/">Open app</a>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count">Development actions</span>
  </div>

  <div class="action-layout">
    <section class="runtime-pane" data-cache-panel>
      <div class="section-heading">
        <h2>View Cache</h2>
        <div class="cache-actions">
          <form method="post" action="/_golazy/cache/on">
            <button type="submit" class="toolbar-button">On</button>
          </form>
          <form method="post" action="/_golazy/cache/off">
            <button type="submit" class="toolbar-button">Off</button>
          </form>
        </div>
      </div>
      <dl class="cache-stats">
        <dt>Status</dt>
        <dd>{{.cache.StatusText}}</dd>
        <dt>Entries</dt>
        <dd>{{.cache.Stats.Entries}}</dd>
        <dt>Hits</dt>
        <dd>{{.cache.Stats.Hits}}</dd>
        <dt>Misses</dt>
        <dd>{{.cache.Stats.Misses}}</dd>
        <dt>Sets</dt>
        <dd>{{.cache.Stats.Sets}}</dd>
        <dt>Evictions</dt>
        <dd>{{.cache.Stats.Evictions}}</dd>
      </dl>
      <ul class="cache-keys compact-list" data-cache-keys>
        {{range .cache.Keys}}
          <li><code>{{.}}</code></li>
        {{else}}
          <li class="muted">{{if $.cache.Error}}{{$.cache.Error}}{{else}}No keys.{{end}}</li>
        {{end}}
      </ul>
    </section>

    <section class="runtime-pane">
      <h2>Action Notes</h2>
      <ul class="compact-list">
        <li>Rebuild recompiles the app and restarts the child process.</li>
        <li>Restart starts the latest successful build without recompiling.</li>
        <li>Cache controls proxy to the app lazydev control plane.</li>
      </ul>
    </section>
  </div>
</section>
