package main

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"runny/src/core"
)

// chooseScript presents the interactive TUI menu and returns the selected script path.
// Returns the selected path and true, or empty string and false if the user exits.
func chooseScript(scripts []string, displayRoot string, cfg core.Config) (string, bool) {
	initTerminal()
	defer finiTerminal()

	autoRefresh := cfg.AutoRefresh != nil && *cfg.AutoRefresh
	if autoRefresh {
		stop := startAutoRefresh(displayRoot, cfg, scripts)
		defer stop()
	}
	setTerminalTitle("Runny")

	var query string
	selectedIndex := 0
	scrollOffset := 0

	for {
		// Filter scripts based on search query
		filtered := filterScripts(scripts, query)

		// Clamp selection index
		if len(filtered) == 0 {
			selectedIndex = 0
		} else if selectedIndex >= len(filtered) {
			selectedIndex = len(filtered) - 1
		}

		// Draw the menu
		drawMenu(filtered, displayRoot, query, selectedIndex, &scrollOffset, autoRefresh)
		// Wait for key press
		kp := readKey()

		switch kp.Action {
		case ActionUp:
			if len(filtered) > 0 {
				selectedIndex = (selectedIndex - 1 + len(filtered)) % len(filtered)
			}
		case ActionDown:
			if len(filtered) > 0 {
				selectedIndex = (selectedIndex + 1) % len(filtered)
			}
		case ActionEnter:
			if len(filtered) > 0 {
				return filtered[selectedIndex], true
			}
		case ActionEsc:
			return "", false
		case ActionRefresh:
			if !autoRefresh {
				scripts = core.FindScripts(displayRoot, cfg)
			}
		case ActionAutoRefresh:
			if autoRefresh && kp.Scripts != nil {
				scripts = kp.Scripts
			}
		case ActionBackspace:
			if len(query) > 0 {
				_, size := utf8.DecodeLastRuneInString(query)
				query = query[:len(query)-size]
				selectedIndex = 0
				scrollOffset = 0
			}
		case ActionChar:
			query += string(kp.Char)
			selectedIndex = 0
			scrollOffset = 0
		}
	}
}

// startAutoRefresh starts a background worker that polls for script changes every 2 seconds.
func startAutoRefresh(displayRoot string, cfg core.Config, initial []string) func() {
	quit := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		knownScripts := initial
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				newScripts := core.FindScripts(displayRoot, cfg)
				if !slices.Equal(knownScripts, newScripts) {
					s := screen
					if s == nil {
						return
					}
					err := s.PostEvent(tcell.NewEventInterrupt(refreshScriptsEvent{scripts: newScripts}))
					if err == nil {
						knownScripts = newScripts
					}
				}
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(quit)
			<-done
		})
	}
}

// filterScripts returns scripts whose filename contains the query (case-insensitive).
func filterScripts(scripts []string, query string) []string {
	if query == "" {
		return scripts
	}

	lowerQuery := strings.ToLower(query)
	var result []string
	for _, s := range scripts {
		filename := filepath.Base(s)
		if strings.Contains(strings.ToLower(filename), lowerQuery) {
			result = append(result, s)
		}
	}
	return result
}

// drawMenu renders the menu to the tcell screen.
func drawMenu(filtered []string, displayRoot, query string, selectedIndex int, scrollOffset *int, autoRefresh bool) {
	maxVisible := getMaxVisibleItems()

	// Clamp scroll offset
	if selectedIndex < *scrollOffset {
		*scrollOffset = selectedIndex
	}
	if selectedIndex >= *scrollOffset+maxVisible {
		*scrollOffset = selectedIndex - maxVisible + 1
	}
	maxOffset := max(len(filtered)-maxVisible, 0)
	if *scrollOffset > maxOffset {
		*scrollOffset = maxOffset
	}

	// Calculate required terminal width
	header := "Use UP / DOWN to move, Enter to run, F5 to refresh, Esc to exit."
	if autoRefresh {
		header = "Use UP / DOWN to move, Enter to run, Esc to exit."
	}
	maxWidth := len(header)
	searchLine := "Search: " + query
	if query == "" {
		searchLine = "Type to search..."
	}
	if len(searchLine) > maxWidth {
		maxWidth = len(searchLine)
	}

	// Check widths of visible items
	visibleEnd := min(*scrollOffset+maxVisible, len(filtered))
	for i := *scrollOffset; i < visibleEnd; i++ {
		relPath := getRelPath(filtered[i], displayRoot)
		lineLen := len(relPath) + 2 // for "> " or "  " prefix
		if lineLen > maxWidth {
			maxWidth = lineLen
		}
	}

	maxWidth += 2 // padding
	if maxWidth < 52 {
		maxWidth = 52
	}

	resizeTerminalWidth(maxWidth)

	// Clear and draw
	screen.Clear()

	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDefault)
	highlightStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorDefault).Bold(true)

	row := 0

	// Header
	drawString(0, row, style, header)
	row++
	drawString(0, row, style, searchLine)
	row++
	row++ // blank line
	drawString(0, row, style, "Choose a Python script to run:")
	row++
	row++ // blank line

	// Script list
	if len(filtered) == 0 {
		drawString(0, row, style, "  (No matching scripts found)")
	} else {
		// "..." indicator at top
		if *scrollOffset > 0 {
			drawString(0, row, style, "  ...")
			row++
		}

		for i := *scrollOffset; i < visibleEnd; i++ {
			relPath := getRelPath(filtered[i], displayRoot)
			if i == selectedIndex {
				drawString(0, row, highlightStyle, "> "+relPath)
			} else {
				drawString(0, row, style, "  "+relPath)
			}
			row++
		}

		// "..." indicator at bottom
		if visibleEnd < len(filtered) {
			drawString(0, row, style, "  ...")
		}
	}

	screen.Show()
}

// drawString writes a string to the screen at the given position.
func drawString(x, y int, style tcell.Style, str string) {
	for i, ch := range str {
		screen.SetContent(x+i, y, ch, nil, style)
	}
}

// getRelPath returns the relative path of script from displayRoot.
// Falls back to the absolute path if Rel fails.
func getRelPath(script, displayRoot string) string {
	rel, err := filepath.Rel(displayRoot, script)
	if err != nil {
		return script
	}
	return filepath.ToSlash(rel)
}
