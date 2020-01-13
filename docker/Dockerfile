# -- multistage docker build: stage #1: build stage
FROM golang:1.13-alpine AS build
ARG KASPAD_VERSION

RUN mkdir -p /go/src/github.com/kaspanet/dnsseeder

WORKDIR /go/src/github.com/kaspanet/dnsseeder

RUN apk add --no-cache curl git openssh binutils gcc musl-dev
RUN go get -u golang.org/x/lint/golint

COPY go.mod .
COPY go.sum .

# For development it's useful to have kaspad from filesystem, but for deployment
# we should use the one in github
RUN go mod edit -dropreplace github.com/kaspanet/kaspad
RUN if [ -n "${KASPAD_VERSION}" ]; then go mod edit -require github.com/kaspanet/kaspad@$KASPAD_VERSION; fi
RUN cp go.mod go.mod.bu

RUN go mod download

COPY . .

# Restore go.mod because `COPY . .` overwrote it
RUN mv go.mod.bu go.mod

# Run linters and tests
RUN GOFMT_RESULT=`go fmt ./...`; echo $GOFMT_RESULT; test -z "$GOFMT_RESULT"
RUN go vet ./...
RUN golint -set_exit_status ./...
RUN go test -gcflags='-l' ./...

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dnsseeder .

# --- multistage docker build: stage #2: runtime image
FROM alpine
WORKDIR /app

RUN apk add --no-cache tini

COPY --from=build /go/src/github.com/kaspanet/dnsseeder/ /app/

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/app/dnsseeder"]
