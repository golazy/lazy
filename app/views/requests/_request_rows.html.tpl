{{range .requests.Rows}}
  <tr aria-selected="{{.Selected}}">
    <td><span class="request-status-dot" data-status-class="{{.Trace.StatusClass}}"></span></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.PathText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.MethodText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.StatusText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.DomainText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.TypeText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.InitiatorText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.SizeText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="_top">{{.Trace.DurationText}}</a></td>
  </tr>
{{end}}
