package main

import (
	"fmt"
	"image"
	"log"

	g "github.com/AllenDang/giu"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"
	"golang.design/x/hotkey/mainthread"
)

var (
	calibActive       bool
	calibScreenTex    = &g.ReflectiveBoundTexture{}
	calibPhysW        int
	calibPhysH        int
	calibIsDragging   bool
	calibMouseWasDown bool
	calibDragStart    imgui.Vec2
	calibDragEnd      imgui.Vec2
	calibImgMin       imgui.Vec2
	calibImgMax       imgui.Vec2
	calibTitle        string
	calibStepNum      int
	calibTotalSteps   int
	calibIsPoint      bool // true = clique único (posição), false = arraste (área)
	calibOnDone       func(image.Rectangle)
	savedWndX         int
	savedWndY         int
	savedWndW         int
	savedWndH         int
)

func col32(r, gr, b, a uint8) uint32 {
	return uint32(r) | uint32(gr)<<8 | uint32(b)<<16 | uint32(a)<<24
}

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// StartCalibration inicia uma calibração de área (arrastar para selecionar).
func StartCalibration(title string, onDone func(image.Rectangle)) {
	startCalibBase(title, false, onDone)
}

// StartCalibrationPoint inicia uma calibração de ponto (clique único).
func StartCalibrationPoint(title string, onDone func(image.Point)) {
	startCalibBase(title, true, func(r image.Rectangle) {
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		onDone(image.Point{X: cx, Y: cy})
	})
}

func startCalibBase(title string, isPoint bool, onDone func(image.Rectangle)) {
	go func() {
		// Detecta o monitor onde o cursor está (provável monitor do Dofus)
		displayIdx := 0
		if n := screenshot.NumActiveDisplays(); n > 1 {
			mx, my := robotgo.GetMousePos()
			for i := 0; i < n; i++ {
				b := screenshot.GetDisplayBounds(i)
				if mx >= b.Min.X && mx < b.Max.X && my >= b.Min.Y && my < b.Max.Y {
					displayIdx = i
					break
				}
			}
		}

		bounds := screenshot.GetDisplayBounds(displayIdx)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			log.Printf("Falha ao capturar tela: %v", err)
			return
		}

		screenW, screenH := robotgo.GetScreenSize()
		log.Printf("Display %d | Tela lógica: %dx%d | Screenshot físico: %dx%d", displayIdx, screenW, screenH, bounds.Dx(), bounds.Dy())

		mainthread.Call(func() {
			// Salva posição/tamanho apenas no primeiro passo da sequência
			if !calibActive {
				savedWndX, savedWndY = wnd.GetPos()
				savedWndW, savedWndH = wnd.GetSize()
			}
			calibPhysW = bounds.Dx()
			calibPhysH = bounds.Dy()
			calibScreenTex.SetSurfaceFromRGBA(img, false)
			calibIsDragging = false
			calibMouseWasDown = false
			calibTitle = title
			calibIsPoint = isPoint
			calibOnDone = onDone
			calibActive = true
			wnd.SetPos(bounds.Min.X, bounds.Min.Y)
			wnd.SetSize(bounds.Dx(), bounds.Dy())
		})
		g.Update()
	}()
}

// StartFullCalibration encadeia todos os passos de calibração em sequência.
func StartFullCalibration() {
	calibStepNum = 0
	calibTotalSteps = 8
	runCalibStep1()
}

func runCalibStep1() {
	calibStepNum = 1
	StartCalibration("Barra de Busca — arraste sobre o campo de pesquisa inteiro", func(r image.Rectangle) {
		GlobalScanner.SearchBarRect = r
		GlobalScanner.HasSearchBar = true
		log.Printf("Busca calibrada: %v", r)
		SaveConfig()
		runCalibStep2()
	})
}

func runCalibStep2() {
	calibStepNum = 2
	StartCalibrationPoint("1º Resultado — clique sobre o primeiro item da lista", func(pt image.Point) {
		GlobalScanner.FirstResult = pt
		log.Printf("1º resultado calibrado: %v", pt)
		SaveConfig()
		runCalibStep3()
	})
}

func runCalibStep3() {
	calibStepNum = 3
	StartCalibrationPoint("2º Resultado — clique sobre o segundo item da lista", func(pt image.Point) {
		GlobalScanner.SecondResult = pt
		GlobalScanner.HasSecondResult = true
		log.Printf("2º resultado calibrado: %v", pt)
		SaveConfig()
		runCalibStep4()
	})
}

func runCalibStep4() {
	calibStepNum = 4
	StartCalibrationPoint("3º Resultado — clique sobre o terceiro item da lista", func(pt image.Point) {
		GlobalScanner.ThirdResult = pt
		GlobalScanner.HasThirdResult = true
		log.Printf("3º resultado calibrado: %v", pt)
		SaveConfig()
		runCalibStep5()
	})
}

