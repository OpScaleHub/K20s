FROM docker.io/library/golang:1.25 AS builder
WORKDIR /go/src
COPY go.mod .
#COPY go.sum .
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:3.21.3 AS certs
RUN apk add --no-cache ca-certificates=20250619-r0

FROM scratch
ENV PATH=/bin
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/src/main /main
EXPOSE 8080
ENTRYPOINT ["/main"]