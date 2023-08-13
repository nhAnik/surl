package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/nhAnik/surl/database"
	"github.com/nhAnik/surl/handlers"
	"github.com/nhAnik/surl/middleware"
)

func setRoutes(r *mux.Router) {
	r.HandleFunc("/{url}", handlers.ResolveURL).Methods(http.MethodGet)
	r.Use(middleware.Logging)

	v1 := r.PathPrefix("/api/v1").Subrouter()
	v1.HandleFunc("/signup", handlers.SignUp).Methods(http.MethodPost)
	v1.HandleFunc("/login", handlers.Login).Methods(http.MethodPost)
	v1.HandleFunc("/token", middleware.Jwt(handlers.NewAccessToken)).Methods(http.MethodGet)

	v1.HandleFunc("", middleware.Jwt(handlers.ShortenURL)).Methods(http.MethodPost)
	v1.HandleFunc("/urls", middleware.Jwt(handlers.GetSurls)).Methods(http.MethodGet)
	v1.HandleFunc("/urls/:id", middleware.Jwt(handlers.GetSurl)).Methods(http.MethodGet)
	v1.HandleFunc("/urls/:id", middleware.Jwt(handlers.DeleteSurl)).Methods(http.MethodDelete)
	v1.HandleFunc("/urls/:id", middleware.Jwt(handlers.UpdateSurl)).Methods(http.MethodPut)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(err)
	}
	database.ConnectPg()
	database.ConnectRedis()
	defer database.DB.Close()

	r := mux.NewRouter()
	setRoutes(r)
	server := &http.Server{
		Addr:    os.Getenv("APP_PORT"),
		Handler: r,
	}
	log.Fatal(server.ListenAndServe())
}
