{{range .service_task_filters}}
  <a href="{{.URL}}" data-turbo-frame="_top" aria-current="{{if .Selected}}page{{else}}false{{end}}">{{.Label}}</a>
{{end}}
