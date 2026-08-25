package action

import (
    "fmt"
    "path/filepath"
    "strings"

    "github.com/micro-editor/micro/v2/internal/buffer"
    "github.com/micro-editor/micro/v2/internal/config"
    "github.com/micro-editor/micro/v2/internal/screen"
    "github.com/micro-editor/micro/v2/internal/shell"
)

// BTMagit is a scratch buffer type with syntax highlighting enabled
var BTMagit = buffer.BufType{Kind: 3, Readonly: false, Scratch: true, Syntax: true}

// getGitRoot returns the absolute path to the root of the current git repository
func getGitRoot() string {
    out, err := shell.RunCommand("git rev-parse --show-toplevel")
    if err != nil {
        return ""
    }
    return strings.TrimSpace(out)
}

// MagitStatus opens a new buffer showing the raw git status (porcelain format).
func (h *BufPane) MagitStatus() bool {
    gitRoot := getGitRoot()
    if gitRoot == "" {
        InfoBar.Error("Not a git repository")
        return false
    }

    out, err := shell.RunCommand(fmt.Sprintf("git -C %s status --porcelain=v1 -b", gitRoot))
    if err != nil {
        InfoBar.Error("Git error: ", err)
        return false
    }

    var buf *buffer.Buffer
    for _, b := range buffer.OpenBuffers {
        if b.GetName() == "magit" {
            buf = b
            break
        }
    }

    if buf == nil {
        buf = buffer.NewBufferFromString("", "magit", BTMagit)
        buf.SetOption("filetype", "magit")
    }

	// Check if the magit buffer is already open in a tab
	magitTabIndex := -1
	for i, t := range Tabs.List {
		if p := t.CurPane(); p != nil && p.Buf == buf {
			magitTabIndex = i
			break
		}
	}

	if magitTabIndex != -1 {
		// Switch to the existing magit tab
		Tabs.SetActive(magitTabIndex)
	} else {
		// Open a new tab with the magit buffer
		width, height := screen.Screen.Size()
		iOffset := config.GetInfoBarOffset()
		tp := NewTabFromBuffer(0, 0, width, height-iOffset, buf)
		Tabs.AddTab(tp)
		Tabs.SetActive(len(Tabs.List) - 1)
	}

	// The active pane is now the magit pane
	magitPane := MainTab().CurPane()
	if magitPane == nil {
		magitPane = h
	}

	c := magitPane.Cursor
	prevY := c.Y
	prevX := c.X

	// Temporarily set readonly to false so we can modify the buffer
	buf.SetOption("readonly", "false")
	buf.Remove(buf.Start(), buf.End())
	buf.Insert(buf.Start(), out)

	// Restore cursor position, clamping Y if the buffer got shorter
	if prevY >= buf.LinesNum() {
		prevY = buf.LinesNum() - 1
		if prevY < 0 {
			prevY = 0
		}
	}
	c.GotoLoc(buffer.Loc{X: prevX, Y: prevY})

	buf.SetOption("readonly", "true")
	magitPane.Relocate()
	return true
}

// MagitToggleFile stages or unstages the file under the cursor.
func (h *BufPane) MagitToggleFile() bool {
    if ft, ok := h.Buf.Settings["filetype"].(string); ok && ft != "magit" {
        return false
    }

    gitRoot := getGitRoot()
    if gitRoot == "" {
        return false
    }

    c := h.Buf.GetActiveCursor()
    line := h.Buf.Line(c.Y)
    if len(line) < 3 || strings.HasPrefix(line, "##") {
        return true
    }

    status := line[0:2]
    file := strings.TrimSpace(line[3:])

    var cmd string
    if status == "??" {
        // Untracked file
        cmd = "add "
    } else if status[0] != ' ' && status[0] != '?' {
        // Staged file (first char is not empty)
        cmd = "reset HEAD "
    } else if status[1] != ' ' {
        // Unstaged file (second char is not empty)
        cmd = "add "
    } else {
        return true
    }

    _, err := shell.RunCommand(fmt.Sprintf("git -C %s %s '%s'", gitRoot, cmd, file))
    if err != nil {
        InfoBar.Error("Git toggle error: ", err)
        return false
    }

    // Refresh the Magit buffer
    h.MagitStatus()
    return true
}

// MagitOpenFile opens the file under the cursor in the Magit buffer.
func (h *BufPane) MagitOpenFile() bool {
    if ft, ok := h.Buf.Settings["filetype"].(string); ok && ft != "magit" {
        return false
    }

    gitRoot := getGitRoot()
    if gitRoot == "" {
        return false
    }

    c := h.Buf.GetActiveCursor()
    line := h.Buf.Line(c.Y)
    if len(line) < 3 || strings.HasPrefix(line, "##") {
        return true
    }

    file := strings.TrimSpace(line[3:])
    // Resolve the absolute path so it works from any subdirectory
    fullPath := filepath.Join(gitRoot, file)

    b, err := buffer.NewBufferFromFile(fullPath, buffer.BTDefault)
    if err != nil {
        InfoBar.Error("Error opening file: ", err)
        return false
    }
    h.OpenBuffer(b)
    return true
}
