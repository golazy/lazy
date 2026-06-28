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
          <button type="button" class="toolbar-button" data-cache-action="/_golazy/cache/on">On</button>
          <button type="button" class="toolbar-button" data-cache-action="/_golazy/cache/off">Off</button>
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
      <ul class="cache-keys compact-list" data-cache-keys>
        <li class="muted">No keys.</li>
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
