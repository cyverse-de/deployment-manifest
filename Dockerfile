FROM golang:1.7-alpine

COPY . /go/src/github.com/cyverse-de/deployment-manifest
RUN go install github.com/cyverse-de/deployment-manifest

ENTRYPOINT ["deployment-manifest"]
CMD ["--help"]
