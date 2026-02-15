FROM alpine:edge AS build
RUN apk add --no-cache --update go gcc g++
WORKDIR /app
COPY . .
RUN go test -v
RUN CGO_ENABLED=1 GOOS=linux go build -o fatbot

FROM alpine:edge
WORKDIR /app
RUN apk add --no-cache sqlite tzdata ttf-freefont ttf-dejavu
COPY --from=build /app/fatbot /app/fatbot
COPY --from=build /app/config.yaml /app/config.yaml
ENTRYPOINT /app/fatbot
