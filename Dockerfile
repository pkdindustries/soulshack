FROM golang:alpine as build
RUN apk add build-base
RUN apk add git
WORKDIR /src
COPY . .
RUN go get .
RUN go build -o /gptbot

FROM alpine
COPY --from=build /gptbot /gptbot
CMD ["/gptbot"]

