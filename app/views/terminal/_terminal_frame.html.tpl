<section id="terminal" class="tool-view is-active terminal-view" data-view="terminal" data-controller="terminal" data-terminal-url-value="{{path_for "terminal"}}/connect">
  <div class="terminal-toolbar filter-toolbar">
    <span class="status-chip" data-terminal-target="status">Disconnected</span>
    <span class="toolbar-spacer"></span>
    <span class="terminal-cwd" title="{{.terminal.Dir}}">{{.terminal.Dir}}</span>
  </div>
  <div class="terminal-pane" data-terminal-target="container" tabindex="0" aria-label="GoLazy terminal"></div>
</section>
