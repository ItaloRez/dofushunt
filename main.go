package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	g "github.com/AllenDang/giu"
	"github.com/go-vgo/robotgo"
)

func DecodeEmbedded(data []byte) (*image.RGBA, error) {
	r := bytes.NewReader(data)
	img, err := png.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("LoadImage: error decoding png image: %w", err)
	}
	return g.ImageToRgba(img), nil
}

//go:embed winres/splash.png
var splashHeaderLogo []byte

func DecodeSplashHeaderLogo() (*image.RGBA, error) {
	return DecodeEmbedded(splashHeaderLogo)
}

//go:embed winres/icon16.png
var appIcon16 []byte

func DecodeAppIcon16() (*image.RGBA, error) {
	return DecodeEmbedded(appIcon16)
}

//go:embed winres/icon.png
var appIcon []byte

func DecodeAppIcon() (*image.RGBA, error) {
	return DecodeEmbedded(appIcon)
}

const (
	SELECTED_CLUE_RESET       = "[SET Position -> Direction]"
	SELECTED_CLUE_TRAVELED    = "[Choose NEXT -> Direction]"
	SELECTED_CLUE_POS_CHANGED = "[Position Changed -> Set Direction]"
	SELECTED_CLUE_NOTFOUND    = "(X_x) No clues. You messed up"
)

var (
	curPosX            = int32(0)
	curPosY            = int32(0)
	curDir             = ClueDirectionNone
	curClues           = []string{}
	curFilteredClues   = []string{}
	curSelectedClue    = SELECTED_CLUE_RESET
	canConfirm         = false
	curResultSet       = ClueResultSet{}
	lastPosX           = curPosX
	lastPosY           = curPosY
	rgbaIcon16         *image.RGBA
	rgbaIcon           *image.RGBA
	headerSplashRgba   *image.RGBA
	splashTexture      = &g.ReflectiveBoundTexture{}
	icon16Texture      = &g.ReflectiveBoundTexture{}
	pricePreviewTexture = &g.ReflectiveBoundTexture{}
	curSelectedIndex   = int32(-1)
	filterText         = ""
	wnd                *g.MasterWindow
	isMovingFrame      = false
	language           = "fr"
	initialized        = false
	shouldFilterFocus  = false
	shouldListboxFocus = false
	itemsToScan        = ""
	scanResults        = []ScanResult{}
	isScanning         = false
)

type ScanResult struct {
	Name   string
	Prices []PriceTier
}

func framelessWindowMoveWidget(widget g.Widget) *g.CustomWidget {
	return g.Custom(func() {
		if isMovingFrame && !g.IsMouseDown(g.MouseButtonLeft) {
			isMovingFrame = false
			return
		}

		widget.Build()

		if g.IsItemHovered() {
			if g.IsMouseDown(g.MouseButtonLeft) {
				isMovingFrame = true
			}
		}

		if isMovingFrame {
			delta := imgui.CurrentIO().MouseDelta()
			dx := int(delta.X)
			dy := int(delta.Y)
			if dx != 0 || dy != 0 {
				ox, oy := wnd.GetPos()
				wnd.SetPos(ox+dx, oy+dy)
			}
		}
	})
}

func titleBarLayout() *g.CustomWidget {
	return framelessWindowMoveWidget(g.Custom(func() {
		icon16Texture.ToImageWidget().Scale(0.75, 0.75).Build()
		imgui.SameLine()
		imgui.PushStyleVarVec2(imgui.StyleVarSeparatorTextAlign, imgui.Vec2{0.0, 1.0})
		imgui.PushStyleVarVec2(imgui.StyleVarSeparatorTextPadding, imgui.Vec2{20.0, 2.0})
		imgui.SeparatorText("DofHunt")
		imgui.PopStyleVarV(2)
	}))
}

func langSetupLayout() *g.RowWidget {
	return g.Row(g.Custom(func() {
		g.Dummy(-1, 5).Build()
		imgui.PushStyleVarVec2(imgui.StyleVarSelectableTextAlign, imgui.Vec2{0.5, 0.0})
		g.ListBox(AppSupportedLanguages.Langs()).Size(-1, 100).SelectedIndex(AppSupportedLanguages.SelectedIndex()).OnChange(func(idx int) {
			langs := AppSupportedLanguages.Langs()
			GetDatas(AppSupportedLanguages.CountryCode(langs[idx]))
			initialized = true
		}).Build()
		imgui.PopStyleVar()
	},
	))
}

func onChange() {
	shouldListboxFocus = true
}

