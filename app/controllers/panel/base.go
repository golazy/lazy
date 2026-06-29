package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazysse"
	"golazy.dev/lazyturbo"
	"golazy.dev/lazyview"
)

type Base struct {
	controllers.BaseController
	Store    *buildservice.Store
	Actions  buildservice.Actions
	Renderer *lazycontroller.Renderer
}

var appControlClient = &http.Client{Timeout: 2 * time.Second}

func NewBase(ctx context.Context) (Base, error) {
	base, err := controllers.NewBaseController(ctx)
	if err != nil {
		return Base{}, err
	}
	store, ok := buildservice.StoreFromContext(ctx)
	if !ok {
		return Base{}, fmt.Errorf("dev panel store is missing")
	}
	actions, ok := buildservice.ActionsFromContext(ctx)
	if !ok {
		return Base{}, fmt.Errorf("dev panel actions are missing")
	}
	renderer, ok := lazycontroller.RendererFromContext(ctx)
	if !ok {
		return Base{}, fmt.Errorf("dev panel renderer is missing")
	}
	return Base{BaseController: base, Store: store, Actions: actions, Renderer: renderer}, nil
}

func (b *Base) SetState() {
	b.Set("state", b.Snapshot())
}

func (b *Base) Snapshot() buildservice.Snapshot {
	if b == nil || b.Store == nil {
		return buildservice.Snapshot{}
	}
	return b.Store.Snapshot()
}

func (b *Base) FetchAppControlJSON(ctx context.Context, path string, target any) error {
	addr := b.Snapshot().ControlPlaneAddr
	if addr == "" {
		return fmt.Errorf("application control plane is not available")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")

	response, err := appControlClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("application control plane returned %s", response.Status)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return err
	}
	return nil
}

func (b *Base) PostAppControl(ctx context.Context, path string) error {
	addr := b.Snapshot().ControlPlaneAddr
	if addr == "" {
		return fmt.Errorf("application control plane is not available")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")

	response, err := appControlClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("application control plane returned %s", response.Status)
	}
	return nil
}

func (b *Base) StreamTurbo(w http.ResponseWriter, r *http.Request, render func(*http.Request, buildservice.Event) (string, error)) error {
	stream, err := b.SSEStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	events, unsubscribe := b.Store.Subscribe()
	defer unsubscribe()
	stream.Heartbeat(15 * time.Second)

	for {
		select {
		case <-stream.Done():
			return nil
		case event := <-events:
			if render == nil {
				continue
			}
			body, err := render(r, event)
			if err != nil || body == "" {
				continue
			}
			if err := stream.Send(lazysse.Event{Data: []string{body}}); err != nil {
				return err
			}
		}
	}
}

func (b *Base) RenderPanelFrame(r *http.Request, id string, controller string, partial string, variables map[string]any) (string, error) {
	body, err := b.RenderPanelPartial(r, controller, partial, variables)
	if err != nil {
		return "", err
	}
	frame, err := lazyturbo.FrameTag(id, body)
	if err != nil {
		return "", err
	}
	return frame.Body, nil
}

func (b *Base) RenderPermanentPanelFrame(r *http.Request, id string, controller string, partial string, variables map[string]any) (string, error) {
	body, err := b.RenderPanelPartial(r, controller, partial, variables)
	if err != nil {
		return "", err
	}
	if err := lazyturbo.ValidateFrameID(id); err != nil {
		return "", err
	}
	return `<turbo-frame id="` + html.EscapeString(id) + `" data-turbo-permanent>` + body + `</turbo-frame>`, nil
}

func (b *Base) RenderPanelPartial(r *http.Request, controller string, partial string, variables map[string]any) (string, error) {
	return b.renderPanelPartial(r, controller, partial, variables, nil)
}

func (b *Base) RenderPanelPartialData(r *http.Request, controller string, partial string, data any) (string, error) {
	return b.renderPanelPartial(r, controller, partial, nil, data)
}

func (b *Base) renderPanelPartial(r *http.Request, controller string, partial string, variables map[string]any, data any) (string, error) {
	if b == nil || b.Renderer == nil {
		return "", fmt.Errorf("dev panel renderer is missing")
	}
	return b.Renderer.RenderString(lazyview.Options{
		Context:    r.Context(),
		Request:    r,
		Variables:  variables,
		Data:       data,
		Controller: controller,
		Partial:    partial,
		Format:     string(lazycontroller.HTML),
		UseLayout:  false,
	})
}

func TurboStream(action string, target string, body string) string {
	var builder strings.Builder
	builder.WriteString(`<turbo-stream action="`)
	builder.WriteString(html.EscapeString(action))
	builder.WriteString(`" target="`)
	builder.WriteString(html.EscapeString(target))
	builder.WriteString(`"><template>`)
	builder.WriteString(body)
	builder.WriteString(`</template></turbo-stream>`)
	return builder.String()
}
