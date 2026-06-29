{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{.services_stream_url}}"></turbo-stream-source>
  {{ partial "services_frame" . }}
</div>
