{{range .traces.TraceRows}}
  <tr aria-selected="{{.Selected}}">
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.Method}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.Path}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.Status}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.DurationText}}</a></td>
  </tr>
{{end}}
