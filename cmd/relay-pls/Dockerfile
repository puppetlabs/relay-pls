FROM golang:1.14-alpine AS builder
ENV GO111MODULE on
ENV CGO_ENABLED 0
WORKDIR /build
COPY . .
RUN go build -a -installsuffix cgo -o /usr/bin/relay-pls ./cmd/relay-pls

FROM alpine:latest
COPY --from=builder /usr/bin/relay-pls /usr/bin/relay-pls
RUN apk --no-cache add ca-certificates && update-ca-certificates
CMD ["/usr/bin/relay-pls"]
