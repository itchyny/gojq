FROM golang:1.16 as builder

WORKDIR /app
COPY . .
ENV CGO_ENABLED 0
RUN make build

FROM alpine:3.13

COPY --from=builder /app/gojq /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/gojq"]
CMD ["--help"]