func runCalibStep5() {
	calibStepNum = 5
	StartCalibrationPoint("Fechar Item — clique no botão de fechar o item", func(pt image.Point) {
		GlobalScanner.CloseItem = pt
		GlobalScanner.HasCloseItem = true
		log.Printf("Fechar item calibrado: %v", pt)
		SaveConfig()
		runCalibStep6()
	})
}

func runCalibStep6() {
	calibStepNum = 6
	StartCalibration("Coluna QUANTIDADE — arraste sobre a célula '1' (1ª linha)", func(r image.Rectangle) {
		GlobalScanner.QtyColRect = r
		log.Printf("Coluna quantidade calibrada: %v", r)
		runCalibStep7()
	})
}

func runCalibStep7() {
	calibStepNum = 7
	StartCalibration("Coluna PREÇO — arraste sobre o preço da 1ª linha", func(r image.Rectangle) {
		GlobalScanner.PriceColRect = r
		GlobalScanner.HasSplitCalib = true
		GlobalScanner.IsCalibrated = true
		log.Printf("Coluna preço calibrada: %v", r)
		SaveConfig()
		runCalibStep8()
	})
}

func runCalibStep8() {
	calibStepNum = 8
	StartCalibration("Nome do Item — arraste sobre a área do nome do item", func(r image.Rectangle) {
		GlobalScanner.ItemNameRect = r
		GlobalScanner.HasNameCalib = true
		log.Printf("Nome do item calibrado: %v", r)
		SaveConfig()
		log.Println("Calibração completa!")
	})
}

func StartPriceCalibration() {
	calibStepNum = 0
	calibTotalSteps = 2
	StartCalibration("coluna QUANTIDADE (ex: '1', '10'...)", func(qtyRect image.Rectangle) {
		GlobalScanner.QtyColRect = qtyRect
		log.Printf("Coluna de quantidade definida: %v", qtyRect)

		StartCalibration("coluna PREÇO (ex: '99 k', '244'...)", func(priceRect image.Rectangle) {
			GlobalScanner.PriceColRect = priceRect
			GlobalScanner.HasSplitCalib = true
			GlobalScanner.IsCalibrated = true
			SaveConfig()
			log.Printf("Coluna de preço definida: %v", priceRect)
			go func() {
				price, err := GlobalScanner.CapturePrice()
				if err != nil {
					log.Printf("Captura pós-calibração: %v", err)
				} else {
					log.Printf("Preço detectado: %d kamas", price)
				}
			}()
		})
	})
}

func StartItemNameCalibration() {
	calibStepNum = 0
	calibTotalSteps = 1
	StartCalibration("nome do item", func(r image.Rectangle) {
		GlobalScanner.ItemNameRect = r
		GlobalScanner.HasNameCalib = true
		SaveConfig()
		log.Printf("Área de nome definida: %v", r)
		go func() {
			name, err := GlobalScanner.CaptureItemName()
			if err != nil {
				log.Printf("Captura pós-calibração nome: %v", err)
			} else {
				log.Printf("Nome detectado: '%s'", name)
			}
		}()
	})
}

