package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/nhAnik/surl/internal/database"
	"github.com/nhAnik/surl/internal/handlers"
	"github.com/nhAnik/surl/internal/middleware"
	"github.com/nhAnik/surl/internal/util"
)

func setRoutes(r *mux.Router, ah *handlers.AuthHandler, sh *handlers.SurlHandler) {
	r.Use(middleware.Logging)
	r.HandleFunc("/{url}", sh.ResolveURL).Methods(http.MethodGet)

	v1 := r.PathPrefix("/api/v1").Subrouter()
	v1.HandleFunc("/signup", ah.SignUp).Methods(http.MethodPost)
	v1.HandleFunc("/login", ah.Login).Methods(http.MethodPost)
	v1.HandleFunc("/token", middleware.Jwt(ah.NewAccessToken)).Methods(http.MethodGet)

	v1.HandleFunc("", middleware.Jwt(sh.ShortenURL)).Methods(http.MethodPost)
	v1.HandleFunc("/urls", middleware.Jwt(sh.GetSurls)).Methods(http.MethodGet)
	v1.HandleFunc("/urls/{id:[0-9]+}", middleware.Jwt(sh.GetSurl)).Methods(http.MethodGet)
	v1.HandleFunc("/urls/{id:[0-9]+}", middleware.Jwt(sh.DeleteSurl)).Methods(http.MethodDelete)
	v1.HandleFunc("/urls/{id:[0-9]+}", middleware.Jwt(sh.UpdateSurl)).Methods(http.MethodPut)
}

func main() {
	DB, err := database.InitDB()
	if err != nil {
		panic(err)
	}
	defer DB.Close()

	redis, err := database.InitRedis()
	if err != nil {
		panic(err)
	}

	sqid, err := util.InitSqid()
	if err != nil {
		panic(err)
	}

	authHandler := handlers.NewAuthHandler(DB, redis)
	surlHandler := handlers.NewSurlHandler(DB, redis, sqid)
	r := mux.NewRouter()
	setRoutes(r, authHandler, surlHandler)

	server := &http.Server{
		Addr:    os.Getenv("APP_PORT"),
		Handler: r,
	}
	log.Fatal(server.ListenAndServe())
}
