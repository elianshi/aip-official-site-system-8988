FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/ /app/
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
