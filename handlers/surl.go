package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/nhAnik/surl/database"
	"github.com/nhAnik/surl/models"
	"github.com/nhAnik/surl/util"
)

const (
	length      = 8
	expiry      = 10 * time.Minute
	serverError = "server error"
)

func GetSurls(w http.ResponseWriter, r *http.Request) {
	userId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}

	sql := "SELECT id, short_url, url, expired_at, updated_at, clicked FROM url_table WHERE user_id = $1"
	rows, err := database.DB.Query(sql, userId)
	if err != nil {
		sendInternalServerError(w)
		return
	}

	defer rows.Close()

	var surlList []models.Surl
	for rows.Next() {
		var surl models.Surl
		err = rows.Scan(&surl.ID, &surl.ShortURL, &surl.URL, &surl.ExpiredAt, &surl.UpdatedAt, &surl.Clicked)
		if err != nil {
			sendInternalServerError(w)
			return
		}
		surlList = append(surlList, surl)
	}
	sendOkJsonResponse(w, responseMap{
		"surls": surlList,
	})
}

func GetSurl(w http.ResponseWriter, r *http.Request) {
	curUserId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}
	urlId := mux.Vars(r)["id"]
	var surl models.Surl
	var userId int64
	surl, userId, err = GetSurlById(urlId)
	if err != nil {
		sendJsonResponse(w, http.StatusNotFound, responseMap{
			"message": "url not found",
		})
		return
	}
	if curUserId != userId {
		sendJsonResponse(w, http.StatusUnauthorized, responseMap{
			"message": "authorization failure",
		})
		return
	}
	sendOkJsonResponse(w, surl)
}

func GetSurlById(urlId string) (models.Surl, int64, error) {
	sql := `SELECT id, short_url, url, expired_at, updated_at, clicked, user_id FROM url_table WHERE id = $1`

	var surl models.Surl
	var userId int64
	if err := database.DB.QueryRow(sql, urlId).Scan(
		&surl.ID, &surl.ShortURL, &surl.URL, &surl.ExpiredAt, &surl.UpdatedAt, &surl.Clicked, &userId); err != nil {
		return surl, 0, err
	}
	return surl, userId, nil
}

func DeleteSurl(w http.ResponseWriter, r *http.Request) {
	curUserId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}
	urlId := mux.Vars(r)["id"]
	var userId int64
	if err = database.DB.QueryRow(`SELECT user_id FROM url_table WHERE id = $1`, urlId).
		Scan(&userId); err != nil {
		sendJsonResponse(w, http.StatusNotFound, responseMap{
			"message": "url not found",
		})
	}
	if curUserId != userId {
		sendJsonResponse(w, http.StatusUnauthorized, responseMap{
			"message": "authorization failure",
		})
	}

	deleteSql := `DELETE FROM url_table WHERE id = $1`
	if _, err := database.DB.Exec(deleteSql, urlId); err != nil {
		sendInternalServerError(w)
		return
	}
	sendOkJsonResponse(w, responseMap{
		"message": "url deleted",
	})
}

func UpdateSurl(w http.ResponseWriter, r *http.Request) {
	curUserId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}
	urlId := mux.Vars(r)["id"]

	var surl models.Surl
	var userId int64
	surl, userId, err = GetSurlById(urlId)
	if err != nil {
		sendErrorMsg(w, http.StatusNotFound, "url not found")
		return
	}
	if curUserId != userId {
		sendErrorMsg(w, http.StatusUnauthorized, "authorization failure")
		return
	}

	type surlRequest struct {
		Alias string `json:"alias"`
	}
	req := new(surlRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorMsg(w, http.StatusBadRequest, "bad json request")
		return
	}

	if len(req.Alias) < 5 {
		sendErrorMsg(w, http.StatusUnprocessableEntity,
			"alias should be at least 5 characters")
		return
	}
	if len(req.Alias) > 10 {
		sendErrorMsg(w, http.StatusUnprocessableEntity,
			"alias should be at most 10 characters")
		return
	}
	if existsShortURL(req.Alias) {
		sendErrorMsg(w, http.StatusUnprocessableEntity, "alias not available")
		return
	}

	now := time.Now().UTC()
	expiredAt := now.Add(expiry)

	updateSql := `UPDATE url_table SET short_url = $1, expired_at = $2, updated_at = $3, is_alias = $4
		WHERE id = $5`
	if _, err := database.DB.Exec(updateSql, req.Alias, expiredAt, now, true, urlId); err != nil {
		sendInternalServerError(w)
		return
	}
	surl.ShortURL = req.Alias
	surl.IsAlias = true
	surl.UpdatedAt = now
	surl.ExpiredAt = expiredAt
	sendOkJsonResponse(w, surl)
}

