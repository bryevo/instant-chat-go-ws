package main

import (
	"flag"
	"github.com/go-chi/chi"
	"github.com/go-redis/redis"
	"log"
	"net/http"
	"text/template"
)

type wsHandler struct {
	hub   *Hub
	redis *redis.Client
}

func homeHandler(tpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tpl.Execute(w, r)
	})
}

func main() {
	flag.Parse()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	tpl := template.Must(template.ParseFiles("index.html"))
	hub := newHub()
	router := chi.NewRouter()
	router.Handle("/", homeHandler(tpl))
	router.Handle("/ws", wsHandler{hub, redisClient})
	log.Printf("Serving on port 3000")
	log.Fatal(http.ListenAndServe(":3000", router))
}
