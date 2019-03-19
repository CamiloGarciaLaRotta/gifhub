FROM golang:alpine as builder
WORKDIR /app
COPY . .
RUN apk add --no-cache git
RUN go mod download && go build

FROM alpine
WORKDIR /app
COPY --from=builder /app/activitygiffer .
COPY svg.tmpl .
RUN apk add --no-cache imagemagick ca-certificates ttf-freefont
ENTRYPOINT [ "./activitygiffer" ]

