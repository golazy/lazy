{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{.assets_stream}}"></turbo-stream-source>
  {{ partial "assets_frame" . }}
</div>
