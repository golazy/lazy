{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{.requests.StreamURL}}" data-request-stream-source></turbo-stream-source>
  {{ partial "requests_frame" . }}
</div>
