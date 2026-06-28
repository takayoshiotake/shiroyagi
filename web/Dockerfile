FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/webmail ./cmd/webmail

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/webmail /app/webmail
EXPOSE 8080
CMD ["/app/webmail"]
