{{ partial "nav" . }}
<div class="panel-page-body">
  <turbo-stream-source src="{{path_for "services"}}?service={{.selected_service}}"></turbo-stream-source>
  {{ partial "services_frame" . }}
</div>
