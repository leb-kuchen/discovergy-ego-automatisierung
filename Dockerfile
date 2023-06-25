FROM golang:latest AS build
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o /discovergy-ego-automatisieurng
EXPOSE 8080
CMD ["/discovergy-ego-automatisierung"]
