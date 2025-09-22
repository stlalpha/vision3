package renderer

import (
	"fmt"
	"strconv"
	"strings"
)

const frameWidth = 68

func renderVisionX(ctx MenuContext, palette Palette) string {
	reset := palette.Reset()
	hiCorner := palette.Colour(palette.FrameCorner)
	hiEdge := palette.Colour(palette.FrameHigh)
	lowEdge := palette.Colour(palette.FrameLow)
	fadeEdge := palette.Colour(palette.FrameFade)
	accentPrimary := palette.Colour(palette.AccentPrimary)
	accentHighlight := palette.Colour(palette.AccentHighlight)
	accentSecondary := palette.Colour(palette.AccentSecondary)
	textPrimary := palette.Colour(palette.TextPrimary)
	textSecondary := palette.Colour(palette.TextSecondary)

	var output strings.Builder

	output.WriteString(buildHorizontalGradient(frameWidth, hiCorner, []string{hiEdge, lowEdge, fadeEdge}, reset))

	headerContent := fmt.Sprintf(" %s.(0o).%s           %sVI%sSiO%sN/3  %sAMiGA LiNK SYSOP CONSOLE%s          %s.(o0).%s ",
		accentHighlight, reset,
		accentPrimary, accentHighlight, accentPrimary,
		textPrimary, reset,
		accentHighlight, reset,
	)
	output.WriteString(buildContentLine(headerContent, hiEdge, hiEdge, reset))

	output.WriteString(buildContentLine("", lowEdge, fadeEdge, reset))

	newFiles := strconv.Itoa(max(0, ctx.Stats.NewFiles))
	online := strconv.Itoa(max(1, ctx.Stats.OnlineCount))
	ratio := ctx.Stats.Ratio
	if strings.TrimSpace(ratio) == "" {
		ratio = "100%"
	}

	primaryMsgLabel := ctx.Stats.PrimaryMessageArea
	if strings.TrimSpace(primaryMsgLabel) == "" {
		primaryMsgLabel = "Message Matrix"
	}
	primaryMsgUnread := strconv.Itoa(max(0, ctx.Stats.PrimaryMessageUnread))

	secondaryLabel := "Total Messages"
	secondaryValue := strconv.Itoa(max(0, ctx.Stats.TotalMessages))
	if len(ctx.Stats.TopMessageAreas) > 0 {
		secondaryLabel = ctx.Stats.TopMessageAreas[0].Name
		secondaryValue = strconv.Itoa(max(0, ctx.Stats.TopMessageAreas[0].Unread))
	}

	primaryFileLabel := ctx.Stats.PrimaryFileArea
	if strings.TrimSpace(primaryFileLabel) == "" {
		primaryFileLabel = "File Vault"
	}
	primaryFileNew := strconv.Itoa(max(0, ctx.Stats.PrimaryFileNew))
	if ctx.Stats.PrimaryFileNew == 0 {
		primaryFileNew = newFiles
	}

	menuLines := []string{
		fmt.Sprintf("  %s⟢%s 1 %s%-18s%s|%s unread: %s%-4s%s        %s⟢%s  ",
			accentHighlight, reset,
			accentSecondary, padLabel(primaryMsgLabel, 18),
			textSecondary, reset,
			accentHighlight, primaryMsgUnread, reset,
			accentHighlight, reset,
		),
		fmt.Sprintf("  %s⟢%s 2 %s%-18s%s|%s focus: %s%-4s%s        %s⟢%s  ",
			accentHighlight, reset,
			accentSecondary, padLabel(secondaryLabel, 18),
			textSecondary, reset,
			accentHighlight, secondaryValue, reset,
			accentHighlight, reset,
		),
		fmt.Sprintf("  %s⟢%s 3 %s%-18s%s|%s new files: %s%-4s%s     %s⟢%s  ",
			accentHighlight, reset,
			accentSecondary, padLabel(primaryFileLabel, 18),
			textSecondary, reset,
			accentHighlight, primaryFileNew, reset,
			accentHighlight, reset,
		),
		fmt.Sprintf("  %s⟢%s 4 %sReal-Time Lounge   %s|%s folks online: %s%-3s%s    %s⟢%s  ",
			accentHighlight, reset,
			accentSecondary,
			textSecondary, reset,
			accentHighlight, online, reset,
			accentHighlight, reset,
		),
	}

	for _, line := range menuLines {
		output.WriteString(buildContentLine(line, hiEdge, lowEdge, reset))
	}

	output.WriteString(buildContentLine("", lowEdge, fadeEdge, reset))

	handle := ctx.User.Handle
	if strings.TrimSpace(handle) == "" {
		handle = "Guest"
	}
	footerContent := fmt.Sprintf("  %s.(0o).%s %s%s%s logged in on %sNODE %02d%s ┊ ratio %s%s%s %s.(o0).%s  ",
		accentHighlight, reset,
		textPrimary, handle, reset,
		accentHighlight, ctx.User.Node, reset,
		accentPrimary, ratio, reset,
		accentHighlight, reset,
	)
	output.WriteString(buildContentLine(footerContent, hiEdge, lowEdge, reset))

	output.WriteString(buildHorizontalGradient(frameWidth, hiCorner, []string{hiEdge, lowEdge, fadeEdge}, reset))

	prompt := fmt.Sprintf("\n%s[%sPress %sRETURN%s or type a command%s]%s\n",
		textSecondary, reset, accentHighlight, reset, textSecondary, reset,
	)
	output.WriteString(prompt)

	return output.String()
}

func buildHorizontalGradient(width int, corner string, colours []string, reset string) string {
	var sb strings.Builder
	inner := width - 2
	sb.WriteString(corner)
	sb.WriteByte('+')
	colourIndex := 0
	for i := 0; i < inner; i++ {
		sb.WriteString(colours[colourIndex])
		sb.WriteByte('-')
		colourIndex = (colourIndex + 1) % len(colours)
	}
	sb.WriteString(corner)
	sb.WriteByte('+')
	sb.WriteString(reset)
	sb.WriteByte('\n')
	return sb.String()
}

func buildContentLine(content string, leftColour string, rightColour string, reset string) string {
	padded := padVisible(content, frameWidth-2)
	var sb strings.Builder
	sb.WriteString(leftColour)
	sb.WriteByte('|')
	sb.WriteString(reset)
	sb.WriteString(padded)
	sb.WriteString(rightColour)
	sb.WriteByte('|')
	sb.WriteString(reset)
	sb.WriteByte('\n')
	return sb.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func padLabel(value string, width int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > width {
		return string(runes[:width])
	}
	return string(runes) + strings.Repeat(" ", width-len(runes))
}
