package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// endpoints
// get test
func getTest(c *gin.Context) {
	fmt.Println("Recieved GET /test")
}

func main() {
	fmt.Println("Hello, World!")

	router := gin.Default()
	router.GET("/test", getTest)

	router.Run("0.0.0.0:8080")
}
