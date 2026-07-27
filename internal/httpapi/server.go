package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
)

//go:embed web
var webFiles embed.FS

type Service interface {
	Doctor(context.Context) (agent.DoctorReport, error)
	Catalog() []agent.CatalogEntry
	Create(context.Context, agent.CreateOptions) (domain.Instance, error)
	List(context.Context) ([]agent.View, error)
	Get(context.Context, string) (agent.View, error)
	Start(context.Context, string) (domain.Instance, error)
	Stop(context.Context, string) (domain.Instance, error)
	Rename(context.Context, string, string) (domain.Instance, error)
	RecentLogs(context.Context, string, int) (string, error)
	Delete(context.Context, string) error
}

type Server struct {
	service Service
	token   string
	handler http.Handler
	index   []byte
	logger  *log.Logger
}

type Option func(*Server)

func WithLogger(output io.Writer) Option {
	return func(server *Server) {
		if output != nil {
			server.logger = log.New(output, "[launcher] ", log.LstdFlags)
		}
	}
}

type instanceResponse struct {
	ID            string               `json:"id"`
	CatalogID     string               `json:"catalogId"`
	Name          string               `json:"name"`
	Image         string               `json:"image"`
	ContainerName string               `json:"containerName"`
	Port          int                  `json:"port"`
	State         launchruntime.Status `json:"state"`
	URL           string               `json:"url"`
	CreatedAt     time.Time            `json:"createdAt"`
	Metrics       *metricsResponse     `json:"metrics,omitempty"`
}

type metricsResponse struct {
	CPUPercent       *float64 `json:"cpuPercent,omitempty"`
	MemoryPercent    *float64 `json:"memoryPercent,omitempty"`
	MemoryUsageBytes uint64   `json:"memoryUsageBytes,omitempty"`
	MemoryLimitBytes uint64   `json:"memoryLimitBytes,omitempty"`
	UptimeSeconds    int64    `json:"uptimeSeconds"`
	Error            string   `json:"error,omitempty"`
}

type createRequest struct {
	CatalogID string `json:"catalogId"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	Port      int    `json:"port"`
	Start     *bool  `json:"start"`
}

type renameRequest struct {
	Name string `json:"name"`
}

type installEvent struct {
	Type     string            `json:"type"`
	Stage    agent.CreateStage `json:"stage,omitempty"`
	Message  string            `json:"message,omitempty"`
	Instance *instanceResponse `json:"instance,omitempty"`
	Error    string            `json:"error,omitempty"`
}

func New(service Service, token string, options ...Option) *Server {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		panic(fmt.Sprintf("read embedded Launcher interface: %v", err))
	}
	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(fmt.Sprintf("open embedded Launcher interface: %v", err))
	}
	server := &Server{
		service: service,
		token:   token,
		index:   index,
		logger:  log.New(io.Discard, "", 0),
	}
	for _, option := range options {
		option(server)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/doctor", server.doctor)
	mux.HandleFunc("GET /api/catalog", server.catalog)
	mux.HandleFunc("GET /api/instances", server.listInstances)
	mux.HandleFunc("POST /api/instances", server.createInstance)
	mux.HandleFunc("POST /api/instances/install", server.installInstance)
	mux.HandleFunc("GET /api/instances/{reference}/logs", server.instanceLogs)
	mux.HandleFunc("GET /api/instances/{reference}", server.getInstance)
	mux.HandleFunc("PATCH /api/instances/{reference}", server.renameInstance)
	mux.HandleFunc(
		"POST /api/instances/{reference}/{action}",
		server.changeInstance,
	)
	mux.HandleFunc("DELETE /api/instances/{reference}", server.deleteInstance)
	mux.Handle(
		"GET /catalog-assets/",
		http.StripPrefix(
			"/catalog-assets/",
			http.FileServer(http.FS(catalog.Assets())),
		),
	)
	mux.HandleFunc("GET /{$}", server.indexPage)
	mux.Handle("/", webAssetHandler(http.FileServer(http.FS(assets))))
	server.handler = server.logRequests(server.protectAPI(mux))
	return server
}

func (server *Server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	server.handler.ServeHTTP(response, request)
}

func webAssetHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		switch {
		case strings.HasSuffix(request.URL.Path, ".js"),
			strings.HasSuffix(request.URL.Path, ".css"):
			response.Header().Set("Cache-Control", "no-store")
		case strings.HasPrefix(request.URL.Path, "/assets/"):
			response.Header().Set("Cache-Control", "public, max-age=3600")
		}
		next.ServeHTTP(response, request)
	})
}

func (server *Server) protectAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/api/") {
			next.ServeHTTP(response, request)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		if request.Header.Get("X-Launcher-Token") != server.token {
			writeError(response, http.StatusUnauthorized, "invalid Launcher session")
			return
		}
		if origin := request.Header.Get("Origin"); origin != "" {
			if !sameRequestOrigin(origin, request.Host) {
				writeError(response, http.StatusForbidden, "invalid request origin")
				return
			}
		}
		next.ServeHTTP(response, request)
	})
}

func (server *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		writer := &statusWriter{ResponseWriter: response, status: http.StatusOK}
		next.ServeHTTP(writer, request)
		server.logger.Printf(
			"%s %s %d %s",
			request.Method,
			request.URL.Path,
			writer.status,
			time.Since(started).Round(time.Millisecond),
		)
	})
}

func (server *Server) indexPage(
	response http.ResponseWriter,
	request *http.Request,
) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	token := strings.NewReplacer(
		"&", "&amp;", `"`, "&#34;", "<", "&lt;", ">", "&gt;",
	).Replace(server.token)
	meta := `<meta name="launcher-token" content="` + token + `">`
	page := strings.Replace(string(server.index), "</head>", meta+"</head>", 1)
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	_, _ = response.Write([]byte(page))
}

