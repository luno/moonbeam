package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"

	"github.com/luno/moonbeam/resolver"
	"github.com/luno/moonbeam/storage"
)

func render(t *template.Template, w http.ResponseWriter, data interface{}) {
	if err := t.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}

const header = `<!DOCTYPE html>
<html>
<head>
<title>Moonbeam</title>
<meta name="robots" content="noindex, nofollow">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
<style>
</style>
</head>
<body>
<div class="container">`

const footer = `
</div>
</body>
</html>`

var indexT = template.Must(template.New("index").Parse(header + `
<h1>Moonbeam</h1>

<p>Moonbeam is a protocol that uses Bitcoin payment channels to facilitate
instant off-chain payments between multi-user platforms.</p>

<p>This is a demo server running on testnet.</p>

<h4>More info</h4>

<ul>
<li><a href="https://github.com/luno/moonbeam">Github</a></li>
<li><a href="https://github.com/luno/moonbeam/blob/master/docs/overview.md">Overview</a></li>
<li><a href="https://github.com/luno/moonbeam/blob/master/docs/quickstart.md">Quickstart</a></li>
<li><a href="https://github.com/luno/moonbeam/blob/master/docs/spec.md">Specification</a></li>
</ul>

<h4>Channels</h4>

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

` + footer))

func indexHandler(ss *ServerState, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	recs, err := ss.Receiver.List()
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	sort.Sort(chanItems(recs))

	c := struct {
		ChanItems []storage.Record
	}{recs}
	render(indexT, w, c)
}

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

var detailsT = template.Must(template.New("index").Parse(header + `
<h1>Channel details</h1>

<p><a href="/">Home</a></p>

<p>ID: {{.ID}}</p>

<pre>{{.StateJSON}}</pre>

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

` + footer))

func detailsHandler(ss *ServerState, w http.ResponseWriter, r *http.Request) {
	txid, vout, ok := splitTxIDVout(r.FormValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	s := ss.Receiver.Get(txid, vout)
	if s == nil {
		http.NotFound(w, r)
		return
	}

	payments, err := ss.Receiver.ListPayments(txid, vout)
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
	}{fmt.Sprintf("%s-%d", txid, vout), string(buf), pl}
	render(detailsT, w, c)
}

func domainHandler(w http.ResponseWriter, r *http.Request) {
	d := resolver.Domain{
		Receivers: []resolver.DomainReceiver{
			{URL: *externalURL + rpcPath},
		},
	}
	json.NewEncoder(w).Encode(d)
}
