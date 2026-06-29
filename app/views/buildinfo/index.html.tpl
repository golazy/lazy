{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "buildinfo"}}"></turbo-stream-source>
  {{ partial "buildinfo_frame" . }}
</div>
