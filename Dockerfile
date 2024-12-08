FROM golang:1.22
#FROM golang:1.22-alpine

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