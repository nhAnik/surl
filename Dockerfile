FROM golang:1.20-alpine

WORKDIR /workspace/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o surl-app

EXPOSE 8090

CMD [ "./surl-app" ]