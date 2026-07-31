package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPreview struct {
	image []byte
	err   error
}

func (preview stubPreview) capture(context.Context) ([]byte, error) {
	return preview.image, preview.err
}

func TestBridgeHandlerHealth(t *testing.T) {
	queue, err := newNotificationQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	newBridgeHandler(stubPreview{}, queue).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ready" {
		t.Fatalf("health status = %q", body.Status)
	}
}

func TestBridgeHandlerPreview(t *testing.T) {
	queue, err := newNotificationQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/preview.jpg", nil)
	response := httptest.NewRecorder()
	newBridgeHandler(stubPreview{image: []byte{0xff, 0xd8, 0xff}}, queue).
		ServeHTTP(response, request)

	if response.Code != http.StatusOK ||
		response.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf(
			"response = status %d, content type %q",
			response.Code,
			response.Header().Get("Content-Type"),
		)
	}
}

func TestBridgeHandlerUnavailablePreview(t *testing.T) {
	queue, err := newNotificationQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/preview.jpg", nil)
	response := httptest.NewRecorder()
	newBridgeHandler(stubPreview{err: errors.New("no display")}, queue).
		ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestBridgeHandlerNotifications(t *testing.T) {
	queue, err := newNotificationQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	queue.generation = "test"
	queue.append(notification{ID: 7, Title: "Complete"})
	request := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	response := httptest.NewRecorder()
	newBridgeHandler(stubPreview{}, queue).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var page notificationPage
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.NextCursor != "test.1" ||
		len(page.Notifications) != 1 ||
		page.Notifications[0].ID != 7 {
		t.Fatalf("page = %#v", page)
	}
}
