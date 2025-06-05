package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Matrix Theme Colors (Lipgloss)
var (
	matrixPrimary   = lipgloss.NewStyle().Foreground(lipgloss.Color("90"))  // Bright Green
	matrixSecondary = lipgloss.NewStyle().Foreground(lipgloss.Color("28"))  // Green
	matrixDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("22"))  // Dim Green
	matrixAccent    = lipgloss.NewStyle().Foreground(lipgloss.Color("80"))  // Bright Cyan
	matrixError     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Bright Red
	matrixBorder    = lipgloss.NewStyle().Foreground(lipgloss.Color("28"))  // Green
)

const NEO_ASCII = `
███╗   ██╗███████╗ ██████╗
████╗  ██║██╔════╝██╔═══██╗
██╔██╗ ██║█████╗  ██║   ██║
██║╚██╗██║██╔══╝  ██║   ██║
██║ ╚████║███████╗╚██████╔╝
╚═╝  ╚═══╝╚══════╝ ╚═════╝
`

var MATRIX_QUOTES = []string{
	"Welcome to the real world...",
	"There is no spoon.",
	"Follow the white rabbit.",
	"The Matrix has you...",
	"Wake up, Neo...",
	"I know kung fu.",
	"Free your mind.",
}

// Matrix Rain Characters (subset for simplicity)
const MATRIX_CHARS = "ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

type MatrixRainDrop struct {
	char     rune
	x, y     int
	speed    float64
	isBright bool
	life     int
}

type MatrixRain struct {
	width, height int
	drops         []*MatrixRainDrop
	consoleWidth  int
}

func NewMatrixRain(width, height int) *MatrixRain {
	consoleW := 80
	if width > 0 {
		consoleW = width
	}
	return &MatrixRain{width: consoleW, height: height, drops: []*MatrixRainDrop{}}
}

func (mr *MatrixRain) Update() {
	if len(mr.drops) < (mr.width*mr.height)/15 { // Control density
		for i := 0; i < mr.width/5; i++ { // Spawn a few drops per frame
			if rand.Float32() < 0.3 { // Chance to spawn
				xPos := rand.Intn(mr.width)
				canSpawn := true
				for _, drop := range mr.drops {
					if drop.x == xPos && drop.y == 0 {
						canSpawn = false
						break
					}
				}
				if canSpawn {
					mr.drops = append(mr.drops, &MatrixRainDrop{
						char:     rune(MATRIX_CHARS[rand.Intn(len(MATRIX_CHARS))]),
						x:        xPos,
						y:        0,
						speed:    rand.Float64()*0.5 + 0.3, // Slower speeds
						isBright: true,
						life:     rand.Intn(mr.height/2) + mr.height/2, // Persist for a while
					})
				}
			}
		}
	}

	newDrops := []*MatrixRainDrop{}
	for _, drop := range mr.drops {
		drop.y += int(drop.speed)
		drop.life--

		if drop.y < mr.height && drop.life > 0 {
			if drop.isBright && drop.y > 0 {
				newDrops = append(newDrops, &MatrixRainDrop{
					char: drop.char,
					x:    drop.x, y: drop.y - 1, speed: 0, isBright: false, life: 2,
				})
			}
			drop.isBright = false
			drop.char = rune(MATRIX_CHARS[rand.Intn(len(MATRIX_CHARS))])
			newDrops = append(newDrops, drop)
		}
	}
	mr.drops = newDrops
}

func (mr *MatrixRain) Render() string {
	grid := make([][]rune, mr.height)
	styles := make([][]lipgloss.Style, mr.height)
	for i := range grid {
		grid[i] = make([]rune, mr.width)
		styles[i] = make([]lipgloss.Style, mr.width)
		for j := range grid[i] {
			grid[i][j] = ' '
			styles[i][j] = lipgloss.NewStyle().SetString(" ")
		}
	}

	for _, drop := range mr.drops {
		if drop.y >= 0 && drop.y < mr.height && drop.x >= 0 && drop.x < mr.width {
			grid[drop.y][drop.x] = drop.char
			if drop.isBright {
				styles[drop.y][drop.x] = matrixPrimary.Copy().SetString(string(drop.char))
			} else {
				styles[drop.y][drop.x] = matrixDim.Copy().SetString(string(drop.char))
			}
		}
	}

	var b strings.Builder
	for i := 0; i < mr.height; i++ {
		for j := 0; j < mr.width; j++ {
			b.WriteString(styles[i][j].String())
		}
		b.WriteString("\n")
	}
	return b.String()
}

