package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func handleDeploy(c *gin.Context) {
	var deployRequest DeployRequest
	if err := c.BindJSON(&deployRequest); err != nil {
		fmt.Print(err.Error())
	}
	go DockerAPI(deployRequest)
	c.JSON(http.StatusOK, deployRequest)
}

func main() {
	fmt.Print("Hello")
	// gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.POST("/deploy", handleDeploy)
	r.Run()
}
