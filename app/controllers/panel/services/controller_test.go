package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
)

func TestRestartEnqueuesServiceAction(t *testing.T) {
	store := buildservice.NewStore(10)
	store.UpdateService("postgres", buildservice.ServiceReady, "ready")
	actions := buildservice.NewActions()
	received := make(chan buildservice.ActionRequest, 1)
	go func() {
		request := <-actions
		received <- request
		request.Reply <- nil
	}()

	controller := &ServicesController{Base: panel.Base{Store: store, Actions: actions}}
	request := httptest.NewRequest(http.MethodPost, "/_golazy/services/postgres/restart", nil)
	request.SetPathValue("service_id", "postgres")
	response := httptest.NewRecorder()
	if err := controller.Restart(response, request); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusSeeOther, response.Body.String())
	}
	if got, want := response.Header().Get("Location"), "/_golazy/services?service=postgres"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	action := <-received
	if action.Action != buildservice.ActionRestartService {
		t.Fatalf("action = %q, want %q", action.Action, buildservice.ActionRestartService)
	}
	if action.Service != "postgres" {
		t.Fatalf("service = %q, want postgres", action.Service)
	}
}

func TestRestartRejectsUnknownService(t *testing.T) {
	store := buildservice.NewStore(10)
	store.UpdateService("postgres", buildservice.ServiceReady, "ready")
	controller := &ServicesController{Base: panel.Base{Store: store, Actions: buildservice.NewActions()}}

	request := httptest.NewRequest(http.MethodPost, "/_golazy/services/minio/restart", nil)
	request.SetPathValue("service_id", "minio")
	response := httptest.NewRecorder()
	if err := controller.Restart(response, request); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
