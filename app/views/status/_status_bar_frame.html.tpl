<footer class="status-bar">
  <span>State: <strong data-panel-state>{{.state.State}}</strong></span>
  <span>Build <span data-panel-build>{{.state.BuildCount}}</span></span>
  <span data-panel-duration>{{.state.Duration}}</span>
  <span>App <code data-panel-app-addr>{{.state.AppAddr}}</code></span>
  <span>Control <code data-panel-control-addr>{{.state.ControlPlaneAddr}}</code></span>
  <span data-cache-state>Cache unknown</span>
  <span class="status-spacer"></span>
  <div class="service-status-strip" data-service-statuses aria-label="Development service status">
    {{range .state.Services}}
      <button type="button" class="service-status-button" data-service-status data-service-name="{{.Name}}" data-service-state="{{.State}}">
        <span class="service-dot"></span>
        <span>{{.Name}}</span>
      </button>
    {{else}}
      <span class="muted" data-service-status-empty>No services</span>
    {{end}}
  </div>
</footer>
