package appinit

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"golazy.dev/lazy/app"

	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazy/services/docsservice"
	"golazy.dev/lazy/services/terminalservice"
	"golazy.dev/lazyapp"
	"golazy.dev/lazyassets"
	"golazy.dev/lazydeps"
	"golazy.dev/lazytui/encoding/tty"
	"golazy.dev/lazywebsocket"

	_ "golazy.dev/lazyview/gotmpl"
)

const terminalConnectPath = "/_golazy/terminal/connect"

type Config struct {
	Store             *buildservice.Store
	Actions           buildservice.Actions
	Docs              *docsservice.Manager
	Terminal          *terminalservice.Manager
	ForceDetailErrors bool
}

func App(config Config) *lazyapp.App {
	return newApp(normalizeConfig(config))
}

func Handler(config Config) http.Handler {
	config = normalizeConfig(config)
	return terminalConnectHandler{
		next:     newApp(config),
		store:    config.Store,
		terminal: config.Terminal,
	}
}

func normalizeConfig(config Config) Config {
	if config.Store == nil {
		config.Store = buildservice.NewStore(200)
	}
	if config.Actions == nil {
		config.Actions = buildservice.NewActions()
	}
	if config.Docs == nil {
		config.Docs = docsservice.NewManager()
	}
	if config.Terminal == nil {
		config.Terminal = terminalservice.NewManager()
	}
	return config
}

func newApp(config Config) *lazyapp.App {
	return lazyapp.New(lazyapp.Config{
		Name:              "golazy.dev/lazy/dev-panel",
		Drawer:            Draw,
		Public:            app.Public,
		Views:             app.Views,
		Dependencies:      dependencies(config),
		ForceDetailErrors: config.ForceDetailErrors,
		AssetOptions: []lazyassets.Option{
			lazyassets.WithURLPrefix("/_golazy"),
			lazyassets.WithDevelopmentMode(true),
		},
	})
}

type terminalConnectHandler struct {
	next     http.Handler
	store    *buildservice.Store
	terminal *terminalservice.Manager
}

func (h terminalConnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != terminalConnectPath {
		h.next.ServeHTTP(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !terminalWebSocketRequest(r) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if h.store == nil || h.terminal == nil {
		http.NotFound(w, r)
		return
	}
	session, err := h.terminal.Session(r.Context(), terminalservice.Options{
		Dir:  h.store.Snapshot().WatchedRoot,
		Size: terminalSize(r),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	_ = session.Attach(w, r)
}

func terminalSize(r *http.Request) tty.Size {
	return tty.Size{
		Rows: queryPositiveInt(r, "rows", 24),
		Cols: queryPositiveInt(r, "cols", 80),
	}
}

func terminalWebSocketRequest(r *http.Request) bool {
	return lazywebsocket.IsWebSocketUpgrade(r) &&
		r.Header.Get("Sec-Websocket-Version") == "13" &&
		strings.TrimSpace(r.Header.Get("Sec-Websocket-Key")) != ""
}

func queryPositiveInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func dependencies(config Config) func(*lazydeps.Scope) error {
	return func(scope *lazydeps.Scope) error {
		_, err := lazydeps.Service(scope, "dev-panel", func(ctx context.Context) (context.Context, struct{}, error, context.CancelFunc) {
			ctx = buildservice.WithStore(ctx, config.Store)
			ctx = buildservice.WithActions(ctx, config.Actions)
			ctx = docsservice.WithManager(ctx, config.Docs)
			ctx = terminalservice.WithManager(ctx, config.Terminal)
			return ctx, struct{}{}, nil, nil
		})
		return err
	}
}
