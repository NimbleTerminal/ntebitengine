//go:build !js

// Package demorun selects terminal I/O for native and browser demos.
package demorun

import tea "charm.land/bubbletea/v2"

func Run(model tea.Model) error {
	_, err := tea.NewProgram(model).Run()
	return err
}
