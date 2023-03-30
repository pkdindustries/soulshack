FROM golang:alpine as build
RUN apk add build-base
RUN apk add git
WORKDIR /src
COPY . .
RUN go get .
RUN go build -o /soulshack

FROM alpine
COPY --from=build /src/personalities /personalities
COPY --from=build /soulshack /soulshack

CMD ["/soulshack"]

