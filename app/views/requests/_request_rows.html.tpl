{{range .requests.Rows}}
  <tr aria-selected="{{.Selected}}">
    <td><span class="request-status-dot" data-status-class="{{.Trace.StatusClass}}"></span></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.PathText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.MethodText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.StatusText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.DomainText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.TypeText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.InitiatorText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.SizeText}}</a></td>
    <td><a href="{{.URL}}" data-turbo-frame="request_detail">{{.Trace.DurationText}}</a></td>
  </tr>
{{end}}
