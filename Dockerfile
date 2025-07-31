FROM golang:1.22 AS builder
#la version alpine pose des problèmes avec le raspberry pi
#FROM golang:1.22-alpine

WORKDIR /app

# Copier les fichiers de dépendances
COPY go.mod go.sum ./

# Télécharger les dépendances
RUN go mod download
RUN go mod tidy

# Installation dependance pour libudev
RUN apt-get update && apt-get install -y libudev-dev && rm -rf /var/lib/apt/lists/*

#COPY *.go ./
# Copie du code source
COPY . .

#RUN go build -o main .
# Compiler l'application
# Ajout de flags pour une binaire statique (meilleure compatibilité)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rpi-manager .

# Étape 2 : Image d'exécution légère basée sur Debian
FROM debian:bookworm-slim

# Mettre à jour et installer les outils réseau + libudev-dev
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libudev-dev \
        tcpdump \
        nmap \
        iproute2 \
        net-tools \
        util-linux \
        iptables \
        procps \
    && rm -rf /var/lib/apt/lists/*


ENV PATH="${PATH}:/usr/sbin"

# Copier l'exécutable compilé
COPY --from=builder /app/rpi-manager /usr/local/bin/rpi-manager

EXPOSE 8080

#CMD ["./main"]
# Lancer l'application
CMD ["rpi-manager"]
