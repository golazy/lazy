{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "dependencies"}}"></turbo-stream-source>
  {{ partial "dependencies_frame" . }}
</div>
