package desktop

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
)

var viewerPage = template.Must(template.New("viewer").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Name}} — Agent Launcher</title>
</head>
<body>
  <script>
    // Asks the native side to badge this process's Dock tile, so an agent
    // window is distinguishable from the launcher. Must happen here: once we
    // navigate to the agent the page is a remote origin with no Wails runtime.
    try {
      window.webkit.messageHandlers.external.postMessage('dockbadge');
    } catch (error) {}
    window.location.replace({{.Target}});
  </script>
  <p><a href="{{.Target}}">Open {{.Name}}</a></p>
</body>
</html>`))

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

func viewerHandler(name string, target string, viewer string) http.Handler {
	return http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method != http.MethodGet || request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		var page bytes.Buffer
		if err := viewerPage.Execute(&page, map[string]string{
			"Name":   name,
			"Target": viewerURL(target, viewer),
		}); err != nil {
			http.Error(response, "prepare agent viewer", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write(page.Bytes())
	})
}
