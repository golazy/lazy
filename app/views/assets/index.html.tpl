{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "assets"}}"></turbo-stream-source>
  {{ partial "assets_frame" . }}
</div>
