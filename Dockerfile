FROM docker.io/library/golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN go build -o /out/rmmctl ./cmd/rmmctl
RUN go build -o /out/rmm-api ./cmd/rmm-api

FROM gcr.io/distroless/base-debian12 AS rmm-api
COPY --from=build /out/rmm-api /rmm-api
COPY --from=build /src/migrations /migrations
EXPOSE 8080
ENTRYPOINT ["/rmm-api"]
