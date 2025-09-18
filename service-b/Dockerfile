FROM golang:1.24 AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o service-b

FROM gcr.io/distroless/base-debian12
WORKDIR /app
ENV PORT=8081
COPY --from=build /app/service-b /app/service-b
EXPOSE 8081
CMD ["/app/service-b"]
