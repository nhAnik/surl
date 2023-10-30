package middleware

import (
	"log"
	"net/http"
)

func Logging(f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path)
		f.ServeHTTP(w, r)
	})
}
