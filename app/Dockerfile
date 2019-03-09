
FROM golang:alpine as builder

WORKDiR /go/src/app


COPy . .

RUN  go build -o app .

FROM amd64/alpine:latest

COPy --from=builder /go/src/app/app /usr/local/bin
RUN chmod +x  /usr/local/bin/app
CMD ["app"]