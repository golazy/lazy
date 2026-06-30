{{range .workers}}
  <tr>
    <td><code>{{.Name}}</code></td>
    <td>{{.Kind}}</td>
    <td><code>{{.Path}}</code></td>
    <td>{{if .Scope}}<code>{{.Scope}}</code>{{else}}-{{end}}</td>
    <td>{{.Type}}</td>
    <td>{{.Source}}</td>
    <td>{{.Owner}}</td>
    <td>{{if .Description}}{{.Description}}{{else}}-{{end}}</td>
  </tr>
{{else}}
  <tr>
    <td colspan="8" class="empty-cell">{{if .workers_error}}{{.workers_error}}{{else}}No workers found.{{end}}</td>
  </tr>
{{end}}
