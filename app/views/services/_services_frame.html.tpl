<section id="services" class="tool-view is-active services-view" data-view="services">
  <div class="filter-toolbar">
    <span>Services</span>
    <span class="toolbar-divider"></span>
    <span class="toolbar-count" data-service-output-title>Select a service</span>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-service-output-count>0 messages</span>
  </div>

  <div class="services-layout" data-services-panel>
    <aside class="services-sidebar" aria-label="Development services">
      <ul class="service-list" data-service-list>
        {{range .state.Services}}
          <li>
            <button type="button" data-service-select="{{.Name}}" data-service-state="{{.State}}">
              <span class="service-dot"></span>
              <span>{{.Name}}</span>
            </button>
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
          <tr>
            <td colspan="3" class="empty-cell">Select a service to inspect output.</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</section>
