FROM golang:1.18 as build
WORKDIR /hackathon

COPY . .

RUN go build main.go

EXPOSE 8080
CMD ["./main"]