func headerLayout() *g.RowWidget {
	return g.Row(g.Custom(func() {
		imgui.PushItemWidth(40.0)
		g.DragInt("X", &curPosX, -100, 150).Build()
		imgui.SameLine()
		g.DragInt("Y", &curPosY, -100, 150).Build()
		imgui.PopItemWidth()
		imgui.SameLine()
		if shouldFilterFocus {
			g.SetKeyboardFocusHere()
			shouldFilterFocus = false
		}
		g.InputText(&filterText).Flags(g.InputTextFlagsEnterReturnsTrue).OnChange(onChange).Build()
		filterClues(&filterText)
	},
	),
	)
}

func filterClues(filter *string) {
	if *filter != "" && len(curClues) > 0 {
		curFilteredClues = []string{}
		for _, clue := range curClues {
			if strings.Contains(strings.ToLower(clue), NormalizeString(language, *filter, true)) {
				curFilteredClues = append(curFilteredClues, clue)
			}
		}
	} else {
		curFilteredClues = curClues
	}
}

func loop() {
	if calibActive {
		calibratorLoop()
		return
	}

	imgui.PushStyleVarVec2(imgui.StyleVarCellPadding, imgui.Vec2{1.0, 1.0})
	imgui.PushStyleVarVec2(imgui.StyleVarSeparatorTextAlign, imgui.Vec2{1.0, 1.0})
	imgui.PushStyleVarVec2(imgui.StyleVarSeparatorTextPadding, imgui.Vec2{20.0, 0.0})
	imgui.PushStyleVarFloat(imgui.StyleVarWindowBorderSize, 0)
	imgui.PushStyleVarFloat(imgui.StyleVarWindowRounding, 6.0)
	imgui.PushStyleVarFloat(imgui.StyleVarChildBorderSize, 0)
	imgui.PushStyleColorVec4(imgui.ColChildBg, g.ToVec4Color(color.RGBA{50, 50, 70, 0}))
	imgui.PushStyleColorVec4(imgui.ColButton, g.ToVec4Color(color.RGBA{50, 50, 70, 130}))
	g.PushColorWindowBg(color.RGBA{50, 50, 70, 130})
	g.PushColorFrameBg(color.RGBA{30, 30, 60, 110})
	if !initialized {
		g.SingleWindow().Layout(
			g.Dummy(-1, 5),
			framelessWindowMoveWidget(splashTexture.ToImageWidget()),
			g.Custom(func() {
				imgui.SeparatorText("Hunt Smarter")
			}),
			langSetupLayout(),
		)
	} else {
		g.SingleWindow().Flags(
			g.WindowFlags(imgui.WindowFlagsNoTitleBar)|
				g.WindowFlags(imgui.WindowFlagsNoCollapse)|
				g.WindowFlags(imgui.WindowFlagsNoMove)|
				g.WindowFlags(imgui.WindowFlagsNoResize)|
				g.WindowFlags(imgui.WindowFlagsNoNav),
		).Layout(
			titleBarLayout(),
			g.TabBar().TabItems(
				g.TabItem("Hunt").Layout(
					headerLayout(),
					g.Row(
						g.Child().Flags(g.WindowFlagsNoNav).Size(115, 100).Layout(
							g.Row(g.Custom(func() {
								g.Dummy(22.0, 0).Build()
								if curDir != ClueDirectionUp {
									imgui.SameLine()
									g.ArrowButton(g.DirectionUp).OnClick(func() {
										curDir = ClueDirectionUp
										UpdateClues()
									}).Build()
								} else {
									g.Label("").Build()
								}
							})),
							g.Row(g.Custom(func() {
								if curDir != ClueDirectionLeft {
									g.ArrowButton(g.DirectionLeft).OnClick(func() {
										curDir = ClueDirectionLeft
										UpdateClues()
									}).Build()
								} else {
									g.Dummy(22.0, 0).Build()
								}
								imgui.SameLine()
								if curDir != ClueDirectionNone {
									g.Button("    ").OnClick(func() {
										ResetClues(SELECTED_CLUE_RESET)
									}).Build()
								} else {
									g.Dummy(21.0, 0).Build()
								}
								imgui.SameLine()
								if curDir != ClueDirectionRight {
									g.ArrowButton(g.DirectionRight).OnClick(func() {
										curDir = ClueDirectionRight
										UpdateClues()
									}).Build()
								} else {
									g.Dummy(21.0, 0).Build()
								}
							})),
							g.Row(g.Custom(func() {
								g.Dummy(22.0, 0).Build()
								if curDir != ClueDirectionDown {
									imgui.SameLine()
									g.ArrowButton(g.DirectionDown).OnClick(func() {
										curDir = ClueDirectionDown
										UpdateClues()
									}).Build()
								} else {
									g.Label("").Build()
								}
							})),
							g.Row(g.Custom(func() {
								if canConfirm {
									g.Button("Confirm Clue").OnClick(TravelNextClue).Build()
								} else {
									g.Label("").Build()
								}
							})),
						),
						g.Custom(func() {
							if shouldListboxFocus {
								imgui.SetNextWindowFocus()
								shouldListboxFocus = false
							} else {
								if g.IsKeyPressed(g.KeyEscape) {
									shouldFilterFocus = true
								}
							}
							onChange := func(selectedIndex int) {
								if g.IsKeyPressed(g.KeyEnter) {
									curSelectedIndex = int32(selectedIndex)
									if len(curFilteredClues) > int(selectedIndex) {
										curSelectedClue = curFilteredClues[selectedIndex]
										TravelNextClue()
									}
								}
							}
							onDclick := func(selectedIndex int) {
								curSelectedIndex = int32(selectedIndex)
								if len(curFilteredClues) > int(selectedIndex) {
									curSelectedClue = curFilteredClues[selectedIndex]
									TravelNextClue()
								}
							}
							g.ListBox(curFilteredClues).Size(-1, 100).OnChange(onChange).SelectedIndex(&curSelectedIndex).OnDClick(onDclick).Build()
							if int(curSelectedIndex) >= 0 && len(curFilteredClues) > int(curSelectedIndex) {
								curSelectedClue = curFilteredClues[curSelectedIndex]
							} else {
								curSelectedIndex = -1
							}
						}),
					),
					g.Row(g.Custom(func() {
						imgui.PushStyleVarVec2(imgui.StyleVarSeparatorTextAlign, imgui.Vec2{1.0, 1.0})
						imgui.PushStyleVarVec2(imgui.StyleVarSeparatorTextPadding, imgui.Vec2{20.0, 0.0})
						imgui.SeparatorText("History")
						imgui.PopStyleVarV(2)
					})),
					g.Custom(func() {
						if len(TravelHistory.GetEntries()) > 0 {
							TravelHistory.Table().Build()
						}
					}),
				),
				g.TabItem("Market").Layout(
					g.Label("Items (one per line):"),
					g.InputTextMultiline(&itemsToScan).Size(-1, 80),
					g.Row(
						g.Button("Calibrate Search").OnClick(func() { go GlobalScanner.CalibrateSearchBar() }),
						g.Button("Calibrate 1st Result").OnClick(func() { go GlobalScanner.CalibrateFirstResult() }),
						g.Button("Calibrate 2nd Result").OnClick(func() { go GlobalScanner.CalibrateSecondResult() }),
					),
					g.Row(
						g.Button("Calibrate Price").OnClick(func() { go StartPriceCalibration() }),
						g.Button("Calibrate Name").OnClick(func() { go StartItemNameCalibration() }),
					),
					g.Row(
						g.Button("Abrir Permissões de Acessibilidade").OnClick(func() {
							exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Run()
						}),
						g.Button("Abrir Permissões de Gravação de Tela").OnClick(func() {
							exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture").Run()
						}),
					),
					g.Row(
						g.Button("Start Scan").Disabled(isScanning || !GlobalScanner.IsCalibrated).OnClick(func() {
							go startMarketScan()
						}),
						g.Button("Clear Results").OnClick(func() {
							scanResults = []ScanResult{}
						}),
					),
					g.Custom(func() {
						// Preview da imagem capturada
						if GlobalScanner.IsCalibrated {
							imgui.SeparatorText("Preview da Captura")
							pricePreviewTexture.ToImageWidget().Scale(1.0, 1.0).Build()
						}
					}),
					g.Custom(func() {
						if len(scanResults) > 0 {
							imgui.SeparatorText("Results")
							for _, res := range scanResults {
								g.Label(res.Name).Build()
								for _, t := range res.Prices {
									g.Label(fmt.Sprintf("  x%-6d %d kamas", t.Qty, t.Price)).Build()
								}
							}
						}
					}),
				),
			),
		)
	}
	g.PopStyleColor()
	g.PopStyleColor()
	imgui.PopStyleVar()
	imgui.PopStyleVar()
	imgui.PopStyleVar()
	imgui.PopStyleVar()
	imgui.PopStyleVar()
	imgui.PopStyleVar()
	imgui.PopStyleColor()
	imgui.PopStyleColor()

	if lastPosX != curPosX || lastPosY != curPosY {
		ResetClues(SELECTED_CLUE_POS_CHANGED)
	}
	lastPosX = curPosX
	lastPosY = curPosY
}

