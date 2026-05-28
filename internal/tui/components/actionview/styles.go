package actionview

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

type actionPalette struct {
	Black        color.Color
	Blue         color.Color
	BrightBlue   color.Color
	BrightGreen  color.Color
	BrightRed    color.Color
	BrightWhite  color.Color
	BrightYellow color.Color
	Purple       color.Color
	Red          color.Color
	White        color.Color
	Yellow       color.Color
	Bg           color.Color
	Fg           color.Color
}

type paneItemStyles struct {
	focusedTitleStyle   lipgloss.Style
	unfocusedTitleStyle lipgloss.Style

	selectedDescStyle        lipgloss.Style
	descStyle                lipgloss.Style
	focusedSelectedDescStyle lipgloss.Style

	selectedStyle             lipgloss.Style
	selectedTitleStyle        lipgloss.Style
	focusedSelectedTitleStyle lipgloss.Style

	focusedSelectedStyle lipgloss.Style
}

type colors struct {
	darkColor      color.Color
	darkerColor    color.Color
	lightColor     color.Color
	errorColor     color.Color
	warnColor      color.Color
	successColor   color.Color
	focusedColor   color.Color
	unfocusedColor color.Color
	subtleWhite    color.Color
	whiteColor     color.Color
	faintColor     color.Color
	fainterColor   color.Color
}

type styles struct {
	colors  colors
	palette actionPalette

	defaultListStyles          lipgloss.Style
	focusedPaneTitleStyle      lipgloss.Style
	unfocusedPaneTitleStyle    lipgloss.Style
	focusedPaneTitleBarStyle   lipgloss.Style
	unfocusedPaneTitleBarStyle lipgloss.Style
	normalItemDescStyle        lipgloss.Style

	paneItem paneItemStyles

	paneStyle                  lipgloss.Style
	focusedPaneStyle           lipgloss.Style
	lineNumbersStyle           lipgloss.Style
	canceledGlyph              lipgloss.Style
	skippedGlyph               lipgloss.Style
	neutralGlyph               lipgloss.Style
	waitingGlyph               lipgloss.Style
	pendingGlyph               lipgloss.Style
	failureGlyph               lipgloss.Style
	successGlyph               lipgloss.Style
	noLogsStyle                lipgloss.Style
	watermarkIllustrationStyle lipgloss.Style
	debugStyle                 lipgloss.Style
	errorBgStyle               lipgloss.Style
	errorStyle                 lipgloss.Style
	errorTitleStyle            lipgloss.Style
	separatorStyle             lipgloss.Style
	commandStyle               lipgloss.Style
	stepStartMarkerStyle       lipgloss.Style
	groupStartMarkerStyle      lipgloss.Style
	faintFgStyle               lipgloss.Style
	keyStyle                   lipgloss.Style
}

