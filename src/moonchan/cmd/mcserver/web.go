package main

import (
	"html/template"
	"log"
	"net/http"
	"sort"

	"moonchan/channels"
	"moonchan/receiver"
)

func render(t *template.Template, w http.ResponseWriter, data interface{}) {
	if err := t.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}

var indexT = template.Must(template.New("index").Parse(`<html>
<head>
<title>Moonchan</title>
<style>
table {
  width: 100%;
}
td {
  border: 1px solid black;
  border-collapse: collapse;
}
</style>
</head>
<body>
<h1>Moonchan</h1>

<table>
<tr>
<td>ID</td>
<td>Status</td>
<td>Capacity</td>
<td>Balance</td>
<td>Count</td>
</tr>
{{range .ChanItems}}
<tr>
<td>{{.ID}}</td>
<td>{{.State.Status}}</td>
<td>{{.State.FundingAmount}}</td>
<td>{{.State.Balance}}</td>
<td>{{.State.Count}}</td>
</tr>
{{end}}
</table>

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

func indexHandler(s *receiver.Receiver, w http.ResponseWriter, r *http.Request) {
	states := s.List()
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
