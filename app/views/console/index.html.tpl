{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "console"}}"></turbo-stream-source>
  {{ partial "console_frame" . }}
</div>
