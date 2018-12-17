FROM golang:1.11 AS builder

# Copy the code from the host and compile it
WORKDIR /go/src/github.com/twendt/secret-controller
# COPY Gopkg.toml Gopkg.lock ./
# RUN dep ensure --vendor-only

ENV GOPATH /go

COPY . ./
RUN go get
RUN go get -d k8s.io/code-generator | /bin/true
RUN hack/update-codegen.sh
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /secret-controller .

FROM alpine:3.8

RUN apk add --no-cache ca-certificates

COPY --from=builder /secret-controller ./
ENTRYPOINT ["./secret-controller"]