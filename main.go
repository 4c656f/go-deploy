package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
	bodyCopy := new(bytes.Buffer)
	_, err := io.Copy(bodyCopy, c.Request.Body)

	if err != nil {
		c.JSON(400, gin.H{"error": "Error parsing request: " + err.Error()})
	}

	if err := verifySignature(c.GetHeader("X-Hub-Signature-256"), bodyCopy.Bytes()); err != nil {
		c.JSON(401, gin.H{"error": "Invalid signature: " + err.Error()})
		return
	}
	
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyCopy.Bytes()))

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

func verifySignature(signature string, payload []byte) error {
	signature_parts := strings.SplitN(signature, "=", 2)
	if len(signature_parts) != 2 {
		return fmt.Errorf("Invalid signature header: '%s' does not contain two parts (hash type and hash)", signature)
	}

	// Ensure secret is a sha1 hash
	signature_type := signature_parts[0]
	signature_hash := signature_parts[1]
	if signature_type != "sha256" {
		return fmt.Errorf("Signature should be a 'sha256' hash not '%s'", signature_type)
	}
	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	// Check that payload came from github
	// skip check if empty secret provided
	if !isValidPayload(secret, signature_hash, payload) {
		return fmt.Errorf("Payload did not come from GitHub")
	}

	return nil
}

func isValidPayload(secret, signature string, payload []byte) bool {
	hm := hmac.New(sha256.New, []byte(secret))
	hm.Write(payload)
	sum := hm.Sum(nil)
	hash := fmt.Sprintf("%x", sum)
	return hmac.Equal(
		[]byte(hash),
		[]byte(signature),
	)
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
