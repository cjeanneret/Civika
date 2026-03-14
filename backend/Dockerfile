FROM golang:1.26-alpine AS build
WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o /bin/civika-api ./cmd/civika-api
RUN go build -o /bin/rag-cli ./cmd/rag-cli
RUN go build -o /bin/data-fetch ./cmd/data-fetch

FROM alpine:3.20
RUN adduser -D appuser
USER appuser
WORKDIR /app
COPY --from=build /bin/civika-api /app/civika-api
COPY --from=build /bin/rag-cli /app/rag-cli
COPY --from=build /bin/data-fetch /app/data-fetch
EXPOSE 8080
ENTRYPOINT ["/app/civika-api"]
