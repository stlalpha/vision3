package renderer

import (
	"fmt"
	"strings"
)

const phosphorWidth = 60

func renderPhosphor(ctx MenuContext, palette Palette) string {
	reset := palette.Reset()
	frame := palette.Colour(palette.FrameLow)
	glow := palette.Colour(palette.FrameHigh)
	accent := palette.Colour(palette.AccentHighlight)
	text := palette.Colour(palette.TextPrimary)
	mono := palette.Colour(palette.TextSecondary)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s+%s%s%s+%s\n", palette.Colour(palette.FrameCorner), glow, strings.Repeat("=", phosphorWidth-2), palette.Colour(palette.FrameCorner), reset))

	header := fmt.Sprintf(" %sVISION/3%s NODE %s%02d%s :: RATIO %s%s%s :: UP %s%d%s ",
		accent, reset, text, ctx.User.Node, reset,
		text, ctx.Stats.Ratio, reset,
		text, ctx.Stats.Uploads, reset,
	)
	sb.WriteString(buildPhosphorLine(header, frame, reset))

	msgLabel := ctx.Stats.PrimaryMessageArea
	if strings.TrimSpace(msgLabel) == "" {
		msgLabel = "MESSAGE"
	}
	msgValue := fmt.Sprintf("%d", max(0, ctx.Stats.PrimaryMessageUnread))
	msgLine := fmt.Sprintf(" %s%-12s%s unread %s%-4s%s ", accent, strings.ToUpper(msgLabel), reset, text, msgValue, reset)
	sb.WriteString(buildPhosphorLine(msgLine, frame, reset))

	if len(ctx.Stats.TopMessageAreas) > 0 {
		for _, summary := range ctx.Stats.TopMessageAreas {
			line := fmt.Sprintf(" %s%-12s%s new %s%-4d%s ", mono, strings.ToUpper(summary.Name), reset, text, summary.Unread, reset)
			sb.WriteString(buildPhosphorLine(line, frame, reset))
		}
	} else {
		totalLine := fmt.Sprintf(" %sTOTAL MSGS%s %s%d%s ", mono, reset, text, max(0, ctx.Stats.TotalMessages), reset)
		sb.WriteString(buildPhosphorLine(totalLine, frame, reset))
	}

	fileLabel := ctx.Stats.PrimaryFileArea
	if strings.TrimSpace(fileLabel) == "" {
		fileLabel = "FILES"
	}
	fileLine := fmt.Sprintf(" %s%-12s%s count %s%-4d%s ", mono, strings.ToUpper(fileLabel), reset, text, max(0, ctx.Stats.PrimaryFileNew), reset)
	sb.WriteString(buildPhosphorLine(fileLine, frame, reset))

	footer := fmt.Sprintf(" %sCOMMAND?%s  ONLINE %s%d%s  DOORS %s%d%s ",
		accent, reset,
		accent, max(1, ctx.Stats.OnlineCount), reset,
		accent, max(0, ctx.Stats.ActiveDoors), reset,
	)
	sb.WriteString(buildPhosphorLine(footer, frame, reset))

	sb.WriteString(fmt.Sprintf("%s+%s%s%s+%s\n", palette.Colour(palette.FrameCorner), glow, strings.Repeat("=", phosphorWidth-2), palette.Colour(palette.FrameCorner), reset))

	prompt := fmt.Sprintf("\n%s>_ %s", text, reset)
	sb.WriteString(prompt)
	return sb.String()
}

func buildPhosphorLine(content string, frameColour string, reset string) string {
	padded := padVisible(content, phosphorWidth-2)
	var sb strings.Builder
	sb.WriteString(frameColour)
	sb.WriteByte('|')
	sb.WriteString(reset)
	sb.WriteString(padded)
	sb.WriteString(frameColour)
	sb.WriteByte('|')
	sb.WriteString(reset)
	sb.WriteByte('\n')
	return sb.String()
}
