#!/bin/bash

# Configuration
API_HOST="localhost"
API_PORT="8080"
BASE_URL="http://${API_HOST}:${API_PORT}"

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Fonction pour afficher l'aide
show_help() {
    echo -e "${BLUE}Usage: $0 [COMMAND] [OPTIONS]${NC}"
    echo ""
    echo "Commandes disponibles:"
    echo -e "  ${GREEN}status${NC}        - Vérifier le statut de l'API"
    echo -e "  ${GREEN}network${NC}       - Afficher la configuration réseau"
    echo -e "  ${GREEN}delhotspot${NC}    - Supprimer le hotspot WiFi"
    echo -e "  ${GREEN}bridge${NC}        - Créer un bridge réseau"
    echo -e "  ${GREEN}shutdown${NC}      - Arrêter le système"
    echo -e "  ${GREEN}help${NC}          - Afficher cette aide"
    echo ""
    echo "Options:"
    echo -e "  ${YELLOW}-h HOST${NC}       - Spécifier l'adresse IP/hostname (défaut: localhost)"
    echo -e "  ${YELLOW}-p PORT${NC}       - Spécifier le port (défaut: 8080)"
    echo -e "  ${YELLOW}-v${NC}            - Mode verbeux"
    echo ""
    echo "Exemples:"
    echo "  $0 status"
    echo "  $0 -h 192.168.1.100 network"
    echo "  $0 -p 9090 delhotspot"
}

# Fonction pour faire une requête GET
api_get() {
    local endpoint=$1
    local verbose=$2
    
    echo -e "${BLUE}[INFO]${NC} Requête GET vers ${BASE_URL}${endpoint}"
    
    if [ "$verbose" = "true" ]; then
        curl_opts="-v"
    else
        curl_opts="-s"
    fi
    
    response=$(curl $curl_opts -w "\n%{http_code}" "${BASE_URL}${endpoint}" 2>/dev/null)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" -eq 200 ]; then
        echo -e "${GREEN}[SUCCESS]${NC} Réponse (HTTP $http_code):"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    else
        echo -e "${RED}[ERROR]${NC} Erreur HTTP $http_code"
        echo "$body"
    fi
}

# Fonction pour faire une requête POST
api_post() {
    local endpoint=$1
    local data=$2
    local verbose=$3
    
    echo -e "${BLUE}[INFO]${NC} Requête POST vers ${BASE_URL}${endpoint}"
    
    if [ "$verbose" = "true" ]; then
        curl_opts="-v"
    else
        curl_opts="-s"
    fi
    
    if [ -n "$data" ]; then
        response=$(curl $curl_opts -X POST -H "Content-Type: application/json" -d "$data" -w "\n%{http_code}" "${BASE_URL}${endpoint}" 2>/dev/null)
    else
        response=$(curl $curl_opts -X POST -w "\n%{http_code}" "${BASE_URL}${endpoint}" 2>/dev/null)
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" -eq 200 ]; then
        echo -e "${GREEN}[SUCCESS]${NC} Réponse (HTTP $http_code):"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    else
        echo -e "${RED}[ERROR]${NC} Erreur HTTP $http_code"
        echo "$body"
    fi
}

# Fonction pour vérifier si l'API est accessible
check_api() {
    echo -e "${BLUE}[INFO]${NC} Vérification de la connectivité avec l'API..."
    
    if ! curl -s --connect-timeout 5 "${BASE_URL}/" >/dev/null 2>&1; then
        echo -e "${RED}[ERROR]${NC} Impossible de se connecter à l'API sur ${BASE_URL}"
        echo -e "${YELLOW}[HINT]${NC} Vérifiez que:"
        echo "  - L'API Go est en cours d'exécution"
        echo "  - L'adresse et le port sont corrects"
        echo "  - Aucun firewall ne bloque la connexion"
        exit 1
    fi
}

# Fonction pour confirmer les actions destructives
confirm_action() {
    local action=$1
    echo -e "${YELLOW}[WARNING]${NC} Vous êtes sur le point d'exécuter: $action"
    read -p "Êtes-vous sûr ? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}[INFO]${NC} Action annulée"
        exit 0
    fi
}

# Traitement des options
VERBOSE=false
while getopts "h:p:v" opt; do
    case $opt in
        h)
            API_HOST="$OPTARG"
            BASE_URL="http://${API_HOST}:${API_PORT}"
            ;;
        p)
            API_PORT="$OPTARG"
            BASE_URL="http://${API_HOST}:${API_PORT}"
            ;;
        v)
            VERBOSE=true
            ;;
        \?)
            echo -e "${RED}[ERROR]${NC} Option invalide: -$OPTARG" >&2
            show_help
            exit 1
            ;;
    esac
done

shift $((OPTIND-1))

# Vérification qu'une commande a été fournie
if [ $# -eq 0 ]; then
    show_help
    exit 1
fi

COMMAND=$1

# Vérification de la connectivité API (sauf pour help)
if [ "$COMMAND" != "help" ]; then
    check_api
fi

# Traitement des commandes
case $COMMAND in
    "status")
        api_get "/" "$VERBOSE"
        ;;
    "network")
        api_get "/network" "$VERBOSE"
        ;;
    "delhotspot")
        confirm_action "suppression du hotspot"
        api_post "/delhotspot" "" "$VERBOSE"
        ;;
    "bridge")
        confirm_action "création du bridge réseau"
        api_post "/bridge" "" "$VERBOSE"
        ;;
    "shutdown")
        confirm_action "arrêt du système"
        api_post "/shutdown" "" "$VERBOSE"
        ;;
    "help")
        show_help
        ;;
    *)
        echo -e "${RED}[ERROR]${NC} Commande inconnue: $COMMAND"
        echo ""
        show_help
        exit 1
        ;;
esac
