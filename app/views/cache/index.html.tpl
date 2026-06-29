{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "cache"}}"></turbo-stream-source>
  {{ partial "cache_frame" . }}
</div>
