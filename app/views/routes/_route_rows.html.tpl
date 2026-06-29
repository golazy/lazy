{{range .routes}}
  <tr>
    <td><code>{{.Method}}</code></td>
    <td>
      {{if .Linkable}}
        <a href="{{.Link}}" target="_top" data-turbo-frame="_top"><code>{{.Path}}</code></a>
      {{else}}
        <code>{{.Path}}</code>
      {{end}}
    </td>
    <td>{{.Name}}</td>
    <td>{{.Target}}</td>
    <td>{{.Params}}</td>
    <td>{{.Namespace}}</td>
  </tr>
{{end}}
