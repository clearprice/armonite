package main

import (
	"fmt"
	"strings"
)

const armoniteASCII = `
 █████╗ ██████╗ ███╗   ███╗ ██████╗ ███╗   ██╗██╗████████╗███████╗
██╔══██╗██╔══██╗████╗ ████║██╔═══██╗████╗  ██║██║╚══██╔══╝██╔════╝
███████║██████╔╝██╔████╔██║██║   ██║██╔██╗ ██║██║   ██║   █████╗  
██╔══██║██╔══██╗██║╚██╔╝██║██║   ██║██║╚██╗██║██║   ██║   ██╔══╝  
██║  ██║██║  ██║██║ ╚═╝ ██║╚██████╔╝██║ ╚████║██║   ██║   ███████╗
╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝   ╚═╝   ╚══════╝
`

const subTitle = "Distributed Load Testing Framework"

func printArmoniteASCII() {
	// Colors for terminal output
	const (
		colorReset  = "\033[0m"
		colorBlue   = "\033[34m"
		colorCyan   = "\033[36m"
		colorGreen  = "\033[32m"
		colorPurple = "\033[35m"
		colorBold   = "\033[1m"
	)

	fmt.Println()

	// Print ASCII art with gradient colors
	lines := strings.Split(strings.TrimSpace(armoniteASCII), "\n")
	colors := []string{colorPurple, colorBlue, colorCyan, colorGreen, colorCyan, colorBlue}

	for i, line := range lines {
		colorIndex := i % len(colors)
		fmt.Printf("%s%s%s\n", colors[colorIndex], line, colorReset)
	}

	// Print subtitle centered
	fmt.Printf("%s%s%s%s%s\n",
		colorBold,
		strings.Repeat(" ", (len(lines[1])-len(subTitle))/2),
		subTitle,
		colorReset,
		colorReset)

	fmt.Println()
}

func printStartupBanner(config *Config) {
	printArmoniteASCII()

	const (
		colorReset = "\033[0m"
		colorGreen = "\033[32m"
		colorCyan  = "\033[36m"
		colorBold  = "\033[1m"
	)

	fmt.Printf("%s🚀 COORDINATOR STARTING%s\n", colorBold, colorReset)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("%s🌐 HTTP Server:%s http://%s:%d\n", colorCyan, colorReset, config.Server.Host, config.Server.HTTPPort)
	fmt.Printf("%s📋 API Info:%s http://%s:%d/\n", colorCyan, colorReset, config.Server.Host, config.Server.HTTPPort)
	fmt.Printf("%s🔌 API Endpoint:%s http://%s:%d/api/v1/status\n", colorCyan, colorReset, config.Server.Host, config.Server.HTTPPort)
	fmt.Printf("%s✅ Ready for agents!%s\n", colorGreen, colorReset)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}
