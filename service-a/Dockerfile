FROM golang:1.24 AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o service-a

FROM gcr.io/distroless/base-debian12
WORKDIR /app
ENV PORT=8080
COPY --from=build /app/service-a /app/service-a
EXPOSE 8080
CMD ["/app/service-a"]
