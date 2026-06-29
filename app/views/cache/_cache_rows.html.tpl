{{range .cache.Entries}}
  <tr id="{{.ID}}" class="{{if eq .Key $.cache.SelectedKey}}is-selected{{end}}">
    <td><a href="{{$.cache.KeyURL .Key}}" data-turbo-frame="_top"><code>{{.Key}}</code></a></td>
    <td>{{.AgeText}}</td>
    <td>{{.SizeText}}</td>
    <td>{{.Hits}}</td>
    <td>{{.Sets}}</td>
  </tr>
{{else}}
  <tr data-cache-empty>
    <td colspan="5" class="muted">{{$.cache.EmptyText}}</td>
  </tr>
{{end}}
