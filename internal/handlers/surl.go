package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/nhAnik/surl/internal/models"
	"github.com/nhAnik/surl/internal/util"
	"github.com/redis/go-redis/v9"
	"github.com/sqids/sqids-go"
)

const (
	serverError = "server error"
)

var chars = "1xnXM9kBN6cdYsAvjW3Co7luRePDh8ywaUQ4TStpfH0rqFVK2zimLGIJOgb5ZE"

type SurlHandler struct {
	db          *sql.DB
	redisClient *redis.Client
	sqid        *sqids.Sqids
}

func NewSurlHandler(DB *sql.DB, rc *redis.Client, s *sqids.Sqids) *SurlHandler {
	return &SurlHandler{
		db:          DB,
		redisClient: rc,
		sqid:        s,
	}
}

func (s *SurlHandler) GetSurls(w http.ResponseWriter, r *http.Request) {
	userId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}

	sql := "SELECT id, short_url, url, updated_at, clicked FROM url_table WHERE user_id = $1"
	rows, err := s.db.Query(sql, userId)
	if err != nil {
		sendInternalServerError(w)
		return
	}

	defer rows.Close()

	var surlList []models.Surl
	for rows.Next() {
		var surl models.Surl
		err = rows.Scan(&surl.ID, &surl.ShortURL, &surl.URL, &surl.UpdatedAt, &surl.Clicked)
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

func (s *SurlHandler) GetSurl(w http.ResponseWriter, r *http.Request) {
	curUserId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}
	urlId := mux.Vars(r)["id"]
	var surl models.Surl
	var userId int64
	surl, userId, err = getSurlById(s.db, urlId)
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

func getSurlById(DB *sql.DB, urlId string) (models.Surl, int64, error) {
	sql := `SELECT id, short_url, url, updated_at, clicked, user_id FROM url_table WHERE id = $1`

	var surl models.Surl
	var userId int64
	if err := DB.QueryRow(sql, urlId).Scan(
		&surl.ID, &surl.ShortURL, &surl.URL, &surl.UpdatedAt, &surl.Clicked, &userId); err != nil {
		return surl, 0, err
	}
	return surl, userId, nil
}

func (s *SurlHandler) DeleteSurl(w http.ResponseWriter, r *http.Request) {
	curUserId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}
	urlId := mux.Vars(r)["id"]
	var userId int64
	if err = s.db.QueryRow(`SELECT user_id FROM url_table WHERE id = $1`, urlId).
		Scan(&userId); err != nil {
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

	deleteSql := `DELETE FROM url_table WHERE id = $1`
	if _, err := s.db.Exec(deleteSql, urlId); err != nil {
		sendInternalServerError(w)
		return
	}
	sendOkJsonResponse(w, responseMap{
		"message": "url deleted",
	})
}

func (s *SurlHandler) UpdateSurl(w http.ResponseWriter, r *http.Request) {
	curUserId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}
	urlId := mux.Vars(r)["id"]

	var surl models.Surl
	var userId int64
	surl, userId, err = getSurlById(s.db, urlId)
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
	if existsShortURL(s.db, req.Alias) {
		sendErrorMsg(w, http.StatusUnprocessableEntity, "alias not available")
		return
	}

	now := time.Now().UTC()
	updateSql := `UPDATE url_table SET short_url = $1, updated_at = $2, is_alias = $3
		WHERE id = $4`
	if _, err := s.db.Exec(updateSql, req.Alias, now, true, urlId); err != nil {
		sendInternalServerError(w)
		return
	}
	surl.ShortURL = req.Alias
	surl.IsAlias = true
	surl.UpdatedAt = now
	sendOkJsonResponse(w, surl)
}

func (s *SurlHandler) ResolveURL(w http.ResponseWriter, r *http.Request) {
	surl := mux.Vars(r)["url"]

	var (
		longUrl string
		clicked uint64
	)
	sql := "SELECT url, clicked FROM url_table WHERE short_url = $1"
	if err := s.db.QueryRow(sql, surl).Scan(&longUrl, &clicked); err != nil {
		sendErrorMsg(w, http.StatusNotFound, surl+" not found")
		return
	}

	update := "UPDATE url_table SET clicked = $1 + 1 WHERE short_url = $2"
	if _, err := s.db.Exec(update, clicked, surl); err != nil {
		sendInternalServerError(w)
		return
	}
	http.Redirect(w, r, longUrl, http.StatusTemporaryRedirect)
}

func (s *SurlHandler) ShortenURL(w http.ResponseWriter, r *http.Request) {
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

	isAliasFromUser := req.Alias != ""

	if isAliasFromUser {
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
		if existsShortURL(s.db, req.Alias) {
			sendErrorMsg(w, http.StatusUnprocessableEntity, "alias not available")
			return
		}
	}

	now := time.Now().UTC()
	surl := models.Surl{
		URL:       req.URL,
		UpdatedAt: now,
	}
	if isAliasFromUser {
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

	sql := `INSERT INTO url_table (url, short_url, clicked, is_alias, updated_at, user_id)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	if err := s.db.QueryRow(sql, longUrl, surl.ShortURL, surl.Clicked, surl.IsAlias,
		surl.UpdatedAt, userId).Scan(&surl.ID); err != nil {
		sendInternalServerError(w)
		return
	}
	if !isAliasFromUser {
		// No user given alias, generate one from the id of the inserted row
		URLHash, err := getSurlFromID(surl.ID)
		if err != nil {
			sendInternalServerError(w)
			return
		}
		surl.ShortURL = URLHash
		updateSql := "UPDATE url_table SET short_url = $1 WHERE id = $2"
		if _, err := s.db.Exec(updateSql, surl.ShortURL, surl.ID); err != nil {
			sendInternalServerError(w)
			return
		}
	}
	sendOkJsonResponse(w, surl)
}

func getSurlFromID(id int64) (string, error) {
	var length = 8
	s, _ := sqids.NewCustom(sqids.Options{
		MinLength: &length,
		Alphabet:  &chars,
	})
	return s.Encode([]uint64{uint64(id)})
}

func extractUserId(ctx context.Context) (int64, error) {
	if claims, ok := ctx.Value(util.JwtClaimsKey).(jwt.MapClaims); ok {
		if id, ok := claims["id"]; ok {
			return int64(id.(float64)), nil
		}
	}
	return 0, errors.New("id not found")
}

func existsShortURL(DB *sql.DB, surl string) bool {
	var id int64
	sql := "SELECT id FROM url_table WHERE short_url = $1"
	if err := DB.QueryRow(sql, surl).Scan(&id); err != nil {
		return false
	}
	return true
}

func sendInternalServerError(w http.ResponseWriter) {
	sendErrorMsg(w, http.StatusInternalServerError, serverError)
}

func sendErrorMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
