<section id="assets" class="tool-view is-active" data-view="assets">
  <div class="filter-toolbar">
    <form method="get" action="{{path_for "assets"}}" class="inline-form" data-controller="debounced-form" data-action="input->debounced-form#queue submit->debounced-form#submit" data-debounced-form-delay-value="250" data-debounced-form-frame-value="assets_table" data-turbo-frame="assets_table">
      <input class="filter-input" type="search" name="q" placeholder="Filter assets" value="{{.assets_query}}">
    </form>
    <span class="toolbar-spacer"></span>
    <span class="toolbar-count" data-assets-count>{{.assets_count_text}}</span>
  </div>
  <turbo-frame id="assets_table" class="assets-table-frame">
    {{partial "assets_table" .}}
  </turbo-frame>
</section>
