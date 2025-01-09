package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"

	"github.com/gin-gonic/gin"
	"github.com/jochenvg/go-udev"
)

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
	cmd := exec.Command("/usr/bin/nmcli", "device", "wifi", "hotspot", "ifname", "wlan0","con-name","Hotspot", "ssid", "Entreprise", "password", "NCC-1701")

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
	}else{
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
		cmd := exec.Command("/usr/bin/nmcli", "connection","delete","id","Hotspot")
		output, err := cmd.Output()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Suppression du Hotspot"})
		fmt.Println(output)
	})

	r.GET("/network", func(c *gin.Context) {
		cmd := exec.Command("ifconfig")
		output, err := cmd.Output()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"output": string(output)})
	})

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
