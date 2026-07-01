<section id="media" class="tool-view is-active media-view" data-view="media">
  <div class="filter-toolbar media-toolbar">
    <form method="get" action="{{path_for "media"}}" class="inline-form">
      {{if .media.SelectedFileID}}<input type="hidden" name="file" value="{{.media.SelectedFileID}}">{{end}}
      {{if .media.SelectedStorage}}<input type="hidden" name="storage" value="{{.media.SelectedStorage}}">{{end}}
      {{if .media.SelectedStorageKey}}<input type="hidden" name="key" value="{{.media.SelectedStorageKey}}">{{end}}
      <input class="filter-input" type="search" name="q" placeholder="Filter media" value="{{.media.Query}}">
    </form>
    <span class="toolbar-count">{{.media.CountText}}</span>
    <span class="toolbar-divider"></span>
    {{if .media.HasStorages}}
      <form method="post" action="{{.media.FileUploadAction}}" enctype="multipart/form-data" class="media-upload-form">
        <input class="media-file-input" type="file" name="file" required>
        <select name="storage" class="media-select" aria-label="File storage">
          {{range .media.Storages}}
            <option value="{{.Name}}" {{if .Selected}}selected{{end}}>{{.Name}}</option>
          {{end}}
        </select>
        <input class="filter-input media-key-input" type="text" name="key" placeholder="object key">
        <button type="submit" class="toolbar-button">Upload file</button>
      </form>
    {{else}}
      <span class="toolbar-count">No storage configured</span>
    {{end}}
  </div>

  <div class="media-layout">
    <aside class="media-storage-pane" aria-label="Storages">
      <div class="media-pane-header">
        <strong>Storages</strong>
        <span>{{.media.ObjectCountText}}</span>
      </div>
      <div class="media-storage-list">
        {{range .media.Storages}}
          <a class="media-storage-row {{if .Selected}}is-selected{{end}}" href="{{.URL}}" data-turbo-frame="_top">
            <span>{{.Name}}</span>
            <small>{{.Capabilities}}</small>
          </a>
        {{else}}
          <div class="empty-state">No storages registered.</div>
        {{end}}
      </div>

      {{if .media.HasStorages}}
        <form method="post" action="{{.media.StorageUploadAction}}" enctype="multipart/form-data" class="media-storage-upload">
          <input type="hidden" name="storage" value="{{.media.SelectedStorage}}">
          <input class="media-file-input" type="file" name="file" required>
          <input class="filter-input" type="text" name="key" placeholder="object key">
          <button type="submit" class="toolbar-button">Upload object</button>
        </form>
      {{end}}

      {{if .media.Snapshot.StorageObjectsError}}
        <div class="empty-state">{{.media.Snapshot.StorageObjectsError}}</div>
      {{else}}
        <table class="data-grid media-object-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>Key</th>
              <th>Size</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {{range .media.Objects}}
              <tr class="{{if .Selected}}is-selected{{end}}">
                <td><a href="{{.SelectURL}}" data-turbo-frame="_top"><code>{{.Key}}</code></a></td>
                <td>{{.Size}}</td>
                <td class="media-actions">
                  <a href="{{.DownloadURL}}">Download</a>
                  {{if .PublicURL}}<a href="{{.PublicURL}}" target="_blank" rel="noreferrer">URL</a>{{end}}
                  <form method="post" action="{{.DeleteAction}}">
                    <input type="hidden" name="storage" value="{{$.media.SelectedStorage}}">
                    <input type="hidden" name="key" value="{{.Key}}">
                    <button type="submit" class="link-button">Remove</button>
                  </form>
                </td>
              </tr>
            {{else}}
              <tr>
                <td colspan="3" class="empty-cell">No objects in {{.media.SelectedStorageLabel}}.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      {{end}}
    </aside>

    <section class="media-files-pane" aria-label="Files">
      {{if .media.Snapshot.FilesError}}
        <div class="empty-state">{{.media.Snapshot.FilesError}}</div>
      {{else}}
        <table class="data-grid media-file-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>File</th>
              <th>Variants</th>
              <th>Storage</th>
              <th>Key</th>
              <th>Type</th>
              <th>Size</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {{range .media.Files}}
              <tr class="{{if .Selected}}is-selected{{end}}">
                <td>
                  <a href="{{.URL}}" data-turbo-frame="_top">{{.Filename}}</a>
                  <code>{{.ID}}</code>
                </td>
                <td>
                  <span class="media-variant-badges">
                    {{range .Variants}}
                      <span class="media-variant-badge" title="{{.VariantKey}}">{{.Badge}}</span>
                    {{else}}
                      <span class="muted">-</span>
                    {{end}}
                  </span>
                </td>
                <td>{{.Storage}}</td>
                <td><code>{{.Key}}</code></td>
                <td>{{.ContentType}}</td>
                <td>{{.Size}}</td>
                <td class="media-actions">
                  <a href="{{.DownloadURL}}">Download</a>
                  {{if .PublicURL}}<a href="{{.PublicURL}}" target="_blank" rel="noreferrer">URL</a>{{end}}
                </td>
              </tr>
            {{else}}
              <tr>
                <td colspan="7" class="empty-cell">No files match this filter.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      {{end}}
    </section>

    <aside class="details-pane media-detail-pane" aria-label="Media detail">
      {{if .media.HasSelectedFile}}
        <div class="detail-header">
          <h2>{{.media.SelectedFile.Filename}}</h2>
          <span>{{.media.SelectedFile.Size}}</span>
        </div>
        <div class="media-detail-actions">
          <a class="toolbar-button" href="{{.media.SelectedFile.DownloadURL}}">Download</a>
          {{if .media.SelectedFile.PublicURL}}<a class="toolbar-button" href="{{.media.SelectedFile.PublicURL}}" target="_blank" rel="noreferrer">URL</a>{{end}}
          <form method="post" action="{{.media.SelectedFile.DeleteAction}}">
            <input type="hidden" name="file" value="{{.media.SelectedFile.ID}}">
            <button type="submit" class="toolbar-button">Remove</button>
          </form>
        </div>
        <dl class="detail-list">
          <dt>ID</dt>
          <dd><code>{{.media.SelectedFile.ID}}</code></dd>
          <dt>Type</dt>
          <dd>{{.media.SelectedFile.ContentType}}</dd>
          <dt>Storage</dt>
          <dd>{{.media.SelectedFile.Storage}}</dd>
          <dt>Key</dt>
          <dd><code>{{.media.SelectedFile.Key}}</code></dd>
          <dt>Status</dt>
          <dd>{{.media.SelectedFile.Status}}</dd>
        </dl>
        <div class="detail-header media-section-header">
          <h2>Variants</h2>
          <span>{{len .media.SelectedFile.Variants}}</span>
        </div>
        <table class="data-grid media-variant-grid" data-controller="table-resize">
          <thead>
            <tr>
              <th>Variant</th>
              <th>Output</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {{range .media.SelectedFile.Variants}}
              <tr>
                <td>{{.VariantKey}}</td>
                <td><code>{{.OutputFileID}}</code></td>
                <td>{{.Status}}</td>
                <td class="media-actions">
                  {{if .OutputURL}}<a href="{{.OutputURL}}" target="_blank" rel="noreferrer">URL</a>{{end}}
                  <form method="post" action="{{.DeleteAction}}">
                    <input type="hidden" name="source_file" value="{{.SourceFileID}}">
                    <input type="hidden" name="variant" value="{{.VariantKey}}">
                    <button type="submit" class="link-button">Remove</button>
                  </form>
                </td>
              </tr>
            {{else}}
              <tr>
                <td colspan="4" class="empty-cell">No variants recorded for this file.</td>
              </tr>
            {{end}}
          </tbody>
        </table>
      {{else if .media.HasSelectedObject}}
        <div class="detail-header">
          <h2>{{.media.SelectedObject.Key}}</h2>
          <span>{{.media.SelectedObject.Size}}</span>
        </div>
        <div class="media-detail-actions">
          <a class="toolbar-button" href="{{.media.SelectedObject.DownloadURL}}">Download</a>
          {{if .media.SelectedObject.PublicURL}}<a class="toolbar-button" href="{{.media.SelectedObject.PublicURL}}" target="_blank" rel="noreferrer">URL</a>{{end}}
          <form method="post" action="{{.media.SelectedObject.DeleteAction}}">
            <input type="hidden" name="storage" value="{{.media.SelectedStorage}}">
            <input type="hidden" name="key" value="{{.media.SelectedObject.Key}}">
            <button type="submit" class="toolbar-button">Remove</button>
          </form>
        </div>
        <dl class="detail-list">
          <dt>Storage</dt>
          <dd>{{.media.SelectedStorage}}</dd>
          <dt>Key</dt>
          <dd><code>{{.media.SelectedObject.Key}}</code></dd>
          <dt>Type</dt>
          <dd>{{.media.SelectedObject.ContentType}}</dd>
          <dt>Checksum</dt>
          <dd><code>{{.media.SelectedObject.Checksum}}</code></dd>
          <dt>Modified</dt>
          <dd>{{.media.SelectedObject.Modified}}</dd>
        </dl>
      {{else}}
        <div class="empty-state">Select a file or storage object to inspect it.</div>
      {{end}}
    </aside>
  </div>
</section>
