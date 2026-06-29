package assets

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazycontroller"
)

const appAssetsPath = "/assets"

type AssetsController struct {
	panel.Base
}

func New(ctx context.Context) (*AssetsController, error) {
	base, err := panel.NewBase(ctx)
	return &AssetsController{Base: base}, err
}

func (c *AssetsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setAssetsState(r)
			c.Set("defer_panel_lists", true)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboInitial(w, r, c.streamAssetsInitial)
		},
	})
}

func (c *AssetsController) setAssetsState(r *http.Request) {
	for key, value := range c.assetsViewData(r) {
		c.Set(key, value)
	}
}

func (c *AssetsController) streamAssetsInitial(r *http.Request) (string, error) {
	variables := c.assetsViewData(r)
	body, err := c.RenderPanelPartial(r, "assets", "asset_rows", variables)
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("update", "[data-assets-list]", body) +
		panel.TurboStreamTargets("update", "[data-assets-count]", assetCountText(variables)), nil
}

func (c *AssetsController) assetsViewData(r *http.Request) map[string]any {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	var manifest assetManifest
	if err := c.FetchAppControlJSON(r.Context(), appAssetsPath, &manifest); err != nil {
		return map[string]any{
			"state":          c.Snapshot(),
			"assets_error":   err.Error(),
			"assets_query":   query,
			"assets_stream":  assetStreamURL(query),
			"assets_total":   0,
			"assets_visible": 0,
			"assets":         []assetRow{},
		}
	}
	rows := assetRows(manifest.Assets)
	filtered := filterAssets(rows, query)
	return map[string]any{
		"state":          c.Snapshot(),
		"assets_query":   query,
		"assets_stream":  assetStreamURL(query),
		"assets_total":   len(rows),
		"assets_visible": len(filtered),
		"assets":         filtered,
	}
}

type assetManifest struct {
	Assets []assetRecord `json:"assets"`
}

type assetRecord struct {
	Path        string `json:"path"`
	Permanent   string `json:"permanent"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash"`
	ETag        string `json:"etag"`
	Integrity   string `json:"integrity"`
	Source      string `json:"source"`
	Generated   bool   `json:"generated"`
	Ignored     bool   `json:"ignored"`
}

type assetRow struct {
	Path        string
	Permanent   string
	ContentType string
	Size        string
	Source      string
	Kind        string
	Status      string
}

func assetRows(records []assetRecord) []assetRow {
	rows := make([]assetRow, 0, len(records))
	for _, record := range records {
		status := "public"
		if record.Ignored {
			status = "ignored"
		}
		kind := "Public"
		if record.Generated {
			kind = "Generated"
		}
		rows = append(rows, assetRow{
			Path:        record.Path,
			Permanent:   record.Permanent,
			ContentType: record.ContentType,
			Size:        formatAssetSize(record.Size),
			Source:      record.Source,
			Kind:        kind,
			Status:      status,
		})
	}
	return rows
}

func filterAssets(assets []assetRow, query string) []assetRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return assets
	}
	filtered := make([]assetRow, 0, len(assets))
	for _, asset := range assets {
		if strings.Contains(strings.ToLower(strings.Join([]string{
			asset.Path,
			asset.Permanent,
			asset.ContentType,
			asset.Source,
			asset.Kind,
			asset.Status,
		}, " ")), query) {
			filtered = append(filtered, asset)
		}
	}
	return filtered
}

func assetCountText(variables map[string]any) string {
	if variables["assets_error"] != nil {
		return "Assets unavailable"
	}
	total, _ := variables["assets_total"].(int)
	visible, _ := variables["assets_visible"].(int)
	query, _ := variables["assets_query"].(string)
	if query != "" {
		return strconv.Itoa(visible) + " / " + strconv.Itoa(total) + " assets"
	}
	return strconv.Itoa(total) + " assets"
}

func assetStreamURL(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "/_golazy/assets"
	}
	return "/_golazy/assets?q=" + url.QueryEscape(query)
}

func formatAssetSize(size int64) string {
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
