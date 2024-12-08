package main

import (
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Raspberry Pi Management API",
		})
	})

	r.POST("/shutdown", func(c *gin.Context) {
		//cmd := exec.Command("shutdown", "-h", "now")
		cmd := exec.Command("systemctl", "halt")
		err := cmd.Run()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Shutdown initiated"})
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
