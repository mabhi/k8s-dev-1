FROM golang:1.18-alpine

WORKDIR /usr/app



COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY ingress-reader/main.go ./

RUN CGO_ENABLED=0 go build -ldflags '-s' -o bin/lister main.go
# RUN CGO_ENABLED=0 go build -ldflags '-s' -o greeter main.go

#---

FROM scratch
COPY --from=0 /usr/app .
ENTRYPOINT ["./bin/lister"]