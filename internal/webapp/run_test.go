package webapp

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestValidateListenAddressAllowsOnlyLoopback(t *testing.T) {
	for _, address := range []string{
		"127.0.0.1:16900",
		"localhost:16900",
		"[::1]:16900",
	} {
		if err := validateListenAddress(address); err != nil {
			t.Fatalf("validateListenAddress(%q) error = %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:16900", ":16900", "example.com:16900"} {
		if err := validateListenAddress(address); err == nil {
			t.Fatalf("validateListenAddress(%q) error = nil", address)
		}
	}
}

func TestSessionTokenIsRandomHex(t *testing.T) {
	first, err := SessionToken()
	if err != nil {
		t.Fatalf("SessionToken() error = %v", err)
	}
	second, err := SessionToken()
	if err != nil {
		t.Fatalf("SessionToken() error = %v", err)
	}
	if len(first) != 64 || first == second {
		t.Fatalf("tokens = %q, %q", first, second)
	}
}

func TestRunRejectsPublicListenerBeforeUsingService(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		nil,
		nil,
		Options{Listen: "0.0.0.0:16900", Stdout: &stdout},
	)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestWaitUntilReadyWaitsForHTTPResponse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	server := &http.Server{
		Handler: http.HandlerFunc(func(
			response http.ResponseWriter,
			_ *http.Request,
		) {
			response.WriteHeader(http.StatusOK)
		}),
	}
	defer server.Close()
	startServing := make(chan struct{})
	go func() {
		<-startServing
		_ = server.Serve(listener)
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		close(startServing)
	}()

	started := time.Now()
	err = waitUntilReady(
		t.Context(),
		"http://"+listener.Addr().String(),
	)

	if err != nil {
		t.Fatalf("waitUntilReady() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed < 20*time.Millisecond {
		t.Fatalf("waitUntilReady() returned before serving after %v", elapsed)
	}
}
