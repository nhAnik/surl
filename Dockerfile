# build

FROM golang:1.20-alpine as build

WORKDIR /workspace/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o surl-app

# run executable

FROM alpine:latest

WORKDIR /bin

RUN apk --no-cache add ca-certificates

COPY --from=build /workspace/app/surl-app ./

EXPOSE 8090
CMD [ "/bin/surl-app" ]
