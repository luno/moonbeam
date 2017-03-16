package main

import (
	"html/template"
	"log"
	"net/http"

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
<body>
<h1>Moonchan</h1>
</body>
</html>`))

func indexHandler(s *receiver.Receiver, w http.ResponseWriter, r *http.Request) {
	render(indexT, w, nil)
}
