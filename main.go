package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jochenvg/go-udev"
)

// Structures pour le JSON de retour
type NetworkInterface struct {
	Name      string    `json:"name"`
	State     string    `json:"state"`
	Type      string    `json:"type"`
	MTU       int       `json:"mtu"`
	Addresses []Address `json:"addresses"`
}

type Address struct {
	Family string `json:"family"` // "inet" ou "inet6"
	IP     string `json:"ip"`
	Prefix int    `json:"prefix"`
	Scope  string `json:"scope,omitempty"`
}

type NetworkInfo struct {
	Interfaces []NetworkInterface `json:"interfaces"`
	Routes     []Route            `json:"routes"`
}

type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Device      string `json:"device"`
	Protocol    string `json:"protocol,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Metric      string `json:"metric,omitempty"`
}

func isInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// Fonction pour detecter si on utilise le reseau host
func isHostNetwork() bool {
	// Verifions si on peut voir les interfaces du host
	cmd := exec.Command("ip", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Si on trouve des interfaces, on est probablement en host network (comme wlan0, eth1, etc.)
	outputStr := string(output)
	hostIndicators := []string{"wlan0", "eth1", "vlan", "br0"}

	count := 0
	for _, indicator := range hostIndicators {
		if bytes.Contains([]byte(outputStr), []byte(indicator)) {
			count++
		}
	}

	return count > 0
}

// Fonction pour creer une commande avec geston des namespaces si besoin
func createNetworkCommande(ctx context.Context, command string, args ...string) *exec.Cmd {
	if !isInDocker() {
		// Execution directe si on est pas dans un container Docker
		return exec.CommandContext(ctx, command, args...)
	}

	if isHostNetwork() {
		// Si on est en host network, on peut executer directement la commande
		return exec.CommandContext(ctx, command, args...)
	}

	// Docker sans --network host : on utilise nsenter pour entrer dans le namespace du container
	nsenterArgs := []string{"-t", "1", "-n", command}
	nsenterArgs = append(nsenterArgs, args...)
	return exec.CommandContext(ctx, "nsenter", nsenterArgs...)
}

// Structure pour stocker les informations d'une interface réseau
type CaptureSession struct {
	ID      string `json:"id"`
	Cmd     *exec.Cmd
	Cancel  context.CancelFunc
	Running bool
	Output  []byte
}

var captureSessions = make(map[string]*CaptureSession)

// Fonction pour parser la sortie de "ip -j addr show"
func parseInterfaces() ([]NetworkInterface, error) {
	cmd := exec.Command("ip", "-j", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'exécution de 'ip addr show': %v", err)
	}

	var interfaces []NetworkInterface
	var rawInterfaces []map[string]interface{}

	if err := json.Unmarshal(output, &rawInterfaces); err != nil {
		return nil, fmt.Errorf("erreur lors du parsing JSON: %v", err)
	}

	for _, rawIface := range rawInterfaces {
		iface := NetworkInterface{
			Name:      rawIface["ifname"].(string),
			State:     strings.ToLower(rawIface["operstate"].(string)),
			Addresses: []Address{},
		}

		// MTU - gestion sécurisée du type
		if mtu, ok := rawIface["mtu"]; ok {
			switch v := mtu.(type) {
			case float64:
				iface.MTU = int(v)
			case int:
				iface.MTU = v
			}
		}

		// Type d'interface
		if linkType, ok := rawIface["link_type"]; ok {
			iface.Type = linkType.(string)
		} else {
			iface.Type = "unknown"
		}

		// Parser les adresses IP
		if addrInfo, ok := rawIface["addr_info"].([]interface{}); ok {
			for _, addr := range addrInfo {
				addrMap := addr.(map[string]interface{})

				// Vérifications de sécurité pour les champs requis
				family, familyOk := addrMap["family"].(string)
				local, localOk := addrMap["local"].(string)
				prefixlen, prefixlenOk := addrMap["prefixlen"]

				if !familyOk || !localOk || !prefixlenOk {
					continue // Ignorer cette adresse si les champs essentiels manquent
				}

				address := Address{
					Family: family,
					IP:     local,
				}

				// Gestion sécurisée du prefixlen
				switch v := prefixlen.(type) {
				case float64:
					address.Prefix = int(v)
				case int:
					address.Prefix = v
				}

				// Scope (optionnel)
				if scope, ok := addrMap["scope"].(string); ok {
					address.Scope = scope
				}

				iface.Addresses = append(iface.Addresses, address)
			}
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// Fonction pour parser la sortie de "ip -j route show"
func parseRoutes() ([]Route, error) {
	cmd := exec.Command("ip", "-j", "route", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'exécution de 'ip route show': %v", err)
	}

	var routes []Route
	var rawRoutes []map[string]interface{}

	if err := json.Unmarshal(output, &rawRoutes); err != nil {
		return nil, fmt.Errorf("erreur lors du parsing JSON des routes: %v", err)
	}

	for _, rawRoute := range rawRoutes {
		route := Route{}

		if dst, ok := rawRoute["dst"]; ok {
			route.Destination = dst.(string)
		} else {
			route.Destination = "default"
		}

		if gw, ok := rawRoute["gateway"]; ok {
			route.Gateway = gw.(string)
		}

		if dev, ok := rawRoute["dev"]; ok {
			route.Device = dev.(string)
		}

		if protocol, ok := rawRoute["protocol"]; ok {
			route.Protocol = protocol.(string)
		}

		if scope, ok := rawRoute["scope"]; ok {
			route.Scope = scope.(string)
		}

		if metric, ok := rawRoute["metric"]; ok {
			route.Metric = fmt.Sprintf("%.0f", metric.(float64))
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// Fonction de fallback si la commande ip ne supporte pas -j
func parseInterfacesLegacy() ([]NetworkInterface, error) {
	cmd := exec.Command("ip", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'exécution de 'ip addr show': %v", err)
	}

	var interfaces []NetworkInterface
	lines := strings.Split(string(output), "\n")
	var currentInterface *NetworkInterface

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Nouvelle interface (commence par un numéro)
		if strings.Contains(line, ": ") && !strings.HasPrefix(line, " ") {
			if currentInterface != nil {
				interfaces = append(interfaces, *currentInterface)
			}

			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := strings.TrimSuffix(parts[1], ":")
				currentInterface = &NetworkInterface{
					Name:      name,
					State:     "unknown",
					Type:      "unknown",
					MTU:       0,
					Addresses: []Address{},
				}

				// Extraire l'état et le MTU de la ligne
				if strings.Contains(line, "state ") {
					stateIndex := strings.Index(line, "state ")
					statePart := line[stateIndex+6:]
					stateEnd := strings.Index(statePart, " ")
					if stateEnd > 0 {
						currentInterface.State = strings.ToLower(statePart[:stateEnd])
					}
				}

				if strings.Contains(line, "mtu ") {
					mtuIndex := strings.Index(line, "mtu ")
					mtuPart := line[mtuIndex+4:]
					mtuEnd := strings.Index(mtuPart, " ")
					if mtuEnd > 0 {
						fmt.Sscanf(mtuPart[:mtuEnd], "%d", &currentInterface.MTU)
					}
				}
			}
		} else if currentInterface != nil && strings.HasPrefix(line, "inet") {
			// Adresse IP
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ipWithPrefix := parts[1]
				ipParts := strings.Split(ipWithPrefix, "/")
				if len(ipParts) == 2 {
					var prefix int
					fmt.Sscanf(ipParts[1], "%d", &prefix)

					address := Address{
						IP:     ipParts[0],
						Prefix: prefix,
					}

					if strings.HasPrefix(line, "inet ") {
						address.Family = "inet"
					} else if strings.HasPrefix(line, "inet6 ") {
						address.Family = "inet6"
					}

					if strings.Contains(line, "scope ") {
						scopeIndex := strings.Index(line, "scope ")
						scopePart := line[scopeIndex+6:]
						scopeEnd := strings.Index(scopePart, " ")
						if scopeEnd > 0 {
							address.Scope = scopePart[:scopeEnd]
						} else {
							address.Scope = strings.TrimSpace(scopePart)
						}
					}

					currentInterface.Addresses = append(currentInterface.Addresses, address)
				}
			}
		}
	}

	if currentInterface != nil {
		interfaces = append(interfaces, *currentInterface)
	}

	return interfaces, nil
}

func networkHandler(c *gin.Context) {
	var networkInfo NetworkInfo
	var err error

	// Essayer d'abord avec l'option JSON native
	networkInfo.Interfaces, err = parseInterfaces()
	if err != nil {
		// Fallback vers parsing manuel si -j n'est pas supporté
		networkInfo.Interfaces, err = parseInterfacesLegacy()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Impossible de récupérer les informations des interfaces réseau",
				"details": err.Error(),
			})
			return
		}
	}

	// Récupérer les routes
	networkInfo.Routes, err = parseRoutes()
	if err != nil {
		// Les routes ne sont pas critiques, on continue sans
		fmt.Printf("Avertissement: impossible de récupérer les routes: %v\n", err)
		networkInfo.Routes = []Route{}
	}

	c.JSON(http.StatusOK, networkInfo)
}

func checkUdev() {
	u := udev.Udev{}
	monitor := u.NewMonitorFromNetlink("udev")

	// Ajout de filtre pour reseau usb
	err := monitor.FilterAddMatchSubsystemDevtype("net", "usb_interface")
	if err != nil {
		//log.Fatalf("Error adding filter: %v", err)
		fmt.Printf("Error adding filter: %v\n", err)
	}

	// Creer un context avec annulation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ouvrir le monitor et recuperer les canaux
	udevChan, errChan, err := monitor.DeviceChan(ctx)
	if err != nil {
		fmt.Printf("Error starting monitor: %v", err)
	}

	// Gestion des interruptions (CTRL+C)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		fmt.Println("\nStopping monitor...")
		cancel() // Annule le context pour Arret propre du monitor
		os.Exit(0)
	}()

	fmt.Println("Monitoring USB network interfaces...")

	for {
		select {
		case device := <-udevChan:
			if device == nil {
				continue
			}
			action := device.Action()
			devPath := device.Devpath()

			if action == "add" {
				fmt.Printf("USB network interface added: %s\n", devPath)
			} else if action == "remove" {
				fmt.Printf("USB network interface removed: %s\n", devPath)
			} else {
				fmt.Printf("Unknow action: %s on device: %s\n", action, devPath)
			}
		case err := <-errChan:
			if err != nil {
				fmt.Printf("Error from monitor: %v", err)
			}
		case <-ctx.Done():
			fmt.Println("Context canceled, stopping monitor.")
			return
		}
	}
}

func createHotspot() {
	fmt.Println("Creation du Hotspot")
	cmd := exec.Command("/usr/bin/nmcli", "device", "wifi", "hotspot", "ifname", "wlan0", "con-name", "Hotspot", "ssid", "Entreprise", "password", "NCC-1701")

	// Buffers pour stdout et stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Redirection des flux
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Affichage de l'erreur, du flux d'erreur et du flux standard
		fmt.Println("Runtime error: ", err)
		fmt.Println("Sortie d'erreur:", stderr.String())
		fmt.Println("Sortie standard:", stdout.String())
		return
	} else {
		// Affichage des sorties
		fmt.Println(stdout.String())
		fmt.Println(stderr.String())
	}
}

func main() {

	createHotspot()

	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Raspberry Pi Management API",
		})
	})

	r.POST("/shutdown", func(c *gin.Context) {
		cmd := exec.Command("systemctl", "halt")
		err := cmd.Run()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Shutdown initiated"})
	})

	r.POST("/delhotspot", func(c *gin.Context) {
		cmd := exec.Command("/usr/bin/nmcli", "connection", "delete", "id", "Hotspot")
		output, err := cmd.Output()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Suppression du Hotspot"})
		fmt.Println(output)
	})

	r.GET("/network", networkHandler)

	r.POST("/bridge", func(c *gin.Context) {
		cmds := []*exec.Cmd{
			exec.Command("ip", "link", "add", "name", "br0", "type", "bridge"),

			exec.Command("ip", "link", "set", "eth0", "master", "br0"),
			exec.Command("ip", "link", "set", "eth1", "master", "br0"),

			exec.Command("ip", "link", "set", "br0", "up"),
			exec.Command("ip", "link", "set", "eth0", "up"),
			exec.Command("ip", "link", "set", "eth1", "up"),

			exec.Command("ip", "link", "set", "eth0", "promisc", "on"),
			exec.Command("ip", "link", "set", "eth1", "promisc", "on"),

			exec.Command("ip", "addr", "flush", "dev", "eth0"),
			exec.Command("ip", "addr", "flush", "dev", "eth1"),
		}

		for _, cmd := range cmds {
			err := cmd.Run()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Bridge created"})
		}

	})

	r.Run(":8080")
}