func makeStyles(appTheme *theme.Theme) styles {
	p := actionPalette{
		Black:        lipgloss.Color("0"),
		Blue:         lipgloss.Color("4"),
		BrightBlue:   lipgloss.Color("12"),
		BrightGreen:  lipgloss.Color("10"),
		BrightRed:    lipgloss.Color("9"),
		BrightWhite:  lipgloss.Color("15"),
		BrightYellow: lipgloss.Color("11"),
		Purple:       lipgloss.Color("5"),
		Red:          lipgloss.Color("1"),
		White:        lipgloss.Color("15"),
		Yellow:       lipgloss.Color("3"),
		Bg:           lipgloss.Color("0"),
		Fg:           lipgloss.Color("15"),
	}

	focusedColor := p.BrightBlue
	colors := colors{
		focusedColor:   focusedColor,
		unfocusedColor: lipgloss.Darken(p.BrightBlue, 0.7),
		darkColor:      lipgloss.Darken(focusedColor, 0.2),
		darkerColor:    lipgloss.Darken(focusedColor, 0.7),
		lightColor:     lipgloss.Lighten(focusedColor, 0.2),
		errorColor:     p.BrightRed,
		warnColor:      p.BrightYellow,
		successColor:   p.BrightGreen,
		faintColor:     lipgloss.Darken(focusedColor, 0.4),
		fainterColor:   lipgloss.Darken(focusedColor, 0.8),
		whiteColor:     p.White,
		subtleWhite:    lipgloss.Darken(p.White, 0.2),
	}

	errorBgStyle := lipgloss.NewStyle().Background(lipgloss.Darken(p.Red, 0.8))
	bg := lipgloss.Darken(p.Bg, 0.4)
	selectedBg := lipgloss.Lighten(p.BrightBlue, 0.2)
	unfocusedBg := lipgloss.Darken(focusedColor, 0.5)
	unfocusedFg := lipgloss.Darken(focusedColor, 0.1)
	metadataColor := lipgloss.Color("245")
	baseTitleStyle := lipgloss.NewStyle().Bold(true).Margin(0)

	s := styles{
		colors:  colors,
		palette: p,

		faintFgStyle: lipgloss.NewStyle().Foreground(colors.faintColor),

		focusedPaneTitleStyle:      baseTitleStyle.Foreground(p.Black),
		unfocusedPaneTitleStyle:    baseTitleStyle.Foreground(p.Fg),
		focusedPaneTitleBarStyle:   lipgloss.NewStyle().Bold(true).PaddingRight(0).MarginBottom(1),
		unfocusedPaneTitleBarStyle: lipgloss.NewStyle().Bold(true).PaddingRight(0).MarginBottom(1),

		normalItemDescStyle: lipgloss.NewStyle().Foreground(metadataColor).PaddingLeft(4),

		paneItem: paneItemStyles{
			selectedStyle: lipgloss.NewStyle().
				Background(bg).
				BorderBackground(bg).
				Border(lipgloss.OuterHalfBlockBorder(), false, false, false, true).
				BorderForeground(unfocusedBg),

			focusedSelectedStyle: lipgloss.NewStyle().
				Background(selectedBg).
				BorderForeground(selectedBg).
				BorderBackground(selectedBg).
				Border(lipgloss.OuterHalfBlockBorder(), false, false, false, true),

			selectedTitleStyle: lipgloss.NewStyle().
				Bold(true).
				Foreground(unfocusedFg).
				Background(bg),

			focusedTitleStyle: lipgloss.NewStyle().Bold(true).Foreground(p.White),
			focusedSelectedTitleStyle: lipgloss.NewStyle().
				Bold(true).
				Foreground(p.Black).
				Background(selectedBg),
			unfocusedTitleStyle: lipgloss.NewStyle().
				Bold(true).
				Foreground(colors.subtleWhite),

			selectedDescStyle: lipgloss.NewStyle().
				Foreground(metadataColor).
				PaddingLeft(2).
				Background(bg),
			descStyle: lipgloss.NewStyle().
				Foreground(metadataColor).
				PaddingLeft(2),
			focusedSelectedDescStyle: lipgloss.NewStyle().
				Foreground(p.Black).
				PaddingLeft(2).
				Background(selectedBg),
		},

		paneStyle: lipgloss.NewStyle().BorderRight(true).BorderStyle(
			lipgloss.NormalBorder(),
		).BorderForeground(colors.faintColor),
		focusedPaneStyle: lipgloss.NewStyle().BorderRight(true).BorderStyle(
			lipgloss.NormalBorder(),
		).BorderForeground(colors.focusedColor),
		lineNumbersStyle: lipgloss.NewStyle().
			Foreground(colors.faintColor).
			Align(lipgloss.Right),
		canceledGlyph: lipgloss.NewStyle().
			Foreground(colors.warnColor).
			SetString(constants.CanceledIcon),
		skippedGlyph: lipgloss.NewStyle().
			Foreground(metadataColor).
			SetString(constants.SkippedIcon),
		neutralGlyph: lipgloss.NewStyle().
			Foreground(colors.whiteColor).
			SetString(constants.NeutralIcon),
		waitingGlyph: lipgloss.NewStyle().Foreground(p.Yellow).SetString(constants.WaitingIcon),
		pendingGlyph: lipgloss.NewStyle().
			Foreground(metadataColor).
			SetString(constants.PendingIcon),
		failureGlyph: lipgloss.NewStyle().Foreground(p.Red).SetString(constants.FailureIcon),
		successGlyph: lipgloss.NewStyle().
			Foreground(colors.successColor).
			SetString(constants.SuccessIcon),
		noLogsStyle:                lipgloss.NewStyle().Foreground(colors.faintColor).Bold(true),
		watermarkIllustrationStyle: lipgloss.NewStyle().Foreground(p.White),
		debugStyle:                 lipgloss.NewStyle().Background(lipgloss.Color("1")),
		errorBgStyle:               errorBgStyle,
		errorStyle:                 errorBgStyle.Foreground(colors.errorColor).Bold(false),
		errorTitleStyle:            errorBgStyle.Foreground(colors.errorColor).Bold(true),
		separatorStyle:             lipgloss.NewStyle().Foreground(colors.fainterColor),
		commandStyle:               lipgloss.NewStyle().Foreground(p.Blue).Inline(true),
		stepStartMarkerStyle:       lipgloss.NewStyle().Bold(true).Inline(true),
		groupStartMarkerStyle:      lipgloss.NewStyle().Inline(true),
		keyStyle: lipgloss.NewStyle().
			Background(colors.fainterColor).
			Background(colors.darkerColor).
			Padding(0, 1),
	}

	if appTheme != nil {
		s.colors.focusedColor = appTheme.PrimaryBorder
		s.colors.unfocusedColor = appTheme.FaintBorder
		s.colors.errorColor = appTheme.ErrorText
		s.colors.warnColor = appTheme.WarningText
		s.colors.successColor = appTheme.SuccessText
		s.colors.faintColor = appTheme.FaintText
		s.colors.fainterColor = appTheme.FaintBorder
		s.colors.whiteColor = appTheme.PrimaryText
		s.colors.subtleWhite = appTheme.SecondaryText
		s.focusedPaneTitleStyle = baseTitleStyle.Foreground(appTheme.InvertedText)
		s.unfocusedPaneTitleStyle = baseTitleStyle.Foreground(appTheme.PrimaryText)
		s.faintFgStyle = lipgloss.NewStyle().Foreground(appTheme.FaintText)
		s.normalItemDescStyle = s.normalItemDescStyle.Foreground(appTheme.FaintText)
		s.paneItem.descStyle = s.paneItem.descStyle.Foreground(appTheme.FaintText)
		s.paneItem.selectedDescStyle = s.paneItem.selectedDescStyle.Foreground(appTheme.FaintText)
		s.paneItem.focusedSelectedDescStyle = s.paneItem.focusedSelectedDescStyle.Foreground(appTheme.FaintText)
		s.paneStyle = s.paneStyle.BorderForeground(appTheme.FaintBorder)
		s.focusedPaneStyle = s.focusedPaneStyle.BorderForeground(appTheme.PrimaryBorder)
		s.lineNumbersStyle = s.lineNumbersStyle.Foreground(appTheme.FaintText)
		s.noLogsStyle = s.noLogsStyle.Foreground(appTheme.FaintText)
		s.failureGlyph = s.failureGlyph.Foreground(appTheme.ErrorText)
		s.successGlyph = s.successGlyph.Foreground(appTheme.SuccessText)
		s.skippedGlyph = s.skippedGlyph.Foreground(appTheme.FaintText)
		s.pendingGlyph = s.pendingGlyph.Foreground(appTheme.FaintText)
		s.canceledGlyph = s.canceledGlyph.Foreground(appTheme.WarningText)
		s.keyStyle = s.keyStyle.Background(appTheme.SelectedBackground).Foreground(appTheme.PrimaryText)
	}

	return s
}

func makePill(text string, textStyle lipgloss.Style, bg color.Color) string {
	sBg := lipgloss.NewStyle().Foreground(bg)
	sFg := lipgloss.NewStyle().Inherit(textStyle).Background(bg)
	return lipgloss.JoinHorizontal(lipgloss.Top, sBg.Render(""), sFg.Render(text), sBg.Render(""))
}

func makePointingBorder(old string) string {
	return strings.Replace(old, lipgloss.NormalBorder().Right, lipgloss.RoundedBorder().TopLeft, 1)
}

func NewClockSpinner(styles styles) spinner.Model {
	return spinner.New(
		spinner.WithSpinner(MoonSpinnerFrames),
		spinner.WithStyle(
			lipgloss.NewStyle().Width(1).Margin(0).Padding(0).Foreground(styles.colors.warnColor),
		),
	)
}
