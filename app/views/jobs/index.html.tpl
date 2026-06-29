{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "jobs"}}"></turbo-stream-source>
  {{ partial "jobs_frame" . }}
</div>
