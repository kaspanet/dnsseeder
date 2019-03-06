# -- multistage docker build: stage #1: build stage
FROM golang:1.11-alpine AS build

RUN mkdir -p /go/src/github.com/daglabs/dnsseeder

WORKDIR /go/src/github.com/daglabs/dnsseeder

RUN apk add --no-cache curl git openssh binutils gcc musl-dev
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN go get -u github.com/golang/lint/golint \
      github.com/kisielk/errcheck \
      github.com/opennota/check/cmd/aligncheck \
      github.com/opennota/check/cmd/structcheck \
      github.com/opennota/check/cmd/varcheck

COPY ./Gopkg.* ./

RUN dep ensure -v -vendor-only

COPY . .

RUN go vet ./...
# RUN aligncheck ./...
# RUN structcheck -e ./...
# RUN varcheck -e ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dnsseeder .

# --- multistage docker build: stage #2: runtime image
FROM alpine
WORKDIR /app

RUN apk add --no-cache ca-certificates tini

COPY --from=build /go/src/github.com/daglabs/dnsseeder/dnsseeder /app/

USER nobody
ENTRYPOINT ["/app/dnsseeder", "-H", "dnsseed.my.home", "-n", "192.168.16.104", "-l", "0.0.0.0:53", "-s", "localhost", "--testnet"]
