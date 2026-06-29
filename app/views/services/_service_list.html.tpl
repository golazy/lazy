{{range .state.Services}}
  <li class="service-list-item" data-service-state="{{.State}}">
    <a class="service-list-link" href="{{path_for "services"}}?service={{.Name}}" data-turbo-frame="_top" data-service-select="{{.Name}}" aria-selected="{{if eq .Name $.selected_service}}true{{else}}false{{end}}">
      <span class="service-dot"></span>
      <span class="service-name">{{.Name}}</span>
    </a>
    <form class="service-restart-form" method="post" action="{{path_for "restart_service" .Name}}">
      <button type="submit" class="service-restart-button" aria-label="Restart {{.Name}} service" title="Restart service"></button>
    </form>
  </li>
{{else}}
  <li class="muted">No services discovered.</li>
{{end}}
