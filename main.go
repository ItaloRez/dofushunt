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
	"strings"
	"sync/atomic"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	g "github.com/AllenDang/giu"
	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
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
	scanStopRequested  atomic.Bool
	dbServer           = ""
	dbServerIndex      = int32(-1)
)

func requestStopMarketScan(source string) {
	if scanStopRequested.CompareAndSwap(false, true) {
		log.Printf("Scan: parada solicitada via %s", source)
		g.Update()
	}
}

func shouldStopMarketScan() bool {
	return scanStopRequested.Load()
}

func setupMarketStopHotkey() func() {
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			if hook.AddEvent("q") {
				requestStopMarketScan("tecla Q")
			}
		}
	}()

	log.Println("Hotkey global Q registrada para parar automação")
	return func() {
		close(stop)
		hook.StopEvent()
	}
}

type ScanResult struct {
	Name   string
	Prices []PriceTier
}

func framelessWindowMoveWidget(widget g.Widget) *g.CustomWidget {
	return g.Custom(func() {
		if isMovingFrame && !g.IsMouseDown(g.MouseButtonLeft) {
			isMovingFrame = false
			SaveConfig()
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
					g.InputTextMultiline(&itemsToScan).Size(-1, 60),
					g.Row(
						g.Label("Servidor:"),
						g.Combo("##server", dbServer, ServerList, &dbServerIndex).
							Size(180).
							OnChange(func() {
								if int(dbServerIndex) >= 0 && int(dbServerIndex) < len(ServerList) {
									dbServer = ServerName(ServerList[dbServerIndex])
									SaveConfig()
								}
							}),
					),
					g.Row(
						g.Button("Calibrar").Disabled(calibActive).OnClick(func() {
							go StartFullCalibration()
						}),
						g.Button(func() string {
							if isScanning {
								if shouldStopMarketScan() {
									return "Stopping..."
								}
								return "Stop Scan"
							}
							return "Start Scan"
						}()).Disabled(!isScanning && !GlobalScanner.IsCalibrated).OnClick(func() {
							if isScanning {
								requestStopMarketScan("botão Stop Scan")
								return
							}
							go startMarketScan()
						}),
						g.Button("Limpar").OnClick(func() {
							scanResults = []ScanResult{}
						}),
					),
					g.Custom(func() {
						if GlobalScanner.IsCalibrated {
							imgui.SeparatorText("Preview")
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

// calibStatusLabel exibe um label colorido indicando se um passo está calibrado.
func calibStatusLabel(name string, ok bool) {
	if ok {
		imgui.PushStyleColorVec4(imgui.ColText, imgui.Vec4{X: 0.3, Y: 1.0, Z: 0.3, W: 1})
		imgui.Text("✓ " + name)
	} else {
		imgui.PushStyleColorVec4(imgui.ColText, imgui.Vec4{X: 1.0, Y: 0.4, Z: 0.4, W: 1})
		imgui.Text("✗ " + name)
	}
	imgui.PopStyleColor()
}

func startMarketScan() {
	if isScanning {
		return
	}
	scanStopRequested.Store(false)
	isScanning = true
	defer func() {
		isScanning = false
		scanStopRequested.Store(false)
		g.Update()
	}()

	lines := strings.Split(itemsToScan, "\n")

	// Monta fila inicial, ignorando linhas vazias.
	var queue []string
	for _, line := range lines {
		if s := strings.TrimSpace(line); s != "" {
			queue = append(queue, s)
		}
	}

	// retryQueue acumula itens que falharam; processados uma única vez no final.
	var retryQueue []string
	isRetryPass := false

	processItem := func(searched, prevName string) (result *ScanResult, failed bool) {
		if shouldStopMarketScan() {
			return nil, false
		}
		GlobalScanner.SearchItem(searched)
		if shouldStopMarketScan() {
			return nil, false
		}
		GlobalScanner.ClickFirstResult()
		scanResult := captureResult(searched, prevName, GlobalScanner.ClickFirstResult)
		if scanResult == nil {
			return nil, true
		}

		// Verifica se o nome capturado bate com o buscado.
		nameOk := !GlobalScanner.HasNameCalib || namesMatch(searched, scanResult.Name)

		// Tenta 2º resultado se nome não bate.
		if !nameOk && GlobalScanner.HasSecondResult && GlobalScanner.HasNameCalib {
			if shouldStopMarketScan() {
				return nil, false
			}
			GlobalScanner.ClickSecondResult()
			sr2 := captureResult(searched, scanResult.Name, GlobalScanner.ClickSecondResult)
			if sr2 != nil {
				nameOk = namesMatch(searched, sr2.Name)
				scanResult = sr2
			}
		}

		// Tenta 3º resultado se ainda não bateu.
		if !nameOk && GlobalScanner.HasThirdResult && GlobalScanner.HasNameCalib {
			if shouldStopMarketScan() {
				return nil, false
			}
			GlobalScanner.ClickThirdResult()
			sr3 := captureResult(searched, scanResult.Name, GlobalScanner.ClickThirdResult)
			if sr3 != nil {
				nameOk = namesMatch(searched, sr3.Name)
				scanResult = sr3
			}
		}

		// Se nenhum resultado bateu e ainda não é a passagem de retry, agenda para tentar depois.
		if !nameOk && !isRetryPass {
			log.Printf("Scan: '%s' não encontrado, agendando para retry", searched)
			return nil, true
		}

		return scanResult, false
	}

	prevName := ""
	for _, searched := range queue {
		if shouldStopMarketScan() {
			log.Println("Scan: interrompido antes de concluir a fila principal")
			return
		}
		result, failed := processItem(searched, prevName)
		if shouldStopMarketScan() {
			log.Println("Scan: interrompido durante a fila principal")
			return
		}
		if failed {
			retryQueue = append(retryQueue, searched)
			continue
		}
		scanResults = append(scanResults, *result)
		prevName = result.Name
		go SavePricesToDB(dbServer, result.Name, result.Prices)
		g.Update()
	}

	// Passagem de retry — uma única vez, sem reenfileirar.
	if len(retryQueue) > 0 {
		isRetryPass = true
		log.Printf("Scan: iniciando retry de %d item(ns): %v", len(retryQueue), retryQueue)
		for _, searched := range retryQueue {
			if shouldStopMarketScan() {
				log.Println("Scan: interrompido antes de concluir a fila de retry")
				return
			}
			result, _ := processItem(searched, prevName)
			if shouldStopMarketScan() {
				log.Println("Scan: interrompido durante a fila de retry")
				return
			}
			if result == nil {
				log.Printf("Scan: retry falhou para '%s', pulando", searched)
				continue
			}
			scanResults = append(scanResults, *result)
			prevName = result.Name
			go SavePricesToDB(dbServer, result.Name, result.Prices)
			g.Update()
		}
	}
}

// captureResult lê nome e todos os tiers de preço do item atualmente selecionado.
// prevName: nome do item anterior — se o nome capturado for igual, considera inválido e retenta.
// retryClick: função a chamar para reabrir o item no retry (ClickFirstResult ou ClickSecondResult).
func captureResult(searched, prevName string, retryClick func()) *ScanResult {
	if shouldStopMarketScan() {
		return nil
	}
	capturedName := searched
	if GlobalScanner.HasNameCalib {
		const maxAttempts = 2
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if shouldStopMarketScan() {
				return nil
			}
			n, err := GlobalScanner.CaptureItemName()
			// Para comparar com o nome anterior, usamos o match estrito (normalizado)
			// para evitar que itens parecidos mas diferentes fiquem presos num loop de retry.
			nameOk := err == nil && n != "" && !namesMatchStrict(n, prevName)
			if nameOk {
				capturedName = n
				break
			}
			if attempt < maxAttempts {
				if err != nil {
					log.Printf("Attempt %d: error capturing name for %s: %v", attempt, searched, err)
				} else {
					log.Printf("Attempt %d: name '%s' inválido (vazio ou igual ao anterior '%s'), fechando e reabrindo", attempt, n, prevName)
				}
				if GlobalScanner.HasCloseItem {
					GlobalScanner.ClickCloseItem()
				}
				// Move levemente em direção aleatória antes de tentar novamente
				nudgeAndRetry(retryClick)
			}
		}
	}

	if shouldStopMarketScan() {
		return nil
	}
	tiers, err := GlobalScanner.CapturePrices()
	if err != nil {
		log.Printf("Error scanning %s: %v", searched, err)
		return nil
	}

	return &ScanResult{Name: capturedName, Prices: tiers}
}

// namesMatch compara dois nomes ignorando maiúsculas/minúsculas, acentos e permite até 2 letras de diferença.
func namesMatch(a, b string) bool {
	normA := NormalizeString(language, strings.TrimSpace(a), true)
	normB := NormalizeString(language, strings.TrimSpace(b), true)
	if normA == normB {
		return true
	}
	return levenshtein(normA, normB) <= 2
}

// namesMatchStrict compara se dois nomes são idênticos após normalização (acentos e case).
func namesMatchStrict(a, b string) bool {
	return NormalizeString(language, strings.TrimSpace(a), true) == NormalizeString(language, strings.TrimSpace(b), true)
}

func levenshtein(s, t string) int {
	if s == t {
		return 0
	}
	if len(s) == 0 {
		return len(t)
	}
	if len(t) == 0 {
		return len(s)
	}

	r1 := []rune(s)
	r2 := []rune(t)
	len1 := len(r1)
	len2 := len(r2)

	column := make([]int, len1+1)
	for i := 0; i <= len1; i++ {
		column[i] = i
	}

	for j := 1; j <= len2; j++ {
		column[0] = j
		lastdiag := j - 1
		for i := 1; i <= len1; i++ {
			oldcolumni := column[i]
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			column[i] = min(column[i]+1, min(column[i-1]+1, lastdiag+cost))
			lastdiag = oldcolumni
		}
	}
	return column[len1]
}

// nudgeAndRetry move o mouse alguns pixels em direção aleatória (cima/baixo/esquerda/direita)
// antes de executar o clique de retry, simulando uma tentativa humana de reposicionar.
func nudgeAndRetry(retryClick func()) {
	cx, cy := robotgo.GetMousePos()
	drift := 4 + rand.Intn(8) // 4–11 pixels
	dirs := [][2]int{{drift, 0}, {-drift, 0}, {0, drift}, {0, -drift}}
	d := dirs[rand.Intn(4)]
	MoveHumanLike(cx+d[0], cy+d[1])
	time.Sleep(time.Duration(randRange(80, 200)) * time.Millisecond)
	retryClick()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	LoadConfig() // Carrega calibrações salvas
	unregisterStopHotkey := setupMarketStopHotkey()
	defer unregisterStopHotkey()
	wnd = g.NewMasterWindow("DofHunt", 380, 263, g.MasterWindowFlagsNotResizable|g.MasterWindowFlagsFrameless|g.MasterWindowFlagsFloating|g.MasterWindowFlagsTransparent)
	wnd.SetTargetFPS(60)
	wnd.SetBgColor(color.RGBA{0, 0, 0, 0})
	rgbaIcon, _ = DecodeAppIcon()
	rgbaIcon16, _ = DecodeAppIcon16()
	headerSplashRgba, _ := DecodeSplashHeaderLogo()
	splashTexture.SetSurfaceFromRGBA(headerSplashRgba, false)
	icon16Texture.SetSurfaceFromRGBA(rgbaIcon16, false)
	if x, y := savedWindowPos(); x != 0 || y != 0 {
		wnd.SetPos(x, y)
	} else {
		wnd.SetPos(300, 300)
	}
	wnd.Run(loop)
}
