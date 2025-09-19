FROM golang:1.24 AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o service-b ./cmd/service-b

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/service-b .
EXPOSE 8081
CMD ["./service-b"]
