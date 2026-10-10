FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/racik-api ./cmd/server \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/mockkenari ./cmd/mockkenari

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
ENV APP_ADDR=:8080 UPLOAD_DIR=/data/uploads
VOLUME /data
EXPOSE 8080
CMD ["racik-api"]
