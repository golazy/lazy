{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{.requests.StreamURL}}"></turbo-stream-source>
  {{ partial "requests_frame" . }}
</div>
