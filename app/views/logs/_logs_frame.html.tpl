<section id="logs" class="tool-view is-active" data-view="logs">
  <div class="filter-toolbar">
    <span class="state-chip" data-state-chip data-state-value="{{.state.State}}">
      <span class="state-dot"></span>
      <span data-panel-state>{{.state.State}}</span>
    </span>
    <span class="toolbar-divider"></span>
    <span data-panel-message>{{.state.Message}}</span>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count">Build <span data-panel-build>{{.state.BuildCount}}</span></span>
  </div>

  <div class="runtime-grid" data-controller="panel-resize" data-panel-resize-direction-value="right" data-panel-resize-min-value="320px" data-panel-resize-max-value="72%">
    <div class="runtime-left-stack" data-panel-resize-target="primary">
      <div class="runtime-summary-grid">
        <section class="runtime-pane">
          <h2>Status</h2>
          <dl class="detail-list">
            <dt>Message</dt>
            <dd data-panel-message>{{.state.Message}}</dd>
            <dt>Build</dt>
            <dd data-panel-build>{{.state.BuildCount}}</dd>
            <dt>Duration</dt>
            <dd data-panel-duration>{{.state.Duration}}</dd>
            <dt>Command</dt>
            <dd><code>{{.state.CommandPath}}</code></dd>
            <dt>Watched root</dt>
            <dd><code>{{.state.WatchedRoot}}</code></dd>
            <dt>App address</dt>
            <dd><code data-panel-app-addr>{{.state.AppAddr}}</code></dd>
            <dt>Control plane</dt>
            <dd><code data-panel-control-addr>{{.state.ControlPlaneAddr}}</code></dd>
          </dl>
        </section>

        <section class="runtime-pane">
          <h2>Changed Files</h2>
          <ul class="compact-list" data-panel-changes>
            {{range .state.Changed}}
              <li><code>{{.}}</code></li>
            {{else}}
              <li class="muted">No recent changes.</li>
            {{end}}
          </ul>
        </section>
      </div>

      <section class="runtime-pane events-pane">
        <h2>Events</h2>
        <ol class="event-list" id="panel_events" data-panel-events>
          {{range .state.Events}}
            {{partial "event_item" .}}
          {{else}}
            <li class="muted">Waiting for development events.</li>
          {{end}}
        </ol>
      </section>
    </div>

    <div class="split-resize-handle runtime-output-resize" data-panel-resize-target="handle" data-action="pointerdown->panel-resize#start keydown->panel-resize#nudge" aria-label="Resize app log detail panes"></div>

    <section class="runtime-pane output-pane">
      <h2>Latest Output</h2>
      <pre class="panel-output" data-panel-output>{{.state.Output}}</pre>
    </section>
  </div>
</section>
