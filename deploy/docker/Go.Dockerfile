FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .
ARG APP
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/server ./apps/${APP}

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/server /app/server
USER app
ENTRYPOINT ["/app/server"]
