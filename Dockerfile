# docker build -t 192.168.5.3:5000/fivethreeone .
FROM golang:1.20 as build

WORKDIR /project

COPY go.mod /project
COPY go.sum /project
COPY main.go /project/main.go
COPY db/ /project/db
COPY fto/ /project/fto
COPY server/ /project/server
COPY testing/ /project/testing

RUN go test ./... && GOOS=linux go build -ldflags "-linkmode external -extldflags -static" -o fivethreeone .

FROM gcr.io/distroless/static-debian11
COPY --from=build /project/fivethreeone /
COPY db/sqldb/migrations /migrations
CMD ["/fivethreeone"]
