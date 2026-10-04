FROM golang:1.26.8 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/devcade-leaderboard/ ./cmd/devcade-leaderboard/
COPY internal/profile/ ./internal/profile/
COPY internal/leaderboard/ ./internal/leaderboard/
COPY internal/metrics/ ./internal/metrics/
COPY internal/engine/ ./internal/engine/
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /devcade-leaderboard ./cmd/devcade-leaderboard

FROM alpine:3.22
RUN adduser -D -u 10001 devcade && mkdir /data && chown devcade:devcade /data
COPY --from=build /devcade-leaderboard /usr/local/bin/devcade-leaderboard
USER devcade
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["devcade-leaderboard"]
CMD ["-listen", "0.0.0.0:8080", "-data", "/data/leaderboard.json"]
