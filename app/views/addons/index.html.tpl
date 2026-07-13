{{ partial "nav" . }}
<div class="panel-page-body">
  <section id="addons" class="tool-view is-active" data-view="addons">
    <div class="addon-panels-layout">
      <aside class="addon-panel-list" aria-label="Registered add-on panels">
        <div class="section-heading">
          <h2>Add-ons</h2>
          {{if .addons_error}}
            <span class="toolbar-count">Unavailable</span>
          {{else}}
            <span class="toolbar-count">{{len .addon_panels}}</span>
          {{end}}
        </div>
        {{if .addons_error}}
          <div class="empty-state">
            <strong>Panels unavailable</strong>
            <p>{{.addons_error}}</p>
          </div>
        {{else if .addon_panels}}
          <div class="addon-panel-items">
            {{range .addon_panels}}
              <a class="addon-panel-item{{if .Selected}} is-selected{{end}}" href="{{.URL}}" data-turbo-frame="_top"{{if .Selected}} aria-current="page"{{end}}>
                <strong>{{.Title}}</strong>
                <span>{{.Description}}</span>
                <code>{{.Owner}}</code>
              </a>
            {{end}}
          </div>
        {{else}}
          <div class="empty-state">
            <strong>No add-on panels</strong>
            <p>The running application has not registered any development panels.</p>
          </div>
        {{end}}
      </aside>

      <div class="addon-panel-content">
        {{if .has_selected_addon_panel}}
          <header class="addon-panel-heading">
            <div>
              <h2>{{.selected_addon_panel.Title}}</h2>
              <p>{{.selected_addon_panel.Description}}</p>
            </div>
            <div class="addon-panel-heading-actions">
              {{range .selected_addon_panel.Actions}}
                <form class="addon-panel-action-form" method="post" action="{{$.addon_action_path}}" target="addon-panel-frame">
                  <input type="hidden" name="panel" value="{{$.selected_addon_panel.ID}}">
                  <input type="hidden" name="action" value="{{.ID}}">
                  <input type="hidden" name="csrf_token" value="{{$.addon_action_csrf_token}}">
                  <button class="toolbar-button addon-panel-action-button" type="submit" title="{{.Description}}">{{.Title}}</button>
                </form>
              {{end}}
              <code>{{.selected_addon_panel.Path}}</code>
            </div>
          </header>
          <iframe class="addon-panel-frame" name="addon-panel-frame" src="{{.selected_addon_panel.ContentURL}}" title="{{.selected_addon_panel.Title}}" sandbox="" referrerpolicy="no-referrer"></iframe>
        {{else if not .addons_error}}
          <div class="empty-state">Select a registered add-on panel.</div>
        {{end}}
      </div>
    </div>
  </section>
</div>
