{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{.pwa_stream}}"></turbo-stream-source>
  {{ partial "pwa_frame" . }}
</div>
