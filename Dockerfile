FROM golang:1.14-alpine AS build
RUN sed -i 's/dl-cdn.alpinelinux.org\/alpine/alpine.42.fr/g' /etc/apk/repositories && apk add build-base ceph-dev linux-headers

WORKDIR /go/src/app
COPY . .

#RUN go build --ldflags '-extldflags "-static"' .
RUN go build .

FROM alpine:3.11
RUN apk add --no-cache ceph-common
COPY --from=build /go/src/app/docker-plugin-cephfs /bin/
CMD ["docker-plugin-cephfs"]
