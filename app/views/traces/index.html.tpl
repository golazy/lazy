{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{.traces.StreamURL}}"></turbo-stream-source>
  {{ partial "traces_frame" . }}
</div>
