<section id="services" class="tool-view is-active services-view" data-view="services">
  <div class="filter-toolbar">
    <span>Services</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{if .selected_service}}{{.selected_service}} output{{else}}Select a service{{end}}</span>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count">{{len .service_output_rows}} messages</span>
  </div>

  <div class="services-layout" data-services-panel>
    <aside class="services-sidebar" aria-label="Development services">
      <ul class="service-list" data-service-list>
        {{range .state.Services}}
          <li>
            <a href="{{path_for "services"}}?service={{.Name}}" data-turbo-frame="_top" data-service-select="{{.Name}}" data-service-state="{{.State}}" aria-selected="{{if eq .Name $.selected_service}}true{{else}}false{{end}}">
              <span class="service-dot"></span>
              <span>{{.Name}}</span>
            </a>
          </li>
        {{else}}
          <li class="muted">No services discovered.</li>
        {{end}}
      </ul>
    </aside>

    <section class="service-output-pane" aria-label="Service output">
      <table class="data-grid service-output-grid">
        <thead>
          <tr>
            <th>stdout/stderr</th>
            <th>timestamp</th>
            <th>message</th>
          </tr>
        </thead>
        <tbody data-service-output>
          {{range .service_output_rows}}
            <tr>
              <td>{{.Stream}}</td>
              <td>{{.Time}}</td>
              <td>{{.Message}}</td>
            </tr>
          {{else}}
            <tr>
              <td colspan="3" class="empty-cell">{{if $.selected_service}}No output recorded for this service.{{else}}Select a service to inspect output.{{end}}</td>
            </tr>
          {{end}}
        </tbody>
      </table>
    </section>
  </div>
</section>
