package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazymedia"
)

type MediaController struct {
	panel.Base
}

var appControlMediaClient = &http.Client{Timeout: 30 * time.Second}

func New(ctx context.Context) (*MediaController, error) {
	base, err := panel.NewBase(ctx)
	return &MediaController{Base: base}, err
}

func (c *MediaController) Index(_ http.ResponseWriter, r *http.Request) error {
	c.Set("state", c.Snapshot())
	c.Set("media", c.mediaView(r))
	return nil
}

func (c *MediaController) Download(w http.ResponseWriter, r *http.Request) error {
	return c.proxyAppControl(w, r, http.MethodGet, lazymedia.LazyDevMediaDownloadPath+"?"+r.URL.RawQuery)
}

func (c *MediaController) StorageUpload(w http.ResponseWriter, r *http.Request) error {
	return c.postAndRedirect(w, r, lazymedia.LazyDevMediaStorageUploadPath)
}

func (c *MediaController) StorageDelete(w http.ResponseWriter, r *http.Request) error {
	return c.postAndRedirect(w, r, lazymedia.LazyDevMediaStorageDeletePath)
}

func (c *MediaController) FileUpload(w http.ResponseWriter, r *http.Request) error {
	return c.postAndRedirect(w, r, lazymedia.LazyDevMediaFileUploadPath)
}

func (c *MediaController) FileDelete(w http.ResponseWriter, r *http.Request) error {
	return c.postAndRedirect(w, r, lazymedia.LazyDevMediaFileDeletePath)
}

func (c *MediaController) VariantDelete(w http.ResponseWriter, r *http.Request) error {
	return c.postAndRedirect(w, r, lazymedia.LazyDevMediaVariantDeletePath)
}

func (c *MediaController) mediaView(r *http.Request) mediaView {
	var snapshot lazymedia.LazyDevSnapshot
	path := lazymedia.LazyDevMediaPath
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	if err := c.FetchAppControlJSON(r.Context(), path, &snapshot); err != nil {
		snapshot.FilesError = err.Error()
	}
	return newMediaView(r, snapshot)
}

func (c *MediaController) postAndRedirect(w http.ResponseWriter, r *http.Request, appPath string) error {
	if err := c.proxyAppControlDiscard(r, appPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	http.Redirect(w, r, safeMediaRedirect(r.URL.Query().Get("redirect")), http.StatusSeeOther)
	return nil
}

func (c *MediaController) proxyAppControlDiscard(r *http.Request, appPath string) error {
	addr := c.Snapshot().ControlPlaneAddr
	if addr == "" {
		return fmt.Errorf("application control plane is not available")
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "http://"+addr+appPath, r.Body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.ContentLength = r.ContentLength

	response, err := appControlMediaClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = response.Status
		}
		return fmt.Errorf("application control plane returned %s: %s", response.Status, detail)
	}
	return nil
}

func (c *MediaController) proxyAppControl(w http.ResponseWriter, r *http.Request, method string, appPath string) error {
	addr := c.Snapshot().ControlPlaneAddr
	if addr == "" {
		http.Error(w, "application control plane is not available", http.StatusBadGateway)
		return nil
	}
	request, err := http.NewRequestWithContext(r.Context(), method, "http://"+addr+appPath, nil)
	if err != nil {
		return err
	}
	response, err := appControlMediaClient.Do(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	defer response.Body.Close()
	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, err = io.Copy(w, response.Body)
	return err
}

func safeMediaRedirect(target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "/_golazy/media") &&
		!strings.HasPrefix(target, "//") &&
		!strings.ContainsAny(target, "\r\n") {
		return target
	}
	return "/_golazy/media"
}

type mediaView struct {
	Snapshot            lazymedia.LazyDevSnapshot
	Query               string
	SelectedFileID      string
	SelectedStorage     string
	SelectedStorageKey  string
	Prefix              string
	CurrentURL          string
	FileUploadAction    string
	StorageUploadAction string
	Files               []mediaFileRow
	Objects             []mediaObjectRow
	Storages            []mediaStorageRow
	SelectedFile        mediaFileRow
	SelectedObject      mediaObjectRow
}

