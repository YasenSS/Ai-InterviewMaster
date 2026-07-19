FROM golang:1.26.5-bookworm AS build

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/interviewmaster-api ./apps/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/interviewmaster-worker ./apps/worker

FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=build /out/interviewmaster-api /interviewmaster-api
EXPOSE 8080
ENTRYPOINT ["/interviewmaster-api"]

FROM gcr.io/distroless/static-debian12:nonroot AS worker
COPY --from=build /out/interviewmaster-worker /interviewmaster-worker
ENTRYPOINT ["/interviewmaster-worker"]
