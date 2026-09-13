package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"
)

var dashboardTmpl = template.Must(template.New("dash").Funcs(template.FuncMap{
	"ago": func(t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return time.Since(t).Round(time.Second).String() + " ago"
	},
	"ms": func(d time.Duration) string { return d.Round(time.Millisecond).String() },
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="{{.Refresh}}">
<title>lantern</title>
<style>
  body { font: 15px/1.5 system-ui, sans-serif; margin: 2rem auto; max-width: 40rem; color: #1a1a1a; }
  h1 { font-size: 1.3rem; }
  table { border-collapse: collapse; width: 100%; }
  th, td { text-align: left; padding: .5rem .6rem; border-bottom: 1px solid #ddd; }
  .up { color: #0a7d28; } .down { color: #c0261d; } .unknown { color: #777; }
  .dot::before { content: "\25CF "; }
  footer { color: #777; margin-top: 1.5rem; font-size: .85rem; }
</style>
</head>
<body>
<h1>lantern</h1>
<table>
<tr><th>service</th><th>status</th><th>response</th><th>last ok</th><th>since</th></tr>
{{range .Services}}
<tr>
  <td title="{{.URL}}">{{.Name}}</td>
  <td class="dot {{.Status}}">{{.Status}}{{if .Last.Err}} <small>({{.Last.Err}})</small>{{end}}</td>
  <td>{{if .Last.Latency}}{{ms .Last.Latency}}{{else}}&mdash;{{end}}</td>
  <td>{{ago .LastSuccess}}</td>
  <td>{{ago .Since}}</td>
</tr>
{{end}}
</table>
<footer>auto-refreshes every {{.Refresh}}s &middot; <a href="/api">/api</a> for JSON</footer>
</body>
</html>`))

// newServer builds the dashboard and JSON API handler.
func newServer(m *Monitor, refresh time.Duration) http.Handler {
	sec := int(refresh.Seconds())
	if sec < 1 {
		sec = 1
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.Snapshot())
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data := struct {
			Services []ServiceState
			Refresh  int
		}{m.Snapshot(), sec}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		dashboardTmpl.Execute(w, data)
	})

	return mux
}