func (server *Server) doctor(
	response http.ResponseWriter,
	request *http.Request,
) {
	report, err := server.service.Doctor(request.Context())
	if err != nil {
		writeJSON(response, http.StatusOK, map[string]any{
			"ready": false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"ready":  true,
		"report": report,
	})
}

func (server *Server) catalog(
	response http.ResponseWriter,
	_ *http.Request,
) {
	writeJSON(response, http.StatusOK, map[string]any{
		"catalog": server.service.Catalog(),
	})
}

func (server *Server) listInstances(
	response http.ResponseWriter,
	request *http.Request,
) {
	views, err := server.service.List(request.Context())
	if err != nil {
		writeServiceError(response, err)
		return
	}
	instances := make([]instanceResponse, 0, len(views))
	for _, view := range views {
		instances = append(instances, responseFromView(view))
	}
	writeJSON(response, http.StatusOK, map[string]any{"instances": instances})
}

func (server *Server) getInstance(
	response http.ResponseWriter,
	request *http.Request,
) {
	view, err := server.service.Get(
		request.Context(),
		request.PathValue("reference"),
	)
	if err != nil {
		writeServiceError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, responseFromView(view))
}

func (server *Server) renameInstance(
	response http.ResponseWriter,
	request *http.Request,
) {
	if !isJSON(request.Header.Get("Content-Type")) {
		writeError(response, http.StatusUnsupportedMediaType, "use application/json")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 16<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var body renameRequest
	if err := decoder.Decode(&body); err != nil {
		writeError(response, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if err := domain.ValidateName(body.Name); err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	instance, err := server.service.Rename(
		request.Context(),
		request.PathValue("reference"),
		body.Name,
	)
	if err != nil {
		writeServiceError(response, err)
		return
	}
	state := launchruntime.StatusStopped
	if instance.DesiredState == domain.DesiredRunning {
		state = launchruntime.StatusRunning
	}
	writeJSON(response, http.StatusOK, responseFromInstance(instance, state))
}

func (server *Server) instanceLogs(
	response http.ResponseWriter,
	request *http.Request,
) {
	logs, err := server.service.RecentLogs(
		request.Context(),
		request.PathValue("reference"),
		200,
	)
	if err != nil {
		writeServiceError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"logs": logs})
}

func (server *Server) createInstance(
	response http.ResponseWriter,
	request *http.Request,
) {
	body, start, ok := decodeCreateRequest(response, request)
	if !ok {
		return
	}
	instance, err := server.service.Create(request.Context(), agent.CreateOptions{
		CatalogID: body.CatalogID,
		Name:      body.Name,
		Image:     body.Image,
		Port:      body.Port,
		Start:     start,
	})
	if err != nil {
		writeServiceError(response, err)
		return
	}
	state := launchruntime.StatusCreated
	if start {
		state = launchruntime.StatusRunning
	}
	writeJSON(
		response,
		http.StatusCreated,
		responseFromInstance(instance, state),
	)
}

func (server *Server) installInstance(
	response http.ResponseWriter,
	request *http.Request,
) {
	body, start, ok := decodeCreateRequest(response, request)
	if !ok {
		return
	}
	response.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusOK)
	controller := http.NewResponseController(response)
	_ = controller.Flush()

	send := func(event installEvent) {
		_ = json.NewEncoder(response).Encode(event)
		_ = controller.Flush()
	}
	instance, err := server.service.Create(request.Context(), agent.CreateOptions{
		CatalogID: body.CatalogID,
		Name:      body.Name,
		Image:     body.Image,
		Port:      body.Port,
		Start:     start,
		Progress: func(progress agent.CreateProgress) {
			server.logger.Printf(
				"install %q: %s",
				body.Name,
				progress.Message,
			)
			send(installEvent{
				Type: "progress", Stage: progress.Stage, Message: progress.Message,
			})
		},
	})
	if err != nil {
		server.logger.Printf("install %q failed: %v", body.Name, err)
		send(installEvent{Type: "error", Error: err.Error()})
		return
	}
	state := launchruntime.StatusCreated
	if start {
		state = launchruntime.StatusRunning
	}
	result := responseFromInstance(instance, state)
	send(installEvent{Type: "complete", Instance: &result})
}

func (server *Server) changeInstance(
	response http.ResponseWriter,
	request *http.Request,
) {
	if !isJSON(request.Header.Get("Content-Type")) {
		writeError(response, http.StatusUnsupportedMediaType, "use application/json")
		return
	}
	reference := request.PathValue("reference")
	var (
		instance domain.Instance
		state    launchruntime.Status
		err      error
	)
	switch request.PathValue("action") {
	case "start":
		instance, err = server.service.Start(request.Context(), reference)
		state = launchruntime.StatusRunning
	case "stop":
		instance, err = server.service.Stop(request.Context(), reference)
		state = launchruntime.StatusStopped
	default:
		http.NotFound(response, request)
		return
	}
	if err != nil {
		writeServiceError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, responseFromInstance(instance, state))
}

func (server *Server) deleteInstance(
	response http.ResponseWriter,
	request *http.Request,
) {
	if err := server.service.Delete(
		request.Context(),
		request.PathValue("reference"),
	); err != nil {
		writeServiceError(response, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func responseFromView(view agent.View) instanceResponse {
	response := responseFromInstance(view.Instance, view.State)
	if view.State != launchruntime.StatusRunning {
		return response
	}
	metrics := &metricsResponse{
		MemoryUsageBytes: view.Metrics.MemoryUsageBytes,
		MemoryLimitBytes: view.Metrics.MemoryLimitBytes,
		UptimeSeconds:    int64(view.Uptime / time.Second),
		Error:            view.MetricsError,
	}
	if view.Metrics.CPUAvailable {
		metrics.CPUPercent = &view.Metrics.CPUPercent
	}
	if view.Metrics.MemoryAvailable {
		metrics.MemoryPercent = &view.Metrics.MemoryPercent
	}
	response.Metrics = metrics
	return response
}

func responseFromInstance(
	instance domain.Instance,
	state launchruntime.Status,
) instanceResponse {
	return instanceResponse{
		ID:            instance.ID,
		CatalogID:     instance.CatalogID,
		Name:          instance.Name,
		Image:         instance.Image,
		ContainerName: instance.ContainerName,
		Port:          instance.Port,
		State:         state,
		URL:           instance.URL(),
		CreatedAt:     instance.CreatedAt,
	}
}

func isJSON(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

func decodeCreateRequest(
	response http.ResponseWriter,
	request *http.Request,
) (createRequest, bool, bool) {
	if !isJSON(request.Header.Get("Content-Type")) {
		writeError(response, http.StatusUnsupportedMediaType, "use application/json")
		return createRequest{}, false, false
	}
	request.Body = http.MaxBytesReader(response, request.Body, 64<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var body createRequest
	if err := decoder.Decode(&body); err != nil {
		writeError(response, http.StatusBadRequest, "invalid request body")
		return createRequest{}, false, false
	}
	start := true
	if body.Start != nil {
		start = *body.Start
	}
	return body, start, true
}

func sameRequestOrigin(origin string, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, host)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(data []byte) (int, error) {
	return writer.ResponseWriter.Write(data)
}

func (writer *statusWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func writeServiceError(response http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, store.ErrDuplicateName) {
		status = http.StatusConflict
	}
	writeError(response, status, err.Error())
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