func newMediaView(r *http.Request, snapshot lazymedia.LazyDevSnapshot) mediaView {
	view := mediaView{
		Snapshot:           snapshot,
		Query:              strings.TrimSpace(r.URL.Query().Get("q")),
		SelectedFileID:     strings.TrimSpace(r.URL.Query().Get("file")),
		SelectedStorage:    strings.TrimSpace(r.URL.Query().Get("storage")),
		SelectedStorageKey: strings.Trim(strings.TrimSpace(r.URL.Query().Get("key")), "/"),
		Prefix:             strings.Trim(strings.TrimSpace(r.URL.Query().Get("prefix")), "/"),
		CurrentURL:         r.URL.RequestURI(),
	}
	if view.SelectedStorage == "" {
		view.SelectedStorage = snapshot.SelectedStorage
	}
	view.FileUploadAction = mediaActionURL("/_golazy/media/file/upload", view.CurrentURL)
	view.StorageUploadAction = mediaActionURL("/_golazy/media/storage/upload", view.CurrentURL)
	view.Storages = mediaStorageRows(snapshot.Storages, view)
	view.Files = filterMediaFiles(mediaFileRows(snapshot.Files, view), view.Query)
	view.Objects = filterMediaObjects(mediaObjectRows(snapshot.StorageObjects, view), view.Query)
	for _, file := range view.Files {
		if file.ID == view.SelectedFileID {
			view.SelectedFile = file
			break
		}
	}
	for _, object := range view.Objects {
		if object.Key == view.SelectedStorageKey {
			view.SelectedObject = object
			break
		}
	}
	return view
}

func (v mediaView) CountText() string {
	if v.Snapshot.FilesError != "" {
		return "Files unavailable"
	}
	total := len(v.Snapshot.Files)
	visible := len(v.Files)
	if v.Query != "" {
		return strconv.Itoa(visible) + " / " + strconv.Itoa(total) + " files"
	}
	return strconv.Itoa(total) + " files"
}

func (v mediaView) ObjectCountText() string {
	if v.Snapshot.StorageObjectsError != "" {
		return "Storage unavailable"
	}
	total := len(v.Snapshot.StorageObjects)
	visible := len(v.Objects)
	suffix := " objects"
	if v.Snapshot.StorageObjectsTruncated {
		suffix = "+ objects"
	}
	if v.Query != "" {
		return strconv.Itoa(visible) + " / " + strconv.Itoa(total) + suffix
	}
	return strconv.Itoa(total) + suffix
}

func (v mediaView) HasSelectedFile() bool {
	return v.SelectedFile.ID != ""
}

func (v mediaView) HasSelectedObject() bool {
	return v.SelectedObject.Key != ""
}

func (v mediaView) HasFiles() bool {
	return len(v.Files) > 0
}

func (v mediaView) HasStorages() bool {
	return len(v.Storages) > 0
}

func (v mediaView) SelectedStorageLabel() string {
	if v.SelectedStorage == "" {
		return "No storage selected"
	}
	return v.SelectedStorage
}

type mediaStorageRow struct {
	Name         string
	URL          string
	Selected     bool
	Default      bool
	Readable     bool
	Writable     bool
	Deletable    bool
	Listable     bool
	URLable      bool
	Capabilities string
}

func mediaStorageRows(storages []lazymedia.LazyDevStorageSnapshot, view mediaView) []mediaStorageRow {
	rows := make([]mediaStorageRow, 0, len(storages))
	for _, storage := range storages {
		capabilities := []string{}
		if storage.Listable {
			capabilities = append(capabilities, "list")
		}
		if storage.Writable {
			capabilities = append(capabilities, "upload")
		}
		if storage.Deletable {
			capabilities = append(capabilities, "remove")
		}
		if storage.URLable {
			capabilities = append(capabilities, "url")
		}
		rows = append(rows, mediaStorageRow{
			Name:         storage.Name,
			URL:          view.mediaURL(map[string]string{"storage": storage.Name, "key": ""}),
			Selected:     storage.Name == view.SelectedStorage,
			Default:      storage.Default,
			Readable:     storage.Readable,
			Writable:     storage.Writable,
			Deletable:    storage.Deletable,
			Listable:     storage.Listable,
			URLable:      storage.URLable,
			Capabilities: strings.Join(capabilities, ", "),
		})
	}
	return rows
}

type mediaFileRow struct {
	ID           string
	Filename     string
	ContentType  string
	Size         string
	Storage      string
	Key          string
	Status       string
	URL          string
	PublicURL    string
	DownloadURL  string
	DeleteAction string
	Selected     bool
	Locations    []mediaLocationRow
	Variants     []mediaVariantRow
}

type mediaLocationRow struct {
	Storage string
	Key     string
	Role    string
	Status  string
}

type mediaVariantRow struct {
	SourceFileID   string
	VariantKey     string
	OutputFileID   string
	OutputURL      string
	OutputURLError string
	Status         string
	Error          string
	DeleteAction   string
	Badge          string
}

