<section id="routes" class="tool-view is-active" data-view="routes">
  <div class="filter-toolbar">
    <form method="get" action="{{path_for "routes"}}" data-controller="debounced-form" data-action="input->debounced-form#queue submit->debounced-form#submit" data-debounced-form-count-source-value="[data-routes-frame-count]" data-debounced-form-count-target-value="[data-routes-count]" data-debounced-form-delay-value="250" data-debounced-form-frame-value="routes_table" data-turbo-frame="routes_table">
      <input class="filter-input" type="search" name="q" value="{{.routes_query}}" placeholder="Filter routes">
    </form>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-routes-count>{{.routes_count_text}}</span>
  </div>

  <turbo-frame id="routes_table" class="routes-table-frame">
    {{partial "routes_table" .}}
  </turbo-frame>
</section>
