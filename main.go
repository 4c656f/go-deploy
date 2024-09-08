package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var (
	buildMutex sync.Mutex
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file")
	}

	r := gin.Default()
	r.POST("/webhook", webhookHandler)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server listening on port %s...\n", port)
	r.Run(":" + port)
}

func webhookHandler(c *gin.Context) {
	if !verifySignature(c.GetHeader("X-Hub-Signature-256")) {
		c.JSON(401, gin.H{"error": "Invalid signature"})
		return
	}

	var event struct {
		Ref string `json:"ref"`
	}
	if err := c.BindJSON(&event); err != nil {
		c.JSON(400, gin.H{"error": "Error parsing JSON: " + err.Error()})
		return
	}

	if event.Ref == "refs/heads/"+os.Getenv("GITHUB_BRANCH") {
		go safeRunUpdateScript()
		c.JSON(200, gin.H{"message": "Update process queued"})
	} else {
		c.JSON(200, gin.H{"message": "Ignoring push to non-target branch"})
	}
}

func verifySignature(signature string) bool {
	secretToken := os.Getenv("GITHUB_WEBHOOK_SECRET")
	mac := hmac.New(sha256.New, []byte(secretToken))
	expectedMAC := mac.Sum(nil)
	expectedSignature := "sha256=" + hex.EncodeToString(expectedMAC)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func safeRunUpdateScript() {
	buildMutex.Lock()
	defer buildMutex.Unlock()

	cmd := exec.Command("/bin/bash", os.Getenv("UPDATE_SCRIPT_PATH"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running update script: %v\n", err)
	}
}
