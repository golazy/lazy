<nav class="panel-tabs tabbed-pane-header" aria-label="GoLazy panel sections">
  <span class="panel-tab">{{link_to "Requests" (path_for "requests") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "Console" (path_for "console") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "App Logs" (path_for "logs") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "Services" (path_for "services") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "Traces" (path_for "traces") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "Routes" (path_for "routes") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "Jobs" (path_for "jobs") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "Assets" (path_for "assets") (data "turbo-frame" "_top") (unless_current)}}</span>
  <span class="panel-tab">{{link_to "Actions" (path_for "actions") (data "turbo-frame" "_top") (unless_current)}}</span>
</nav>
