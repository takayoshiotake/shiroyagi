FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/shiroyagi ./cmd/shiroyagi

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/shiroyagi /app/shiroyagi
EXPOSE 8080
CMD ["/app/shiroyagi"]
