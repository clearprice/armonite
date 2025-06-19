package main

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type UIData struct {
	Title    string
	Page     string
	Host     string
	Port     int
	HTTPPort int
	APIPort  int
}

func (c *Coordinator) setupUIRoutes(router *gin.Engine) {
	// Load templates with error handling
	templates, err := template.ParseGlob(filepath.Join("templates", "*.html"))
	if err != nil {
		LogError("Failed to parse templates: %v", err)
		LogError("UI will be disabled")
		return
	}

	LogInfo("Successfully loaded %d templates", len(templates.Templates()))
	router.SetHTMLTemplate(templates)

	// UI routes
	ui := router.Group("/ui")
	{
		ui.GET("/", c.handleUIDashboard)
		ui.GET("/agents", c.handleUIAgents)
		ui.GET("/test-runs", c.handleUITestRuns)
		ui.GET("/test-runs/:id", c.handleUITestRunDetail)
		ui.GET("/results", c.handleUIResults)
		ui.GET("/settings", c.handleUISettings)
	}

	// Root redirect to UI
	router.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(302, "/ui/")
	})

	uiPort := c.config.Server.HTTPPort + 1
	if uiPort == 1 {
		uiPort = 8081
	}
	LogInfo("Web UI routes configured for port %d", uiPort)
}

func (c *Coordinator) handleUIDashboard(ctx *gin.Context) {
	data := UIData{
		Title:    "Dashboard",
		Page:     "dashboard",
		Host:     c.host,
		Port:     c.port,
		HTTPPort: c.config.Server.HTTPPort,
		APIPort:  c.config.Server.HTTPPort,
	}
	ctx.HTML(http.StatusOK, "base.html", data)
}

func (c *Coordinator) handleUIAgents(ctx *gin.Context) {
	data := UIData{
		Title:    "Agents",
		Page:     "agents",
		Host:     c.host,
		Port:     c.port,
		HTTPPort: c.config.Server.HTTPPort,
		APIPort:  c.config.Server.HTTPPort,
	}
	ctx.HTML(http.StatusOK, "base.html", data)
}

func (c *Coordinator) handleUITestRuns(ctx *gin.Context) {
	data := UIData{
		Title:    "Test Runs",
		Page:     "test-runs",
		Host:     c.host,
		Port:     c.port,
		HTTPPort: c.config.Server.HTTPPort,
		APIPort:  c.config.Server.HTTPPort,
	}
	ctx.HTML(http.StatusOK, "base.html", data)
}

func (c *Coordinator) handleUITestRunDetail(ctx *gin.Context) {
	data := UIData{
		Title:    "Test Run Details",
		Page:     "test-runs",
		Host:     c.host,
		Port:     c.port,
		HTTPPort: c.config.Server.HTTPPort,
		APIPort:  c.config.Server.HTTPPort,
	}
	ctx.HTML(http.StatusOK, "base.html", data)
}

func (c *Coordinator) handleUIResults(ctx *gin.Context) {
	data := UIData{
		Title:    "Results",
		Page:     "results",
		Host:     c.host,
		Port:     c.port,
		HTTPPort: c.config.Server.HTTPPort,
		APIPort:  c.config.Server.HTTPPort,
	}
	ctx.HTML(http.StatusOK, "base.html", data)
}

func (c *Coordinator) handleUISettings(ctx *gin.Context) {
	data := UIData{
		Title:    "Settings",
		Page:     "settings",
		Host:     c.host,
		Port:     c.port,
		HTTPPort: c.config.Server.HTTPPort,
		APIPort:  c.config.Server.HTTPPort,
	}
	ctx.HTML(http.StatusOK, "base.html", data)
}
