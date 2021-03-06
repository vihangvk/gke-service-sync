FROM golang:1.14-buster AS builder
WORKDIR /go/src/app
ENV CGO_ENABLED=0 GOOS=linux
# for testing
RUN mkdir /defaults
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -o /go/src/app/gke-service-sync .
RUN go test . -v

FROM scratch
COPY --from=builder /go/src/app/gke-service-sync /gke-service-sync

ENTRYPOINT ["/gke-service-sync"]