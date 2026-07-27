package web

import "embed"

//go:embed dashboard.html
var dashboardFS embed.FS

func DashboardHTML() string {
	b, _ := dashboardFS.ReadFile("dashboard.html")
	return string(b)
}
