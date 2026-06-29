{{range .service_nodes}}
  <li class="service-list-item" data-service-state="{{.State}}">
    <a class="service-list-link" href="{{.URL}}" data-turbo-frame="_top" data-service-select="{{.Name}}" aria-selected="{{if .Selected}}true{{else}}false{{end}}">
      <span class="service-dot"></span>
      <span class="service-name">{{.Label}}</span>
    </a>
    {{if .App}}
    {{else}}
      {{if .Running}}
        <form class="service-action-form" method="post" action="{{path_for "restart_service" .Name}}">
          <button type="submit" class="service-action-button service-restart-button" aria-label="Restart {{.Name}} service" title="Restart service"></button>
        </form>
        <form class="service-action-form" method="post" action="{{path_for "stop_service" .Name}}">
          <button type="submit" class="service-action-button service-stop-button" aria-label="Stop {{.Name}} service" title="Stop service"></button>
        </form>
      {{else}}
        <form class="service-action-form" method="post" action="{{path_for "start_service" .Name}}">
          <button type="submit" class="service-action-button service-start-button" aria-label="Start {{.Name}} service" title="Start service"></button>
        </form>
      {{end}}
    {{end}}
    {{if .Tasks}}
      <ul class="service-task-tree">
        {{range .Tasks}}
          <li>
            <a href="{{.URL}}" data-turbo-frame="_top" aria-current="{{if .Selected}}page{{else}}false{{end}}">
              <span class="service-task-icon"></span>
              <span>{{.Label}}</span>
            </a>
          </li>
        {{end}}
      </ul>
    {{end}}
  </li>
{{else}}
  <li class="muted">No services discovered.</li>
{{end}}
{{if .mise_tasks}}
  <li class="service-list-separator" aria-hidden="true"></li>
  {{range .mise_tasks}}
    <li class="service-list-item service-task-item">
      <span class="service-list-link">
        <span class="service-task-icon"></span>
        <span class="service-name">{{.Label}}</span>
      </span>
    </li>
  {{end}}
{{end}}
