{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "requests"}}"></turbo-stream-source>
  {{ partial "requests_frame" . }}
</div>
