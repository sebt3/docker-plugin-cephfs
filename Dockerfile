FROM golang:1.17-alpine3.14 AS build
RUN apk add build-base ceph-dev linux-headers
WORKDIR /go/src/app
COPY . .
RUN go build .

FROM alpine:3.14
RUN apk add --no-cache ceph-common
COPY --from=build /go/src/app/docker-plugin-cephfs /bin/
CMD ["docker-plugin-cephfs"]
