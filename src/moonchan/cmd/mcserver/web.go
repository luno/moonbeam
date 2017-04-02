package main

import (
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sort"

	"moonchan/models"
	"moonchan/resolver"
	"moonchan/storage"
)

func render(t *template.Template, w http.ResponseWriter, data interface{}) {
	if err := t.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}

var indexT = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Moonchan</title>
<meta name="robots" content="noindex, nofollow">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
<style>
</style>
</head>
<body>
<div class="container">

<h1>Moonchan</h1>

<ul>
<li><a href="/channels">Channels</a></li>
</ul>

</div>
</body>
</html>`))

func indexHandler(ss *ServerState, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	c := struct {
	}{}
	render(indexT, w, c)
}

var channelsT = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Moonchan</title>
<meta name="robots" content="noindex, nofollow">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
<style>
</style>
</head>
<body>
<div class="container">

<h1>Moonchan</h1>

<p><a href="/">Home</a></p>

<table class="table">
<thead>
<tr>
<th>ID</th>
<th>Status</th>
<th>Capacity</th>
<th>Balance</th>
<th>Count</th>
</tr>
</thead>
<tbody>
{{range .ChanItems}}
<tr>
<td><a href="/details?id={{.ID}}">{{.ID}}</a></td>
<td>{{.SharedState.Status}}</td>
<td>{{.SharedState.Capacity}}</td>
<td>{{.SharedState.Balance}}</td>
<td>{{.SharedState.Count}}</td>
</tr>
{{end}}
</tbody>
</table>

</div>
</body>
</html>`))

type chanItems []storage.Record

func (items chanItems) Len() int {
	return len(items)
}
func (items chanItems) Less(i, j int) bool {
	return items[i].ID > items[j].ID
}
func (items chanItems) Swap(i, j int) {
	items[i], items[j] = items[j], items[i]
}

func channelsHandler(ss *ServerState, w http.ResponseWriter, r *http.Request) {
	recs, err := ss.Receiver.List()
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	sort.Sort(chanItems(recs))

	c := struct {
		ChanItems []storage.Record
	}{recs}
	render(channelsT, w, c)
}

var detailsT = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Moonchan</title>
<meta name="robots" content="noindex, nofollow">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
<style>
</style>
</head>
<body>
<div class="container">

<h1>Channel {{.ID}}</h1>

<p><a href="/">Home</a></p>

<p>ID: {{.ID}}</p>

<pre>{{.StateJSON}}</pre>

<form action="/close" method="post">
<input type="hidden" name="id" value="{{.ID}}">
<button type="submit">Close Channel</button>
</form>

<h2>Payments</h2>

{{if .Payments}}
<table>
{{range .Payments}}
<tr><td><code>{{.}}</code></td></tr>
{{end}}
</table>
{{else}}
None
{{end}}

</div>
</body>
</html>`))

func detailsHandler(ss *ServerState, w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	s := ss.Receiver.Get(id)
	if s == nil {
		http.NotFound(w, r)
		return
	}

	payments, err := ss.Receiver.ListPayments(id)
	if err != nil {
		log.Printf("error: %v", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	var pl []string
	for _, p := range payments {
		pl = append(pl, string(p))
	}

	buf, err := json.MarshalIndent(s, "", "   ")
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	c := struct {
		ID        string
		StateJSON string
		Payments  []string
	}{id, string(buf), pl}
	render(detailsT, w, c)
}

func closeHandler(ss *ServerState, w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")

	req := models.CloseRequest{ID: id}

	resp, err := ss.Receiver.Close(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(hex.EncodeToString(resp.CloseTx)))
}
func domainHandler(w http.ResponseWriter, r *http.Request) {
	d := resolver.Domain{
		Receivers: []resolver.DomainReceiver{
			{URL: *externalURL + rpcPath},
		},
	}
	json.NewEncoder(w).Encode(d)
}
