# -- multistage docker build: stage #1: build stage
FROM golang:1.16-alpine AS build

RUN mkdir -p /go/src/github.com/kaspanet/dnsseeder

WORKDIR /go/src/github.com/kaspanet/dnsseeder

RUN apk add --no-cache curl git openssh binutils gcc musl-dev
RUN go get -u golang.org/x/lint/golint \
              honnef.co/go/tools/cmd/staticcheck 

COPY go.mod .
COPY go.sum .

# For development it's useful to have kaspad from filesystem, but for deployment
# we should use the one in github
RUN go mod edit -dropreplace github.com/kaspanet/kaspad
ARG KASPAD_VERSION
RUN if [ -n "${KASPAD_VERSION}" ]; then \
        # use replace instead of require - to propagate into nested dependancies
        go mod edit -replace github.com/kaspanet/kaspad=github.com/kaspanet/kaspad@$KASPAD_VERSION ;\
    fi

RUN go mod download

RUN cp go.mod go.mod.bu
RUN cp go.sum go.sum.bu

COPY . .

# Restore go.mod and go.sum because `COPY . .` overwrote them
RUN mv go.mod.bu go.mod
RUN mv go.sum.bu go.sum

RUN go mod tidy

# Run linters and tests
RUN GOFMT_RESULT=`go fmt ./...`; echo $GOFMT_RESULT; test -z "$GOFMT_RESULT"
RUN go vet ./...
RUN golint -set_exit_status ./...
RUN staticcheck -checks "SA4006,SA4008,SA4009,SA4010,SA5003,SA1004,SA1014,SA1021,SA1023,SA1024,SA1025,SA1026,SA1027,SA1028,SA2000,SA2001,SA2003,SA4000,SA4001,SA4003,SA4004,SA4011,SA4012,SA4013,SA4014,SA4015,SA4016,SA4017,SA4018,SA4019,SA4020,SA4021,SA4022,SA4023,SA5000,SA5002,SA5004,SA5005,SA5007,SA5008,SA5009,SA5010,SA5011,SA5012,SA6001,SA6002,SA9001,SA9002,SA9003,SA9004,SA9005,SA9006,ST1019" ./...
RUN go test -gcflags='-l' ./...

RUN GOOS=linux go build -a -installsuffix cgo -o dnsseeder .

# --- multistage docker build: stage #2: runtime image
FROM alpine
WORKDIR /app

RUN apk add --no-cache tini

COPY --from=build /go/src/github.com/kaspanet/dnsseeder/ /app/

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/app/dnsseeder"]
