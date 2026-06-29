{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "actions"}}"></turbo-stream-source>
  {{ partial "actions_frame" . }}
</div>
