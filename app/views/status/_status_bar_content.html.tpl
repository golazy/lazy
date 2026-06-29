<footer id="status_bar_content" class="status-bar">
  <a class="app-status-chip" href="{{path_for "logs"}}" data-turbo-frame="_top" data-app-status="{{.state.State}}">
    <span class="service-dot"></span>
    <span>App</span>
  </a>
  <span>Build <span>{{.state.BuildCount}}</span></span>
  <span>{{.state.Duration}}</span>
  <span>App <code>{{.state.AppAddr}}</code></span>
  <span>Control <code>{{.state.ControlPlaneAddr}}</code></span>
  <span>{{.cache.StatusText}}</span>
  <span>{{.monitoring.StatusText}}</span>
  <div class="service-status-strip" aria-label="Development service status">
    {{range .state.Services}}
      <a class="service-status-button" href="{{path_for "services"}}?service={{.Name}}" data-turbo-frame="_top" data-service-state="{{.State}}" title="{{.Message}}">
        <span class="service-dot"></span>
        <span>{{.Name}}</span>
      </a>
    {{else}}
      <span class="muted">No services</span>
    {{end}}
  </div>
</footer>
