# docker build -t 192.168.5.3:5000/stronk .
FROM golang:1.20 as build

WORKDIR /project

COPY go.mod /project
COPY go.sum /project
COPY stronk.go /project/stronk.go
COPY stronk_test.go /project/stronk_test.go
COPY db/ /project/db
COPY cmd/ /project/cmd
COPY server/ /project/server
COPY testing/ /project/testing
# Needed for testing
COPY routine.example.json /project

RUN go test ./... && GOOS=linux go build -ldflags "-linkmode external -extldflags -static" -o stronk github.com/bcspragu/stronk/cmd/server

FROM gcr.io/distroless/static-debian11
COPY --from=build /project/stronk /
COPY db/sqldb/migrations /migrations
CMD ["/stronk"]
