FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/github.com/tucats/ego
COPY . .
COPY tools/build .
COPY tools/buildver.txt ./tools/buildver.txt
RUN go mod download
RUN sh -v ./build
RUN cp ego /go/bin/ego 
RUN cp users.json /go/bin/
RUN cp tools/entrypoint.sh /go/bin/entrypoint.sh
RUN chmod a+x /go/bin/entrypoint.sh

FROM alpine
RUN mkdir /ego/
RUN touch /ego/users.json
COPY --from=builder /go/bin/users.json /ego/users.json
COPY --from=builder /go/bin/ego /go/bin/ego
COPY --from=builder /go/bin/entrypoint.sh /go/bin/entrypoint.sh 
COPY ./lib/. /ego/lib/.

EXPOSE 8080
ENTRYPOINT ["/go/bin/entrypoint.sh"]
