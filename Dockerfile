FROM alpine:edge AS build
RUN apk add --no-cache --update go gcc g++
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o fatbot

FROM alpine:edge
WORKDIR /app
RUN apk add sqlite
COPY --from=build /app/fatbot /app/fatbot
ENTRYPOINT /app/fatbot
