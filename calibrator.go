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
	calibActive     bool
	calibScreenTex  = &g.ReflectiveBoundTexture{}
	calibPhysW      int
	calibPhysH      int
	calibIsDragging bool
	calibDragStart  imgui.Vec2
	calibDragEnd    imgui.Vec2
	calibImgMin     imgui.Vec2
	calibImgMax     imgui.Vec2
	calibTitle      string
	calibOnDone     func(image.Rectangle)
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

func StartPriceCalibration() {
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

func StartCalibration(title string, onDone func(image.Rectangle)) {
	go func() {
		// Captura a tela inteira
		bounds := screenshot.GetDisplayBounds(0)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			log.Printf("Falha ao capturar tela: %v", err)
			return
		}

		// Tamanho LÓGICO da tela (em pontos, não pixels físicos)
		screenW, screenH := robotgo.GetScreenSize()
		log.Printf("Tela lógica: %dx%d | Screenshot físico: %dx%d", screenW, screenH, bounds.Dx(), bounds.Dy())

		mainthread.Call(func() {
			calibPhysW = bounds.Dx()
			calibPhysH = bounds.Dy()
			calibScreenTex.SetSurfaceFromRGBA(img, false)
			calibIsDragging = false
			calibTitle = title
			calibOnDone = onDone
			calibActive = true

			wnd.SetPos(0, 0)
			wnd.SetSize(screenW, screenH)
		})
		g.Update()
	}()
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

			// Renderiza screenshot como fundo
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
				imgui.Vec2{X: dispSize.X, Y: 44},
				col32(0, 0, 0, 210), 0, imgui.DrawFlagsNone,
			)
			inst := fmt.Sprintf("Selecione a [%s]  |  Clique e arraste  |  ESC para cancelar", calibTitle)
			dl.AddTextVec2V(
				imgui.Vec2{X: 10, Y: 14},
				col32(255, 220, 0, 255),
				inst,
			)

			// Crosshair
			dl.AddLineV(
				imgui.Vec2{X: mousePos.X, Y: 44},
				imgui.Vec2{X: mousePos.X, Y: dispSize.Y},
				col32(255, 220, 0, 90), 1,
			)
			dl.AddLineV(
				imgui.Vec2{X: 0, Y: mousePos.Y},
				imgui.Vec2{X: dispSize.X, Y: mousePos.Y},
				col32(255, 220, 0, 90), 1,
			)

			// Sombra escura fora da area do screenshot
			dl.AddRectFilledV(calibImgMin, calibImgMax, col32(0, 0, 0, 0), 0, imgui.DrawFlagsNone)

			// Tracking do drag direto via g.IsMouseDown
			mouseDown := g.IsMouseDown(g.MouseButtonLeft)

			if !calibIsDragging {
				if mouseDown && mousePos.Y > 44 {
					calibDragStart = mousePos
					calibIsDragging = true
				}
			} else {
				calibDragEnd = mousePos

				// Retângulo de seleção
				dl.AddRectFilledV(calibDragStart, calibDragEnd, col32(255, 220, 0, 40), 0, imgui.DrawFlagsNone)
				dl.AddRectV(calibDragStart, calibDragEnd, col32(255, 220, 0, 230), 0, imgui.DrawFlagsNone, 2)

				// Cantos de 8px para facilitar visualização
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

				// Label de tamanho
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

				// Ao soltar o mouse, finaliza
				if !mouseDown {
					finishCalibration()
				}
			}

			// ESC cancela
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

	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	if x2-x1 < 5 || y2-y1 < 5 {
		log.Println("Área muito pequena, tente novamente")
		calibActive = false
		restoreWindow()
		return
	}

	rect := image.Rect(x1, y1, x2, y2)
	log.Printf("Área selecionada: (%d,%d)-(%d,%d) [%dx%d px]", x1, y1, x2, y2, x2-x1, y2-y1)

	if calibOnDone != nil {
		calibOnDone(rect)
	}

	restoreWindow()
	g.Update()
}

func cancelCalibration() {
	calibIsDragging = false
	calibActive = false
	restoreWindow()
	g.Update()
	log.Println("Calibração cancelada")
}

func restoreWindow() {
	wnd.SetSize(380, 350)
	wnd.SetPos(300, 300)
}
