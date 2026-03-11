package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns the service health status.
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}
