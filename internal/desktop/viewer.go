package desktop

import (
	"net/http"
	"net/url"

	"github.com/pdparchitect/launcher/internal/agent"
)

func viewerURL(rawURL string, viewer string) string {
	if viewer != "kasmvnc" {
		return rawURL
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := target.Query()
	query.Set("show_control_bar", "true")
	query.Set("resize", "remote")
	query.Set("clipboard_up", "true")
	query.Set("clipboard_down", "true")
	query.Set("clipboard_seamless", "true")
	query.Set("enable_threading", "false")
	query.Set("enable_webp", "false")
	target.RawQuery = query.Encode()
	return target.String()
}

func viewerHandler(view agent.View, viewer string) http.Handler {
	return http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method != http.MethodGet || request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		http.Redirect(
			response,
			request,
			viewerURL(view.URL(), viewer),
			http.StatusTemporaryRedirect,
		)
	})
}
