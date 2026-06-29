{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "app"}}"></turbo-stream-source>
  {{ partial "app_frame" . }}
</div>
