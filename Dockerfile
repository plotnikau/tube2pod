FROM golang as builder
WORKDIR /go/src/github.com/plotnikau/tube2pod
RUN CGO_ENABLED=0 GOOS=linux go get -u github.com/golang/dep/cmd/dep
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure --vendor-only
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o tube2pod main.go

FROM jrottenberg/ffmpeg:alpine
COPY --from=builder /go/src/github.com/plotnikau/tube2pod/tube2pod .
ENTRYPOINT []
CMD ["./tube2pod"]