func mediaFileRows(files []lazymedia.LazyDevFileSnapshot, view mediaView) []mediaFileRow {
	rows := make([]mediaFileRow, 0, len(files))
	for _, file := range files {
		row := mediaFileRow{
			ID:           file.ID,
			Filename:     fallback(file.Filename, file.ID),
			ContentType:  file.ContentType,
			Size:         formatMediaBytes(file.Size),
			Storage:      file.Location.Storage,
			Key:          file.Location.Key,
			Status:       mediaFileStatus(file),
			URL:          view.mediaURL(map[string]string{"file": file.ID}),
			PublicURL:    file.URL,
			DownloadURL:  "/_golazy/media/download?file=" + url.QueryEscape(file.ID),
			DeleteAction: mediaActionURL("/_golazy/media/file/delete", view.CurrentURL),
			Selected:     file.ID == view.SelectedFileID,
		}
		for _, location := range file.Locations {
			row.Locations = append(row.Locations, mediaLocationRow{
				Storage: location.Storage,
				Key:     location.Key,
				Role:    location.Role,
				Status:  location.Status,
			})
		}
		for _, variant := range file.Variants {
			row.Variants = append(row.Variants, mediaVariantRow{
				SourceFileID:   variant.SourceFileID,
				VariantKey:     variant.VariantKey,
				OutputFileID:   variant.OutputFileID,
				OutputURL:      variant.OutputURL,
				OutputURLError: variant.OutputURLError,
				Status:         variant.Status,
				Error:          variant.Error,
				DeleteAction:   mediaActionURL("/_golazy/media/variant/delete", view.CurrentURL),
				Badge:          mediaVariantBadge(variant.VariantKey),
			})
		}
		rows = append(rows, row)
	}
	return rows
}

func mediaFileStatus(file lazymedia.LazyDevFileSnapshot) string {
	if !file.DeletedAt.IsZero() {
		return "deleted"
	}
	if file.Location.Storage != "" {
		return "active"
	}
	return "cataloged"
}

func filterMediaFiles(files []mediaFileRow, query string) []mediaFileRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return files
	}
	filtered := make([]mediaFileRow, 0, len(files))
	for _, file := range files {
		var parts []string
		parts = append(parts, file.ID, file.Filename, file.ContentType, file.Storage, file.Key, file.Status)
		for _, variant := range file.Variants {
			parts = append(parts, variant.VariantKey, variant.OutputFileID, variant.Status)
		}
		if strings.Contains(strings.ToLower(strings.Join(parts, " ")), query) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

type mediaObjectRow struct {
	Key          string
	ContentType  string
	Size         string
	Checksum     string
	Modified     string
	SelectURL    string
	PublicURL    string
	DownloadURL  string
	DeleteAction string
	Selected     bool
}

func mediaObjectRows(objects []lazymedia.LazyDevStorageObject, view mediaView) []mediaObjectRow {
	rows := make([]mediaObjectRow, 0, len(objects))
	for _, object := range objects {
		rows = append(rows, mediaObjectRow{
			Key:          object.Key,
			ContentType:  object.ContentType,
			Size:         formatMediaBytes(object.Size),
			Checksum:     object.Checksum,
			Modified:     formatMediaTime(object.ModifiedAt),
			SelectURL:    view.mediaURL(map[string]string{"storage": view.SelectedStorage, "key": object.Key}),
			PublicURL:    object.URL,
			DownloadURL:  "/_golazy/media/download?storage=" + url.QueryEscape(view.SelectedStorage) + "&key=" + url.QueryEscape(object.Key),
			DeleteAction: mediaActionURL("/_golazy/media/storage/delete", view.CurrentURL),
			Selected:     object.Key == view.SelectedStorageKey,
		})
	}
	return rows
}

func filterMediaObjects(objects []mediaObjectRow, query string) []mediaObjectRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return objects
	}
	filtered := make([]mediaObjectRow, 0, len(objects))
	for _, object := range objects {
		if strings.Contains(strings.ToLower(strings.Join([]string{
			object.Key,
			object.ContentType,
			object.Checksum,
		}, " ")), query) {
			filtered = append(filtered, object)
		}
	}
	return filtered
}

func (v mediaView) mediaURL(overrides map[string]string) string {
	values := url.Values{}
	if v.Query != "" {
		values.Set("q", v.Query)
	}
	if v.SelectedFileID != "" {
		values.Set("file", v.SelectedFileID)
	}
	if v.SelectedStorage != "" {
		values.Set("storage", v.SelectedStorage)
	}
	if v.SelectedStorageKey != "" {
		values.Set("key", v.SelectedStorageKey)
	}
	if v.Prefix != "" {
		values.Set("prefix", v.Prefix)
	}
	for key, value := range overrides {
		if value == "" {
			values.Del(key)
			continue
		}
		values.Set(key, value)
	}
	if encoded := values.Encode(); encoded != "" {
		return "/_golazy/media?" + encoded
	}
	return "/_golazy/media"
}

func mediaActionURL(path string, redirect string) string {
	return path + "?redirect=" + url.QueryEscape(safeMediaRedirect(redirect))
}

func mediaVariantBadge(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "var"
	}
	if len(key) <= 4 {
		return key
	}
	return key[:4]
}

func formatMediaBytes(size int64) string {
	if size < 0 {
		return "-"
	}
	units := []string{"B", "KiB", "MiB", "GiB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", size, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func formatMediaTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("15:04:05")
}

func fallback(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
