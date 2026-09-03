FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/url-shortener ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/url-shortener /url-shortener
ENTRYPOINT ["/url-shortener"]
