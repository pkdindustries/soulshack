FROM golang:alpine AS build
RUN apk add build-base
RUN apk add git
WORKDIR /src
COPY . .
RUN go mod download
RUN go build -o /soulshack ./cmd/soulshack

FROM alpine
COPY --from=build /src/examples /examples
COPY --from=build /soulshack /soulshack

CMD ["/soulshack"]

