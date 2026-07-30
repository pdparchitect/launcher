package webapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pdparchitect/launcher/internal/httpapi"
	"github.com/pdparchitect/launcher/internal/updatecheck"
)

type Opener interface {
	Open(string) error
}

type pathOpener interface {
	OpenPath(string) error
}

type Options struct {
	Listen        string
	Open          bool
	Stdout        io.Writer
	CatalogAssets fs.FS
	UpdateStatus  func() updatecheck.Status
	UpdateRefresh func(context.Context) (updatecheck.Status, error)
}

const startupTimeout = 5 * time.Second

func Run(
	ctx context.Context,
	service httpapi.Service,
	opener Opener,
	options Options,
) error {
	if err := validateListenAddress(options.Listen); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", options.Listen)
	if err != nil {
		return fmt.Errorf("listen for Launcher interface: %w", err)
	}
	defer listener.Close()

	token, err := SessionToken()
	if err != nil {
		return err
	}
	url := "http://" + listener.Addr().String()
	fmt.Fprintf(options.Stdout, "Starting Launcher web interface on %s\n", url)

	serverOptions := []httpapi.Option{httpapi.WithLogger(options.Stdout)}
	if options.CatalogAssets != nil {
		serverOptions = append(
			serverOptions,
			httpapi.WithCatalogAssets(options.CatalogAssets),
		)
	}
	if options.UpdateStatus != nil {
		serverOptions = append(
			serverOptions,
			httpapi.WithUpdateStatus(options.UpdateStatus),
		)
	}
	if options.UpdateRefresh != nil {
		serverOptions = append(
			serverOptions,
			httpapi.WithUpdateRefresh(options.UpdateRefresh),
		)
	}
	if opener, ok := opener.(pathOpener); ok {
		serverOptions = append(
			serverOptions,
			httpapi.WithPathOpener(opener.OpenPath),
		)
	}
	server := &http.Server{
		Handler:           httpapi.New(service, token, serverOptions...),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	result := make(chan error, 1)
	go func() {
		result <- server.Serve(listener)
	}()
	if err := waitUntilReady(ctx, url); err != nil {
		_ = server.Close()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("start Launcher web interface: %w", err)
	}
	fmt.Fprintf(options.Stdout, "Launcher web interface ready: %s\n", url)
	if options.Open {
		if err := opener.Open(url); err != nil {
			_ = server.Close()
			return fmt.Errorf("open Launcher interface: %w", err)
		}
	}

	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("stop Launcher interface: %w", err)
		}
		return nil
	}
}

func waitUntilReady(ctx context.Context, url string) error {
	readyCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	defer client.CloseIdleConnections()

	for {
		request, err := http.NewRequestWithContext(
			readyCtx,
			http.MethodGet,
			url,
			nil,
		)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-readyCtx.Done():
			return readyCtx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func validateListenAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", address, err)
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("Launcher interface must listen on a loopback address")
	}
	return nil
}

func SessionToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create Launcher session: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
