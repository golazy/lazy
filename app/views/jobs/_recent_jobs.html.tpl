{{range .jobs.Recent}}
  <tr>
    <td>{{.ID}}</td>
    <td>{{.Kind}}</td>
    <td>{{.Queue}}</td>
    <td>{{.State}}</td>
    <td>{{.AttemptText}}</td>
    <td>{{.RunAtText}}</td>
    <td>{{.LastError}}</td>
  </tr>
{{end}}
