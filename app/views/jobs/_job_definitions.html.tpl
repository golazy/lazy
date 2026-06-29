{{range .jobs.Definitions}}
  <li><code>{{.Kind}}</code> {{.Queue}} attempts {{.MaxAttempts}}</li>
{{end}}
