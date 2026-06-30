package docs

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/docsservice"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazydoc"
	"golazy.dev/lazyturbo"
)

type DocsController struct {
	panel.Base
	Docs *docsservice.Manager
}

func New(ctx context.Context) (*DocsController, error) {
	base, err := panel.NewBase(ctx)
	if err != nil {
		return nil, err
	}
	manager, ok := docsservice.ManagerFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("dev panel docs manager is missing")
	}
	return &DocsController{Base: base, Docs: manager}, nil
}

func (c *DocsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setDocsState(r)
			return nil
		},
		lazycontroller.TurboFrame: func() error {
			return c.renderDocsFrame(w, r)
		},
	})
}

func (c *DocsController) setDocsState(r *http.Request) {
	c.Set("state", c.Snapshot())
	c.Set("docs", c.docsView(r))
}

func (c *DocsController) renderDocsFrame(w http.ResponseWriter, r *http.Request) error {
	if frameID := lazyturbo.FrameID(r); frameID != "docs" {
		return lazycontroller.Error(http.StatusBadRequest, fmt.Errorf("docs frame %q is not available", frameID))
	}
	body, err := c.RenderPanelFrame(r, "docs", "docs", "docs_frame", map[string]any{
		"state": c.Snapshot(),
		"docs":  c.docsView(r),
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write([]byte(body))
	return err
}

func (c *DocsController) docsView(r *http.Request) docsView {
	snapshot := c.Snapshot()
	view := docsView{
		BaseURL:        "/_golazy/docs",
		Root:           snapshot.WatchedRoot,
		Query:          strings.TrimSpace(r.URL.Query().Get("q")),
		SelectedModule: strings.TrimSpace(r.URL.Query().Get("module")),
	}
	index, module, err := c.Docs.ModuleProject(r.Context(), snapshot.WatchedRoot, view.SelectedModule)
	if err != nil {
		view.Error = err.Error()
		return view
	}
	if view.SelectedModule == "" {
		view.SelectedModule = module.Path
	}
	version, ok := index.Latest()
	if !ok {
		view.Error = "project docs are empty"
		return view
	}
	view.Module = version.Module
	modules, err := c.Docs.Modules(r.Context(), snapshot.WatchedRoot)
	if err != nil {
		view.ModulesError = err.Error()
		modules = []docsservice.Module{module}
	}
	view.Modules = moduleRows(modules, view.SelectedModule, view.BaseURL)
	selected := selectPackage(version, r.URL.Query().Get("pkg"))
	if selected != nil {
		view.Selected = selected
		view.SelectedSlug = selected.Slug()
	}
	view.Packages = packageRows(version, selected, view.BaseURL, view.SelectedModule)
	view.CountText = strconv.Itoa(len(version.Packages)) + " packages"
	view.Search = searchRows(index.Search("local", view.Query), view.BaseURL, view.SelectedModule)

	sourceFile := strings.TrimSpace(r.URL.Query().Get("source"))
	if sourceFile != "" {
		line, _ := strconv.Atoi(r.URL.Query().Get("line"))
		source, err := c.Docs.Source(r.Context(), snapshot.WatchedRoot, view.SelectedModule, sourceFile, line)
		if err != nil {
			view.SourceError = err.Error()
		} else {
			view.Source = source
		}
	}
	return view
}

type docsView struct {
	BaseURL        string
	Root           string
	Module         string
	SelectedModule string
	Modules        []moduleRow
	ModulesError   string
	Query          string
	Error          string
	Selected       *lazydoc.Package
	SelectedSlug   string
	Packages       []packageRow
	Search         []searchRow
	Source         *docsservice.SourceFile
	SourceError    string
	CountText      string
}

func (v docsView) PackageURL(slug string) string {
	values := url.Values{}
	if v.SelectedModule != "" {
		values.Set("module", v.SelectedModule)
	}
	if slug != "" {
		values.Set("pkg", slug)
	}
	return v.url(values, "")
}

func (v docsView) CurrentPackageURL() string {
	return v.PackageURL(v.SelectedSlug)
}

func (v docsView) SourceURL(source *lazydoc.Source) string {
	if source == nil || source.File == "" {
		return ""
	}
	values := url.Values{}
	if v.SelectedModule != "" {
		values.Set("module", v.SelectedModule)
	}
	if v.SelectedSlug != "" {
		values.Set("pkg", v.SelectedSlug)
	}
	if v.Query != "" {
		values.Set("q", v.Query)
	}
	values.Set("source", source.File)
	if source.Line > 0 {
		values.Set("line", strconv.Itoa(source.Line))
	}
	return v.url(values, "")
}

func (v docsView) SymbolURL(symbol string) string {
	values := url.Values{}
	if v.SelectedModule != "" {
		values.Set("module", v.SelectedModule)
	}
	if v.SelectedSlug != "" {
		values.Set("pkg", v.SelectedSlug)
	}
	if v.Query != "" {
		values.Set("q", v.Query)
	}
	return v.url(values, symbol)
}

func (v docsView) SearchURL() string {
	values := url.Values{}
	if v.SelectedModule != "" {
		values.Set("module", v.SelectedModule)
	}
	if v.SelectedSlug != "" {
		values.Set("pkg", v.SelectedSlug)
	}
	return v.url(values, "")
}

func (v docsView) HasSearch() bool {
	return v.Query != ""
}

func (v docsView) url(values url.Values, fragment string) string {
	out := v.BaseURL
	if encoded := values.Encode(); encoded != "" {
		out += "?" + encoded
	}
	if fragment != "" {
		out += "#" + url.QueryEscape(fragment)
	}
	return out
}

type packageRow struct {
	Label      string
	ImportPath string
	Synopsis   string
	URL        string
	Current    bool
}

type moduleRow struct {
	Path    string
	Version string
	Label   string
	URL     string
	Current bool
	Main    bool
}

type searchRow struct {
	Kind    string
	Name    string
	Path    string
	Summary string
	URL     string
}

func selectPackage(version *lazydoc.Version, value string) *lazydoc.Package {
	if version == nil {
		return nil
	}
	value = strings.TrimSpace(value)
	if value != "" {
		if pkg, ok := version.Package(value); ok {
			return pkg
		}
	}
	if len(version.Packages) == 0 {
		return nil
	}
	return &version.Packages[0]
}

func moduleRows(modules []docsservice.Module, selected string, baseURL string) []moduleRow {
	rows := make([]moduleRow, 0, len(modules))
	for _, module := range modules {
		if module.Path == "" {
			continue
		}
		values := url.Values{}
		values.Set("module", module.Path)
		label := module.Path
		if module.Main {
			label = "Project"
		}
		rows = append(rows, moduleRow{
			Path:    module.Path,
			Version: module.Version,
			Label:   label,
			URL:     baseURL + "?" + values.Encode(),
			Current: module.Path == selected,
			Main:    module.Main,
		})
	}
	return rows
}

func packageRows(version *lazydoc.Version, selected *lazydoc.Package, baseURL string, modulePath string) []packageRow {
	if version == nil {
		return nil
	}
	rows := make([]packageRow, 0, len(version.Packages))
	for index := range version.Packages {
		pkg := &version.Packages[index]
		slug := pkg.Slug()
		values := url.Values{}
		if modulePath != "" {
			values.Set("module", modulePath)
		}
		values.Set("pkg", slug)
		rows = append(rows, packageRow{
			Label:      packageLabel(version.Module, pkg.ImportPath),
			ImportPath: pkg.ImportPath,
			Synopsis:   firstNonEmpty(pkg.Synopsis, pkg.Doc),
			URL:        baseURL + "?" + values.Encode(),
			Current:    selected != nil && selected.ImportPath == pkg.ImportPath,
		})
	}
	return rows
}

func searchRows(results []lazydoc.SearchResult, baseURL string, modulePath string) []searchRow {
	rows := make([]searchRow, 0, len(results))
	for _, result := range results {
		values := url.Values{}
		if modulePath != "" {
			values.Set("module", modulePath)
		}
		values.Set("pkg", packageSlug(result.PackagePath))
		fragment := result.Name
		if result.Kind == "package" {
			fragment = ""
		}
		link := baseURL + "?" + values.Encode()
		if fragment != "" {
			link += "#" + url.QueryEscape(fragment)
		}
		rows = append(rows, searchRow{
			Kind:    result.Kind,
			Name:    result.Name,
			Path:    result.PackagePath,
			Summary: result.Summary,
			URL:     link,
		})
	}
	return rows
}

func packageLabel(module string, importPath string) string {
	label := strings.TrimPrefix(importPath, strings.TrimSuffix(module, "/")+"/")
	if label == importPath {
		return importPath
	}
	if label == "" {
		return "."
	}
	return label
}

func packageSlug(importPath string) string {
	slug := strings.TrimPrefix(importPath, "golazy.dev/")
	if slug == "golazy.dev" {
		return "golazy"
	}
	return strings.ReplaceAll(slug, "/", ".")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
