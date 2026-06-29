{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "routes"}}?q={{.routes_query}}"></turbo-stream-source>
  {{ partial "routes_frame" . }}
</div>
