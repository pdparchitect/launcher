package desktop

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/pdparchitect/launcher/internal/agent"
)

var viewerPage = template.Must(template.New("viewer").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="theme-color" content="#050604">
    <title>{{.Name}} — Agent Launcher</title>
    <style>
      html, body {
        width: 100%;
        height: 100%;
        overflow: hidden;
        overscroll-behavior: none;
        margin: 0;
        background: #050604;
      }
      iframe {
        display: block;
        width: 100%;
        height: 100%;
        border: 0;
        background: #050604;
      }
    </style>
  </head>
  <body>
    <iframe src="{{.URL}}" title="{{.Name}}"
      allow="clipboard-read; clipboard-write; fullscreen"></iframe>
  </body>
</html>`))

type viewerPageData struct {
	Name string
	URL  string
}

func viewerHandler(view agent.View) http.Handler {
	return http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method != http.MethodGet || request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		if err := viewerPage.Execute(response, viewerPageData{
			Name: view.Name,
			URL:  view.URL(),
		}); err != nil {
			panic(fmt.Sprintf("render agent viewer: %v", err))
		}
	})
}
