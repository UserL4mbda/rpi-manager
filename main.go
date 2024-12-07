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
		cmd := exec.Command("sudo", "shutdown", "-h", "now")
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

	r.Run(":8080")
}
