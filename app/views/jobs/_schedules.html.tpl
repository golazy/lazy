{{range .jobs.Schedules}}
  <li><code>{{.Key}}</code> {{.Kind}} on {{.Queue}} every {{.Interval}} next {{.NextRunAtText}}</li>
{{else}}
  <li>No schedules registered</li>
{{end}}
