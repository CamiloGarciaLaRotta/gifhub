FROM golang:alpine as compiler
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod .
COPY go.sum .
RUN go mod download

FROM alpine as final
WORKDIR /app
RUN apk add --no-cache ca-certificates ttf-freefont
ENTRYPOINT [ "./gifhub" ]

FROM compiler as base
COPY main.go .
RUN go build

FROM final
COPY --from=base /app/gifhub .

