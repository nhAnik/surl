include .env

all: build run migrate

build:
	docker compose build

run:
	docker compose up

migrate:
	migrate -database postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/${POSTGRES_DB}?sslmode=disable \
	-path database/migrations up