func ResolveURL(w http.ResponseWriter, r *http.Request) {
	surl := mux.Vars(r)["url"]

	sql := "SELECT url, expired_at, clicked FROM url_table WHERE short_url = $1"
	var (
		longUrl   string
		clicked   uint64
		expiredAt time.Time
	)
	if err := database.DB.QueryRow(sql, surl).Scan(&longUrl, &expiredAt, &clicked); err != nil {
		sendErrorMsg(w, http.StatusNotFound, surl+" not found")
		return
	}
	if expiredAt.Before(time.Now().UTC()) {
		sendErrorMsg(w, http.StatusNotFound, surl+" expired")
		return
	}
	update := "UPDATE url_table SET clicked = $1 + 1 WHERE short_url = $2"
	if _, err := database.DB.Exec(update, clicked, surl); err != nil {
		sendInternalServerError(w)
		return
	}
	http.Redirect(w, r, longUrl, http.StatusTemporaryRedirect)
}

func ShortenURL(w http.ResponseWriter, r *http.Request) {
	type surlRequest struct {
		URL   string `json:"url"`
		Alias string `json:"alias"`
	}
	req := new(surlRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorMsg(w, http.StatusBadRequest, "bad json request")
		return
	}

	if !govalidator.IsURL(req.URL) {
		sendErrorMsg(w, http.StatusBadRequest, "invalid url")
		return
	}

	if req.Alias != "" {
		if len(req.Alias) < 5 {
			sendErrorMsg(w, http.StatusUnprocessableEntity,
				"alias should be at least 5 characters")
			return
		}
		if len(req.Alias) > 10 {
			sendErrorMsg(w, http.StatusUnprocessableEntity,
				"alias should be at most 10 characters")
			return
		}
	}

	now := time.Now().UTC()
	surl := models.Surl{
		URL:       req.URL,
		ShortURL:  getRandomStr(length),
		ExpiredAt: now.Add(expiry),
		UpdatedAt: now,
	}
	if req.Alias == "" {
		randomStr := getRandomStr(length)
		for existsShortURL(randomStr) {
			randomStr = getRandomStr(length)
		}
		surl.ShortURL = randomStr
	} else {
		if existsShortURL(req.Alias) {
			sendErrorMsg(w, http.StatusUnprocessableEntity, "alias not available")
			return
		}
		surl.ShortURL = req.Alias
		surl.IsAlias = true
	}

	longUrl := req.URL
	if !strings.Contains(longUrl, "://") {
		longUrl = "https://" + longUrl
	}

	userId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}

	sql := `INSERT INTO url_table (url, short_url, clicked, is_alias, expired_at, updated_at, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	if err := database.DB.QueryRow(sql, longUrl, surl.ShortURL, surl.Clicked, surl.IsAlias,
		surl.ExpiredAt, surl.UpdatedAt, userId).Scan(&surl.ID); err != nil {
		sendInternalServerError(w)
		return
	}
	sendOkJsonResponse(w, surl)
}

func extractUserId(ctx context.Context) (int64, error) {
	if claims, ok := ctx.Value(util.JwtClaimsKey).(jwt.MapClaims); ok {
		if id, ok := claims["id"]; ok {
			return int64(id.(float64)), nil
		}
	}
	return 0, errors.New("id not found")
}

func existsShortURL(surl string) bool {
	var id int64
	sql := "SELECT id FROM url_table WHERE short_url = $1"
	if err := database.DB.QueryRow(sql, surl).Scan(&id); err != nil {
		return false
	}
	return true
}

var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func getRandomStr(n int) string {
	str := make([]rune, n)
	for i := 0; i < n; i++ {
		str[i] = chars[rand.Intn(len(chars))]
	}
	return string(str)
}

func sendInternalServerError(w http.ResponseWriter) {
	sendErrorMsg(w, http.StatusInternalServerError, serverError)
}

func sendErrorMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
