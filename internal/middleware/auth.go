package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nhAnik/surl/internal/util"
)

func Jwt(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		split := strings.Split(r.Header.Get("Authorization"), " ")
		if len(split) != 2 || split[0] != "Bearer" {
			sendUnauthorizedMsg(w, "malformed Authorization header")
			return
		}
		tokenStr := split[1]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if !token.Valid || err != nil {
			sendUnauthorizedMsg(w, "malformed JWT")
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			ctx := context.WithValue(r.Context(), util.JwtClaimsKey, claims)
			next(w, r.WithContext(ctx))
		} else {
			sendUnauthorizedMsg(w, "malformed JWT")
		}
	}
}

func sendUnauthorizedMsg(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"message": msg,
	})
}
