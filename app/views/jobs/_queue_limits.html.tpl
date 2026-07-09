{{range .jobs.QueueLimits}}
  <li><code>{{.Queue}}</code> {{.Running}}/{{.MaxRunning}} running</li>
{{else}}
  <li>No queue limits configured</li>
{{end}}
