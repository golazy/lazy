{{range .service_output_rows}}
  <tr>
    <td>{{.Task}}</td>
    <td>{{.RunLabel}}</td>
    <td>{{.Stream}}</td>
    <td>{{.Time}}</td>
    <td>{{.Message}}</td>
    <td>{{.Attrs}}</td>
  </tr>
{{end}}