func calibratorLoop() {
	io := imgui.CurrentIO()
	mousePos := io.MousePos()
	dispSize := io.DisplaySize()

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{X: 0, Y: 0})
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.Vec4{X: 0, Y: 0, Z: 0, W: 1})

	g.SingleWindow().Flags(
		g.WindowFlags(imgui.WindowFlagsNoTitleBar) |
			g.WindowFlags(imgui.WindowFlagsNoDecoration) |
			g.WindowFlags(imgui.WindowFlagsNoMove) |
			g.WindowFlags(imgui.WindowFlagsNoScrollbar) |
			g.WindowFlags(imgui.WindowFlagsNoScrollWithMouse),
	).Layout(
		g.Custom(func() {
			avail := imgui.ContentRegionAvail()

			if calibPhysW > 0 && calibPhysH > 0 {
				sx := avail.X / float32(calibPhysW)
				sy := avail.Y / float32(calibPhysH)
				calibScreenTex.ToImageWidget().Scale(sx, sy).Build()
				calibImgMin = imgui.ItemRectMin()
				calibImgMax = imgui.ItemRectMax()
			}

			dl := imgui.WindowDrawList()

			// Barra de instrução no topo
			dl.AddRectFilledV(
				imgui.Vec2{X: 0, Y: 0},
				imgui.Vec2{X: dispSize.X, Y: 52},
				col32(0, 0, 0, 220), 0, imgui.DrawFlagsNone,
			)

			stepLabel := ""
			if calibTotalSteps > 0 {
				stepLabel = fmt.Sprintf("[%d/%d]  ", calibStepNum, calibTotalSteps)
			}
			action := "Clique e arraste"
			if calibIsPoint {
				action = "Clique"
			}
			inst := fmt.Sprintf("%s%s  |  %s  |  ESC cancela", stepLabel, calibTitle, action)
			dl.AddTextVec2V(imgui.Vec2{X: 10, Y: 18}, col32(255, 220, 0, 255), inst)

			// Crosshair
			dl.AddLineV(
				imgui.Vec2{X: mousePos.X, Y: 52},
				imgui.Vec2{X: mousePos.X, Y: dispSize.Y},
				col32(255, 220, 0, 90), 1,
			)
			dl.AddLineV(
				imgui.Vec2{X: 0, Y: mousePos.Y},
				imgui.Vec2{X: dispSize.X, Y: mousePos.Y},
				col32(255, 220, 0, 90), 1,
			)

			// Labels de status flutuando junto ao crosshair (abaixo+direita do mouse)
			type calibStep struct {
				name string
				done bool
			}
			steps := []calibStep{
				{"Busca",       GlobalScanner.HasSearchBar},
				{"1º Result",   GlobalScanner.FirstResult != image.Point{}},
				{"2º Result",   GlobalScanner.HasSecondResult},
				{"3º Result",   GlobalScanner.HasThirdResult},
				{"Fechar Item", GlobalScanner.HasCloseItem},
				{"Quantidade",  GlobalScanner.HasSplitCalib},
				{"Preço",       GlobalScanner.HasSplitCalib},
				{"Nome",        GlobalScanner.HasNameCalib},
			}
			ox := mousePos.X + 14
			oy := mousePos.Y + 14
			// Se está muito perto da borda direita/inferior, espelha para o outro lado
			if ox > dispSize.X-120 {
				ox = mousePos.X - 120
			}
			if oy > dispSize.Y-float32(len(steps))*18-8 {
				oy = mousePos.Y - float32(len(steps))*18 - 8
			}
			boxH := float32(len(steps))*18 + 8
			dl.AddRectFilledV(
				imgui.Vec2{X: ox - 4, Y: oy - 4},
				imgui.Vec2{X: ox + 112, Y: oy + boxH},
				col32(0, 0, 0, 190), 4, imgui.DrawFlagsNone,
			)
			for i, s := range steps {
				var c uint32
				label := "✗ " + s.name
				if s.done {
					c = col32(80, 230, 80, 255)
					label = "✓ " + s.name
				} else {
					c = col32(230, 80, 80, 255)
				}
				// Destaca o passo atual
				if calibTotalSteps > 0 && i == calibStepNum-1 {
					dl.AddRectFilledV(
						imgui.Vec2{X: ox - 4, Y: oy + float32(i)*18 - 2},
						imgui.Vec2{X: ox + 112, Y: oy + float32(i)*18 + 16},
						col32(255, 220, 0, 40), 0, imgui.DrawFlagsNone,
					)
					c = col32(255, 220, 0, 255)
					label = "→ " + s.name
				}
				dl.AddTextVec2V(imgui.Vec2{X: ox, Y: oy + float32(i)*18}, c, label)
			}

			mouseDown := g.IsMouseDown(g.MouseButtonLeft)

			if calibIsPoint {
				// Modo ponto: captura ao soltar o clique
				if mouseDown && mousePos.Y > 52 {
					calibMouseWasDown = true
				}
				if calibMouseWasDown && !mouseDown {
					calibMouseWasDown = false
					calibDragStart = mousePos
					calibDragEnd = mousePos
					finishCalibration()
				}
			} else {
				// Modo área: arrastar para selecionar
				if !calibIsDragging {
					if mouseDown && mousePos.Y > 52 {
						calibDragStart = mousePos
						calibIsDragging = true
					}
				} else {
					calibDragEnd = mousePos

					dl.AddRectFilledV(calibDragStart, calibDragEnd, col32(255, 220, 0, 40), 0, imgui.DrawFlagsNone)
					dl.AddRectV(calibDragStart, calibDragEnd, col32(255, 220, 0, 230), 0, imgui.DrawFlagsNone, 2)

					cornerLen := float32(8)
					drawCorner := func(x1, y1, x2, y2 float32) {
						dl.AddLineV(imgui.Vec2{X: x1, Y: y1}, imgui.Vec2{X: x2, Y: y1}, col32(255, 220, 0, 255), 2)
						dl.AddLineV(imgui.Vec2{X: x1, Y: y1}, imgui.Vec2{X: x1, Y: y2}, col32(255, 220, 0, 255), 2)
					}
					x1, y1 := calibDragStart.X, calibDragStart.Y
					x2, y2 := calibDragEnd.X, calibDragEnd.Y
					if x1 > x2 {
						x1, x2 = x2, x1
					}
					if y1 > y2 {
						y1, y2 = y2, y1
					}
					drawCorner(x1, y1, x1+cornerLen, y1+cornerLen)
					drawCorner(x2, y1, x2-cornerLen, y1+cornerLen)
					drawCorner(x1, y2, x1+cornerLen, y2-cornerLen)
					drawCorner(x2, y2, x2-cornerLen, y2-cornerLen)

					imgW := calibImgMax.X - calibImgMin.X
					imgH := calibImgMax.Y - calibImgMin.Y
					if imgW > 0 && imgH > 0 {
						pw := int(abs32(calibDragEnd.X-calibDragStart.X) * float32(calibPhysW) / imgW)
						ph := int(abs32(calibDragEnd.Y-calibDragStart.Y) * float32(calibPhysH) / imgH)
						label := fmt.Sprintf(" %dx%d px ", pw, ph)
						lx := (calibDragStart.X + calibDragEnd.X) / 2
						ly := calibDragEnd.Y + 6
						if ly > dispSize.Y-20 {
							ly = calibDragEnd.Y - 20
						}
						dl.AddRectFilledV(
							imgui.Vec2{X: lx - 30, Y: ly - 2},
							imgui.Vec2{X: lx + 50, Y: ly + 16},
							col32(0, 0, 0, 180), 0, imgui.DrawFlagsNone,
						)
						dl.AddTextVec2V(imgui.Vec2{X: lx - 24, Y: ly}, col32(255, 220, 0, 255), label)
					}

					if !mouseDown {
						finishCalibration()
					}
				}
			}

			if g.IsKeyPressed(g.KeyEscape) {
				cancelCalibration()
			}
		}),
	)

	imgui.PopStyleColor()
	imgui.PopStyleVar()
}