func UpdateClues() {
	curResultSet = getClueResultSet(MapPosition{
		X: int(curPosX),
		Y: int(curPosY),
	}, curDir, 10)
	curClues = curResultSet.Pois()
	if len(curClues) > 0 {
		shouldFilterFocus = true
		curSelectedClue = curClues[0]
		canConfirm = true
	} else {
		curSelectedClue = SELECTED_CLUE_NOTFOUND
		canConfirm = false
	}
}

func ResetClues(message string) {
	curDir = ClueDirectionNone
	curClues = []string{}
	curSelectedClue = message
	curResultSet = ClueResultSet{}
	canConfirm = false
}

func TravelNextClue() {
	poi := curSelectedClue
	pos, err := curResultSet.Pos(poi)
	if err != nil {
		log.Println(err)
		return
	}
	travel := pos.TravelCommand()

	// imgui.LogToClipboard()
	// imgui.LogText(travel)
	// imgui.LogFinish()

	TravelHistory.AddEntry(MapPosition{
		X: int(curPosX),
		Y: int(curPosY),
	}, curDir, curSelectedClue, MapPosition{
		X: pos.X,
		Y: pos.Y,
	})
	curPosX = int32(pos.X)
	curPosY = int32(pos.Y)
	filterText = ""
	ResetClues(SELECTED_CLUE_TRAVELED)

	// Digita a pista no chat do Dofus
	fpid, err := robotgo.FindIds("Dofus")
	if err != nil {
		fmt.Println(err)
		return
	}
	robotgo.ActivePid(fpid[0])
	time.Sleep(500 * time.Millisecond)

	// Simula pressionar a tecla "Espaço"
	robotgo.KeyTap("space")
	time.Sleep(500 * time.Millisecond)

	// Digita o texto
	robotgo.TypeStr(travel)
	time.Sleep(500 * time.Millisecond)
	log.Println("Digitou o texto")

	// Pressiona Enter duas vezes com atraso
	time.Sleep(500 * time.Millisecond)
	robotgo.KeyTap("enter")
	time.Sleep(500 * time.Millisecond)
	robotgo.KeyTap("enter")
}

