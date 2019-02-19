package main

import (
	"flag"
	"github.com/go-chi/chi"
	"log"
	"net/http"
	"sync"
	"text/template"
)

func homeHandler(tpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tpl.Execute(w, r)
	})
}

func main() {
	flag.Parse()
	tpl := template.Must(template.ParseFiles("index.html"))
	hub := &Hub{
		connectionsMx: sync.RWMutex{},
		connections:   make(map[string]map[*connection]bool),
	}
	hub.newHub()
	router := chi.NewRouter()
	router.Handle("/", homeHandler(tpl))
	router.Handle("/ws", wsHandler{hub})
	log.Printf("Serving on port 3000")
	log.Fatal(http.ListenAndServe(":3000", router))
}
