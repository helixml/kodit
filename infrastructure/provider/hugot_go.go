//go:build !ORT

package provider

import "github.com/knights-analytics/hugot"

func newHugotSession() (*hugot.Session, error) {
	return hugot.NewGoSession()
}
