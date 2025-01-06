FROM golang:1.22
#FROM golang:1.22-alpine

# Installation dependance pour libudev
RUN apt-get update && apt-get install -y libudev-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY *.go ./

RUN go mod tidy
RUN go build -o main .

RUN apt-get update && apt-get install -y iproute2
ENV PATH="${PATH}:/usr/sbin"

EXPOSE 8080

CMD ["./main"]