func DisplayMatrixExit() {
	fmt.Println(matrixDim.Render("\n> Exiting the Matrix..."))
	rain := NewMatrixRain(80, 10) // Shorter rain for exit
	for i := 0; i < 20; i++ {     // Shorter animation
		ClearScreen()
		rain.Update()
		fmt.Print(rain.Render())
		fmt.Println(matrixDim.Render("\n> Exiting the Matrix...")) // Keep message visible
		time.Sleep(120 * time.Millisecond)
	}
	ClearScreen()
	fmt.Println(matrixPrimary.Render("> Remember... there is no spoon."))
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func ClearScreen() {
	fmt.Print("[H[2J")
}

func DisplayInitialScreen() {
	ClearScreen()
	rain := NewMatrixRain(80, 15)
	for i := 0; i < 30; i++ {
		ClearScreen()
		rain.Update()
		fmt.Print(rain.Render())
		time.Sleep(100 * time.Millisecond)
	}
	ClearScreen()
	fmt.Println()

	fmt.Println(matrixPrimary.Render(NEO_ASCII))

	quote := MATRIX_QUOTES[rand.Intn(len(MATRIX_QUOTES))]
	fmt.Println(matrixDim.Italic(true).Render(centerText(quote, 80)))
	fmt.Println()

	infoContent := []string{
		matrixPrimary.Render("SYSTEM: NEO v1.0 (Go Edition)"),
		matrixSecondary.Render("STATUS: ONLINE"),
		matrixDim.Render("REALITY: SIMULATED"),
	}
	infoBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(matrixBorder.GetForeground()).
		PaddingLeft(2).PaddingRight(2).
		Width(40).
		Align(lipgloss.Center).
		Render(strings.Join(infoContent, "\n"))

	fmt.Println(lipgloss.PlaceHorizontal(80, lipgloss.Center, infoBox))
	fmt.Println()
	fmt.Println(matrixDim.Render(centerText("COMMANDS: /add <path> | /clear | /exit | /red_pill | /blue_pill", 80)))
	fmt.Println()
}

func centerText(text string, width int) string {
	if len(text) >= width {
		return text
	}
	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text
}

var PromptPrefix = matrixPrimary.Render("neo") + "@" + matrixSecondary.Render("matrix") + ":" + matrixDim.Render("~$ ")
var PromptUser = matrixPrimary.Render("You> ")

func FormatAIResponseChunk(chunk string, inCodeBlock bool) string {
	if inCodeBlock {
		return matrixPrimary.Foreground(lipgloss.Color("lightgreen")).Render(chunk)
	}
	return matrixPrimary.Render(chunk)
}

type MatrixTextStreamFormatter struct {
	buffer       string
	inCodeBlock  bool
	codeLanguage string
}

func NewMatrixTextStreamFormatter() *MatrixTextStreamFormatter {
	return &MatrixTextStreamFormatter{}
}

func (f *MatrixTextStreamFormatter) ProcessChunk(chunk string) {
	f.buffer += chunk
	lines := strings.Split(f.buffer, "\n")

	for i, line := range lines[:len(lines)-1] {
		f.formatAndPrintLine(line)
		if i < len(lines)-2 {
		}
	}
	f.buffer = lines[len(lines)-1]
}

func (f *MatrixTextStreamFormatter) formatAndPrintLine(line string) {
	trimmedLine := strings.TrimSpace(line)

	if strings.HasPrefix(trimmedLine, "```") {
		if !f.inCodeBlock {
			f.inCodeBlock = true
			f.codeLanguage = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "```"))
			if f.codeLanguage == "" {
				f.codeLanguage = "text"
			}
			fmt.Println(matrixAccent.Render(fmt.Sprintf("┌─ Code (%s) ─", f.codeLanguage)))
		} else {
			f.inCodeBlock = false
			fmt.Println(matrixAccent.Render("└─────────────────"))
		}
		return
	}

	if f.inCodeBlock {
		fmt.Println(matrixPrimary.Render(fmt.Sprintf("│ %s", line)))
	} else {
		if strings.HasPrefix(trimmedLine, "* ") || strings.HasPrefix(trimmedLine, "- ") {
			fmt.Println(fmt.Sprintf("%s %s", matrixAccent.Render("•"), matrixPrimary.Render(strings.TrimSpace(trimmedLine[2:]))))
		} else if len(trimmedLine) > 0 && strings.ContainsAny(trimmedLine[:2], "0123456789") && strings.HasPrefix(strings.TrimLeft(trimmedLine, "0123456789. "), "") && (strings.Contains(trimmedLine, ". ") || strings.Contains(trimmedLine, ") ")) {
			parts := strings.Fields(trimmedLine)
			if len(parts) > 0 {
				numPart := parts[0]
				restOfLine := strings.TrimSpace(strings.Join(parts[1:], " "))
				fmt.Println(fmt.Sprintf("%s %s", matrixAccent.Render(numPart), matrixPrimary.Render(restOfLine)))
			} else {
				fmt.Println(matrixPrimary.Render(trimmedLine))
			}
		} else if trimmedLine != "" {
			fmt.Println(matrixPrimary.Render(trimmedLine))
		} else {
			fmt.Println()
		}
	}
}

func (f *MatrixTextStreamFormatter) Finalize() {
	if f.buffer != "" {
		f.formatAndPrintLine(f.buffer)
		f.buffer = ""
	}
	if f.inCodeBlock {
		fmt.Println(matrixAccent.Render("└─────────────────"))
		f.inCodeBlock = false
	}
}

func PrintNeoResponsePrefix() {
	fmt.Print(matrixAccent.Render("NEO> "))
}
