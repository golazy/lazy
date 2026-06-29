{{range .assets}}
  <tr>
    <td><code>{{.Path}}</code></td>
    <td>{{if .Permanent}}<code>{{.Permanent}}</code>{{else}}-{{end}}</td>
    <td>{{.ContentType}}</td>
    <td>{{.Size}}</td>
    <td>{{.Source}}</td>
    <td>{{.Kind}}</td>
    <td>{{.Status}}</td>
  </tr>
{{else}}
  <tr>
    <td colspan="7" class="empty-cell">{{if .assets_error}}{{.assets_error}}{{else}}No assets found.{{end}}</td>
  </tr>
{{end}}
