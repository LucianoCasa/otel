FROM golang:1.24 AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o service-a ./cmd/service-a

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/service-a .
EXPOSE 8080
CMD ["./service-a"]
