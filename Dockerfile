FROM golang:alpine AS build
RUN apk add build-base
RUN apk add git
WORKDIR /src
COPY . .
RUN go get .
RUN go build -o /soulshack

FROM alpine
COPY --from=build /src/config /config
COPY --from=build /soulshack /soulshack

CMD ["/soulshack"]