func startMarketScan() {
	if isScanning {
		return
	}
	isScanning = true
	defer func() { isScanning = false }()

	lines := strings.Split(itemsToScan, "\n")
	for _, line := range lines {
		searched := strings.TrimSpace(line)
		if searched == "" {
			continue
		}

		GlobalScanner.SearchItem(searched)
		GlobalScanner.ClickFirstResult()
		scanResult := captureResult(searched)
		if scanResult == nil {
			continue
		}
		scanResults = append(scanResults, *scanResult)
		g.Update()

		// Se o nome encontrado não bate com o buscado e há segundo resultado calibrado,
		// captura o segundo item também.
		if GlobalScanner.HasSecondResult && GlobalScanner.HasNameCalib &&
			len(scanResult.Prices) > 0 && !namesMatch(searched, scanResult.Name) {
			GlobalScanner.ClickSecondResult()
			scanResult2 := captureResult(searched)
			if scanResult2 != nil {
				scanResults = append(scanResults, *scanResult2)
				g.Update()
			}
		}
	}
}

// captureResult lê nome e todos os tiers de preço do item atualmente selecionado.
// O parâmetro searched é usado como fallback de nome quando o OCR de nome não está calibrado.
// Tenta até 2 vezes clicar no item caso o nome não apareça.
func captureResult(searched string) *ScanResult {
	capturedName := searched
	if GlobalScanner.HasNameCalib {
		const maxAttempts = 2
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			n, err := GlobalScanner.CaptureItemName()
			if err != nil {
				log.Printf("Attempt %d: error capturing name for %s: %v", attempt, searched, err)
			} else if n != "" {
				capturedName = n
				break
			}
			if attempt < maxAttempts {
				log.Printf("Attempt %d: name empty, tentando segundo resultado...", attempt)
				if GlobalScanner.HasSecondResult {
					GlobalScanner.ClickSecondResult()
				} else {
					GlobalScanner.ClickFirstResult()
				}
			}
		}
	}

	tiers, err := GlobalScanner.CapturePrices()
	if err != nil {
		log.Printf("Error scanning %s: %v", searched, err)
		return nil
	}

	return &ScanResult{Name: capturedName, Prices: tiers}
}

// namesMatch compara dois nomes ignorando maiúsculas/minúsculas e espaços extras.
func namesMatch(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func main() {
	rand.Seed(time.Now().UnixNano())
	LoadConfig() // Carrega calibrações salvas
	wnd = g.NewMasterWindow("DofHunt", 380, 263, g.MasterWindowFlagsNotResizable|g.MasterWindowFlagsFrameless|g.MasterWindowFlagsFloating|g.MasterWindowFlagsTransparent)
	wnd.SetTargetFPS(60)
	wnd.SetBgColor(color.RGBA{0, 0, 0, 0})
	rgbaIcon, _ = DecodeAppIcon()
	rgbaIcon16, _ = DecodeAppIcon16()
	headerSplashRgba, _ := DecodeSplashHeaderLogo()
	splashTexture.SetSurfaceFromRGBA(headerSplashRgba, false)
	icon16Texture.SetSurfaceFromRGBA(rgbaIcon16, false)
	wnd.SetPos(300, 300)
	wnd.Run(loop)
}
