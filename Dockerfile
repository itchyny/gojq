FROM golang:1.24 AS builder

WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN make build

FROM gcr.io/distroless/static:debug

COPY --from=builder /app/gojq /
ENTRYPOINT ["/gojq"]
CMD ["--help"]
