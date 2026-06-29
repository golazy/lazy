<section id="services" class="tool-view is-active services-view" data-view="services">
  <div class="filter-toolbar">
    <span>Services</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count">{{if .selected_service}}{{.selected_service}} output{{else}}Select a service{{end}}</span>
    {{if .selected_service_task}}
      <span class="toolbar-divider"></span>
      <span class="toolbar-count">{{.selected_service_task}}</span>
    {{end}}
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-service-output-count>{{len .service_output_rows}} messages</span>
  </div>

  <div class="type-filter service-task-filter" aria-label="Service task log filters" data-service-task-filter>
    {{partial "service_task_filters" .}}
  </div>

  <div class="services-layout" data-services-panel>
    <aside class="services-sidebar" aria-label="Development services">
      <ul class="service-list" data-service-list>
        {{partial "service_list" .}}
      </ul>
    </aside>

    <section class="service-output-pane" aria-label="Service output">
      <table class="data-grid service-output-grid">
        <thead>
          <tr>
            <th>script</th>
            <th>run</th>
            <th>stdout/stderr</th>
            <th>timestamp</th>
            <th>message</th>
          </tr>
        </thead>
        <tbody data-service-output>
          {{if .defer_panel_lists}}
          {{else}}
            {{partial "service_output_rows" .}}
          {{end}}
        </tbody>
      </table>
    </section>
  </div>
</section>
