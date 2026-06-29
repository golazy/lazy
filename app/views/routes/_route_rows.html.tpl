{{range .routes}}
  <tr>
    <td><code>{{.Method}}</code></td>
    <td><code>{{.Path}}</code></td>
    <td>{{.Name}}</td>
    <td>{{.Target}}</td>
    <td>{{.Params}}</td>
    <td>{{.Namespace}}</td>
  </tr>
{{end}}
