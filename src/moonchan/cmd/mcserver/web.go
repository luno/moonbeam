package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sort"

	"moonchan/channels"
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
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
<style>
</style>
</head>
<body>
<div class="container">

<h1>Moonchan</h1>

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
<td>{{.State.Status}}</td>
<td>{{.State.FundingAmount}}</td>
<td>{{.State.Balance}}</td>
<td>{{.State.Count}}</td>
</tr>
{{end}}
</tbody>
</table>

</div>
</body>
</html>`))

type chanItem struct {
	ID    string
	State channels.SharedState
}

type chanItems []chanItem

func (items chanItems) Len() int {
	return len(items)
}
func (items chanItems) Less(i, j int) bool {
	return items[i].ID < items[j].ID
}
func (items chanItems) Swap(i, j int) {
	items[i], items[j] = items[j], items[i]
}

func indexHandler(ss *ServerState, w http.ResponseWriter, r *http.Request) {
	states := ss.Receiver.List()
	var items []chanItem
	for id, state := range states {
		item := chanItem{id, state}
		items = append(items, item)
	}

	sort.Sort(chanItems(items))

	c := struct {
		ChanItems []chanItem
	}{items}
	render(indexT, w, c)
}

var detailsT = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Moonchan</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
<style>
</style>
</head>
<body>
<div class="container">

<h1>Channel {{.ID}}</h1>

<p>ID: {{.ID}}</p>

<pre>{{.StateJSON}}</pre>

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

	simple, err := s.ToSimple()
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	buf, err := json.MarshalIndent(simple, "", "   ")
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	c := struct {
		ID        string
		StateJSON string
	}{id, string(buf)}
	render(detailsT, w, c)
}
