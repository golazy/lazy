<nav class="panel-tabs tabbed-pane-header" aria-label="GoLazy panel sections" data-controller="panel-close">
  <div class="panel-tab-list">
    <span class="panel-tab">{{link_to "App" (path_for "app") (data "turbo-frame" "_top") (unless_current)}}</span>
    <span class="panel-tab">{{link_to "Requests" (path_for "requests") (data "turbo-frame" "_top") (unless_current)}}</span>
    <span class="panel-tab">{{link_to "Services" (path_for "services") (data "turbo-frame" "_top") (unless_current)}}</span>
    <span class="panel-tab">{{link_to "Routes" (path_for "routes") (data "turbo-frame" "_top") (unless_current)}}</span>
    <span class="panel-tab">{{link_to "Jobs" (path_for "jobs") (data "turbo-frame" "_top") (unless_current)}}</span>
    <span class="panel-tab">{{link_to "BuildInfo" (path_for "buildinfo") (data "turbo-frame" "_top") (unless_current)}}</span>
    <span class="panel-tab">{{link_to "Assets" (path_for "assets") (data "turbo-frame" "_top") (unless_current)}}</span>
    <span class="panel-tab">{{link_to "Cache" (path_for "cache") (data "turbo-frame" "_top") (unless_current)}}</span>
  </div>
  <button type="button" class="panel-close-button" data-panel-close hidden aria-label="Close GoLazy development panel" title="Close GoLazy development panel" data-panel-close-target="button" data-action="panel-close#close"></button>
</nav>
