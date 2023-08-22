package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nhAnik/surl/models"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	AccountExistsMsg = "account already exists"
	BadJsonMsg       = "bad json request"
	InvalidEmailMsg  = "invalid email"
)

type responseMap map[string]any

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthHandler struct {
	db          *sql.DB
	redisClient *redis.Client
}

func NewAuthHandler(DB *sql.DB, redisClient *redis.Client) *AuthHandler {
	return &AuthHandler{
		db:          DB,
		redisClient: redisClient,
	}
}

func (a *AuthHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	req := new(authRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorMsg(w, http.StatusBadRequest, BadJsonMsg)
		return
	}

	if !govalidator.IsEmail(req.Email) {
		sendErrorMsg(w, http.StatusBadRequest, InvalidEmailMsg)
		return
	}

	if existsEmail(a.db, req.Email) {
		sendErrorMsg(w, http.StatusUnprocessableEntity, AccountExistsMsg)
		return
	}

	hashedPass, _ := hashPassword(req.Password)
	user := models.User{
		Email:    req.Email,
		Password: hashedPass,
	}

	if id, err := insertUser(a.db, user.Email, user.Password); err != nil {
		sendErrorMsg(w, http.StatusInternalServerError, "signup failed")
	} else {
		user.ID = id
	}

	sendOkJsonResponse(w, user)
}

func insertUser(DB *sql.DB, email, password string) (int64, error) {
	var id int64
	sql := "INSERT INTO user_table (email, password, is_enabled) VALUES ($1, $2, $3) RETURNING id"
	if err := DB.QueryRow(sql, email, password, false).Scan(&id); err != nil {
		return id, err
	}
	return id, nil
}

func (a *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req := new(authRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorMsg(w, http.StatusBadRequest, BadJsonMsg)
		return
	}

	if !govalidator.IsEmail(req.Email) {
		sendErrorMsg(w, http.StatusBadRequest, InvalidEmailMsg)
		return
	}

	var expectedPassword string
	var userId int64
	sql := "SELECT id, password FROM user_table WHERE email = $1"
	if err := a.db.QueryRow(sql, req.Email).Scan(&userId, &expectedPassword); err != nil {
		sendErrorMsg(w, http.StatusUnauthorized, "login failed")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(expectedPassword), []byte(req.Password)); err != nil {
		sendErrorMsg(w, http.StatusUnauthorized, "login failed")
		return
	}

	accessToken, err := generateAccessToken(userId)
	if err != nil {
		sendInternalServerError(w)
		return
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		sendInternalServerError(w)
		return
	}

	if err := saveRefreshToken(a.redisClient, userId, refreshToken); err != nil {
		sendInternalServerError(w)
		return
	}

	sendOkJsonResponse(w, responseMap{
		"message":       "login successful",
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (a *AuthHandler) NewAccessToken(w http.ResponseWriter, r *http.Request) {
	userId, err := extractUserId(r.Context())
	if err != nil {
		sendInternalServerError(w)
		return
	}

	if _, err := loadRefreshToken(a.redisClient, userId); err != nil {
		sendInternalServerError(w)
		return
	}

	accessToken, err := generateAccessToken(userId)
	if err != nil {
		sendInternalServerError(w)
		return
	}

	sendOkJsonResponse(w, responseMap{
		"message":      "token generation successful",
		"access_token": accessToken,
	})
}

func sendOkJsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v)
}

func sendJsonResponse(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v)
}

func existsEmail(DB *sql.DB, email string) bool {
	var id int64
	sql := "SELECT id FROM user_table WHERE email = $1"
	if err := DB.QueryRow(sql, email).Scan(&id); err != nil {
		return false
	}
	return true
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func generateAccessToken(userId int64) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	expiryHr, _ := strconv.Atoi(os.Getenv("JWT_EXPIRATION_IN_HR"))

	claims := jwt.MapClaims{}
	claims["id"] = userId
	claims["expires"] = time.Now().Add(time.Hour * time.Duration(expiryHr)).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

func generateRefreshToken() (string, error) {
	refToken, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return refToken.String(), nil
}

func saveRefreshToken(redisClient *redis.Client, userId int64, token string) error {
	expiryHr, _ := strconv.Atoi(os.Getenv("REFRESH_TOKEN_EXPIRATION_IN_HR"))
	expiryHrDur := time.Duration(time.Hour * time.Duration(expiryHr))
	key := fmt.Sprintf("user:%d", userId)
	return redisClient.Set(context.Background(), key, token, expiryHrDur).Err()
}

func loadRefreshToken(redisClient *redis.Client, userId int64) (string, error) {
	key := fmt.Sprintf("user:%d", userId)
	getCmd := redisClient.Get(context.Background(), key)
	if err := getCmd.Err(); err != nil {
		return "", err
	}
	token, err := getCmd.Result()
	if err != nil {
		return "", err
	}
	return token, nil
}
