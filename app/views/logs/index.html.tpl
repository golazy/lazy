{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "logs"}}"></turbo-stream-source>
  {{ partial "logs_frame" . }}
</div>