func finishCalibration() {
	calibIsDragging = false
	calibActive = false

	imgW := calibImgMax.X - calibImgMin.X
	imgH := calibImgMax.Y - calibImgMin.Y
	if imgW == 0 || imgH == 0 || calibPhysW == 0 || calibPhysH == 0 {
		log.Println("Calibração inválida, tente novamente")
		restoreWindow()
		return
	}

	scaleX := float32(calibPhysW) / imgW
	scaleY := float32(calibPhysH) / imgH

	x1 := int((clamp32(calibDragStart.X, calibImgMin.X, calibImgMax.X) - calibImgMin.X) * scaleX)
	y1 := int((clamp32(calibDragStart.Y, calibImgMin.Y, calibImgMax.Y) - calibImgMin.Y) * scaleY)
	x2 := int((clamp32(calibDragEnd.X, calibImgMin.X, calibImgMax.X) - calibImgMin.X) * scaleX)
	y2 := int((clamp32(calibDragEnd.Y, calibImgMin.Y, calibImgMax.Y) - calibImgMin.Y) * scaleY)

	// Corrige escala DPI: no Windows com HiDPI, GetDisplayBounds retorna pixels físicos
	// enquanto robotgo.Move() espera pixels lógicos (SM_CXSCREEN). No macOS ambos usam
	// CGDisplayBounds (lógico), então logW == calibPhysW e a correção é no-op.
	logW, logH := robotgo.GetScreenSize()
	if calibPhysW > 0 && logW > 0 && calibPhysW != logW {
		x1 = x1 * logW / calibPhysW
		y1 = y1 * logH / calibPhysH
		x2 = x2 * logW / calibPhysW
		y2 = y2 * logH / calibPhysH
		log.Printf("Correção DPI aplicada (fator %.2fx): coords físicas→lógicas", float64(calibPhysW)/float64(logW))
	}

	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	// Para modo área, rejeita seleções muito pequenas
	if !calibIsPoint && (x2-x1 < 5 || y2-y1 < 5) {
		log.Println("Área muito pequena, tente novamente")
		calibActive = false
		restoreWindow()
		return
	}

	// Para modo ponto, garante pelo menos 1px de tamanho para o centro funcionar
	if calibIsPoint {
		x2 = x1 + 1
		y2 = y1 + 1
	}

	rect := image.Rect(x1, y1, x2, y2)
	log.Printf("Área selecionada: (%d,%d)-(%d,%d)", x1, y1, x2, y2)

	restoreWindow()
	g.Update()

	if calibOnDone != nil {
		calibOnDone(rect)
	}
}

func cancelCalibration() {
	calibIsDragging = false
	calibActive = false
	calibStepNum = 0
	calibTotalSteps = 0
	restoreWindow()
	g.Update()
	log.Println("Calibração cancelada")
}

func restoreWindow() {
	if savedWndW > 0 && savedWndH > 0 {
		wnd.SetSize(savedWndW, savedWndH)
		wnd.SetPos(savedWndX, savedWndY)
	} else {
		wnd.SetSize(380, 350)
		wnd.SetPos(300, 300)
	}
}
