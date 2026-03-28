package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/AllenDang/giu"
	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"
	"github.com/otiai10/gosseract/v2"
	"golang.design/x/hotkey/mainthread"
	"golang.org/x/image/draw"
)

type MarketScanner struct {
	SearchBarRect   image.Rectangle // área do campo de busca (para OCR + clique)
	FirstResult     image.Point
	SecondResult    image.Point
	ThirdResult     image.Point
	CloseItem       image.Point
	PriceAreaRect   image.Rectangle // fallback (sem split)
	QtyColRect      image.Rectangle // coluna de quantidade (split)
	PriceColRect    image.Rectangle // coluna de preço (split)
	ItemNameRect    image.Rectangle
	IsCalibrated    bool
	HasNameCalib    bool
	HasSecondResult bool
	HasThirdResult  bool
	HasSplitCalib   bool
	HasCloseItem    bool
	HasSearchBar    bool
}

type PriceTier struct {
	Qty   int64
	Price int64
}

var GlobalScanner = &MarketScanner{}

// focusDofus traz a janela do Dofus para frente antes de interagir.
func focusDofus() {
	fpid, err := robotgo.FindIds("Dofus")
	if err != nil || len(fpid) == 0 {
		log.Printf("focusDofus: Dofus não encontrado: %v", err)
		return
	}
	robotgo.ActivePid(fpid[0])
	time.Sleep(300 * time.Millisecond)
}

// searchBarHasText captura a área do campo de busca e verifica via OCR se há texto digitado.
func (s *MarketScanner) searchBarHasText() bool {
	img, err := screenshot.CaptureRect(s.SearchBarRect)
	if err != nil || img == nil {
		return false
	}
	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, gv, b, a := img.At(x, y).RGBA()
			inverted.SetRGBA(x, y, struct{ R, G, B, A uint8 }{
				R: uint8(255 - r>>8),
				G: uint8(255 - gv>>8),
				B: uint8(255 - b>>8),
				A: uint8(a >> 8),
			})
		}
	}
	scaled := scaleImage(inverted, 3)
	tmpPath := "/Volumes/ssd/www/Pessoal/dofus/dofushunt/debug_search.png"
	f, err := os.Create(tmpPath)
	if err != nil {
		return false
	}
	png.Encode(f, scaled)
	f.Close()

	client := gosseract.NewClient()
	defer client.Close()
	client.SetImage(tmpPath)
	client.SetPageSegMode(gosseract.PSM_SINGLE_LINE)
	text, err := client.Text()
	if err != nil {
		return false
	}
	result := strings.TrimSpace(text) != ""
	log.Printf("searchBarHasText: '%s' -> %v", strings.TrimSpace(text), result)
	return result
}

func (s *MarketScanner) SearchItem(name string) {
	log.Printf("Searching for item: %s", name)

	// Garante foco no Dofus antes de qualquer interação
	mainthread.Call(focusDofus)

	cx := (s.SearchBarRect.Min.X + s.SearchBarRect.Max.X) / 2
	cy := (s.SearchBarRect.Min.Y + s.SearchBarRect.Max.Y) / 2

	if s.searchBarHasText() {
		// Tem texto: mover para a direita do input (onde fica o X), pausar e clicar
		rightX := s.SearchBarRect.Max.X - 12
		MoveHumanLike(rightX, cy)
		time.Sleep(time.Duration(randRange(200, 450)) * time.Millisecond)
		ClickHumanLike()
		time.Sleep(time.Duration(randRange(250, 450)) * time.Millisecond)
	} else {
		// Campo vazio: um clique simples para focar
		MoveHumanLike(cx, cy)
		ClickHumanLike()
		time.Sleep(time.Duration(randRange(200, 350)) * time.Millisecond)
	}

	// 15% de chance de um clique extra (simulando erro humano)
	if rand.Float64() < 0.15 {
		time.Sleep(time.Duration(randRange(100, 300)) * time.Millisecond)
		ClickHumanLike()
		time.Sleep(200 * time.Millisecond)
	}

	TypeHumanLike(name)
	mainthread.Call(func() {
		robotgo.KeyTap("enter")
	})
	// Aguarda os resultados carregarem antes de clicar (tempo variável)
	time.Sleep(time.Duration(randRange(1200, 2200)) * time.Millisecond)
}

func (s *MarketScanner) ClickFirstResult() {
	log.Println("Clicking first result")
	MoveHumanLike(s.FirstResult.X, s.FirstResult.Y)
	ClickHumanLike()
	time.Sleep(500 * time.Millisecond)
}

func (s *MarketScanner) ClickSecondResult() {
	log.Println("Clicking second result")
	MoveHumanLike(s.SecondResult.X, s.SecondResult.Y)
	ClickHumanLike()
	time.Sleep(500 * time.Millisecond)
}

func (s *MarketScanner) ClickThirdResult() {
	log.Println("Clicking third result")
	MoveHumanLike(s.ThirdResult.X, s.ThirdResult.Y)
	ClickHumanLike()
	time.Sleep(500 * time.Millisecond)
}

func (s *MarketScanner) ClickCloseItem() {
	log.Println("Closing item")
	MoveHumanLike(s.CloseItem.X, s.CloseItem.Y)
	ClickHumanLike()
	time.Sleep(time.Duration(randRange(300, 600)) * time.Millisecond)
}

func (s *MarketScanner) CapturePrice() (int64, error) {
	if s.HasSplitCalib {
		return ocrNumber(s.PriceColRect, true)
	}
	return capturePriceRect(s.PriceAreaRect, true)
}

func shiftRectY(r image.Rectangle, dy int) image.Rectangle {
	return image.Rect(r.Min.X, r.Min.Y+dy, r.Max.X, r.Max.Y+dy)
}

// dofusQtyByRow são as quantidades fixas como fallback quando não há split calibration.
var dofusQtyByRow = [4]int64{1, 10, 100, 1000}

// CapturePrices captura até 4 linhas de preço.
// Com split calibration: lê qty e preço de regiões separadas (mais preciso).
// Sem split calibration: usa PriceAreaRect com qty hardcoded.
func (s *MarketScanner) CapturePrices() ([]PriceTier, error) {
	if s.HasSplitCalib {
		return s.capturePricesSplit()
	}
	return s.capturePricesFallback()
}

func (s *MarketScanner) capturePricesSplit() ([]PriceTier, error) {
	step := s.QtyColRect.Dy()
	var tiers []PriceTier
	for i := 0; i < 4; i++ {
		qtyRect := shiftRectY(s.QtyColRect, i*step)
		priceRect := shiftRectY(s.PriceColRect, i*step)

		qty, err := ocrNumber(qtyRect, false)
		if err != nil || qty == 0 {
			break
		}
		price, err := ocrNumber(priceRect, i == 0)
		if err != nil || price == 0 {
			break
		}
		tiers = append(tiers, PriceTier{Qty: qty, Price: price})
	}
	return tiers, nil
}

func (s *MarketScanner) capturePricesFallback() ([]PriceTier, error) {
	step := s.PriceAreaRect.Dy()
	var tiers []PriceTier
	for i := 0; i < 4; i++ {
		rect := shiftRectY(s.PriceAreaRect, i*step)
		price, err := capturePriceRect(rect, i == 0)
		if err != nil {
			return tiers, err
		}
		if price == 0 {
			break
		}
		tiers = append(tiers, PriceTier{Qty: dofusQtyByRow[i], Price: price})
	}
	return tiers, nil
}

// ocrNumber captura uma região e retorna o número lido pelo OCR.
// Usa a mesma pipeline de inversão + escala 3x.
func ocrNumber(rect image.Rectangle, updatePreview bool) (int64, error) {
	img, err := screenshot.CaptureRect(rect)
	if err != nil {
		return 0, fmt.Errorf("failed to capture rect: %w", err)
	}
	if img == nil {
		return 0, fmt.Errorf("captura retornou nil")
	}

	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, gv, b, a := img.At(x, y).RGBA()
			inverted.SetRGBA(x, y, struct{ R, G, B, A uint8 }{
				R: uint8(255 - r>>8),
				G: uint8(255 - gv>>8),
				B: uint8(255 - b>>8),
				A: uint8(a >> 8),
			})
		}
	}

	if updatePreview {
		mainthread.Call(func() {
			pricePreviewTexture.SetSurfaceFromRGBA(img, false)
		})
		g.Update()
	}

	scaled := scaleImage(inverted, 3)

	tmpPath := "/Volumes/ssd/www/Pessoal/dofus/dofushunt/debug_price.png"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create debug file: %w", err)
	}
	err = png.Encode(f, scaled)
	f.Close()
	if err != nil {
		return 0, fmt.Errorf("failed to encode image: %w", err)
	}

	client := gosseract.NewClient()
	defer client.Close()
	client.SetImage(tmpPath)
	client.SetWhitelist("0123456789. ,kK")
	client.SetPageSegMode(gosseract.PSM_SINGLE_LINE)

	text, err := client.Text()
	if err != nil {
		return 0, fmt.Errorf("OCR failed: %w", err)
	}

	n := parsePriceOnly(text)
	log.Printf("ocrNumber rect=%v text='%s' -> %d", rect, strings.TrimSpace(text), n)
	return n, nil
}

func capturePriceRect(rect image.Rectangle, updatePreview bool) (int64, error) {
	log.Printf("Capturing price area: %v (%dx%d)", rect, rect.Dx(), rect.Dy())

	img, err := screenshot.CaptureRect(rect)
	if err != nil {
		return 0, fmt.Errorf("failed to capture screen: %w", err)
	}
	if img == nil {
		return 0, fmt.Errorf("imagem capturada é nula")
	}

	// Inverte as cores: texto amarelo/branco em fundo escuro → escuro em fundo claro
	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, gv, b, a := img.At(x, y).RGBA()
			inverted.SetRGBA(x, y, struct{ R, G, B, A uint8 }{
				R: uint8(255 - r>>8),
				G: uint8(255 - gv>>8),
				B: uint8(255 - b>>8),
				A: uint8(a >> 8),
			})
		}
	}

	if updatePreview {
		mainthread.Call(func() {
			pricePreviewTexture.SetSurfaceFromRGBA(img, false)
		})
		g.Update()
	}

	scaled := scaleImage(inverted, 3)

	tmpPath := "/Volumes/ssd/www/Pessoal/dofus/dofushunt/debug_price.png"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create debug file: %w", err)
	}
	err = png.Encode(f, scaled)
	f.Close()
	if err != nil {
		return 0, fmt.Errorf("failed to encode image: %w", err)
	}

	client := gosseract.NewClient()
	defer client.Close()
	client.SetImage(tmpPath)
	client.SetWhitelist("0123456789. ,kK")
	client.SetPageSegMode(gosseract.PSM_SINGLE_LINE)

	text, err := client.Text()
	if err != nil {
		return 0, fmt.Errorf("OCR failed: %w", err)
	}

	price := parsePriceOnly(text)
	log.Printf("Captured price text: '%s' -> price=%d", strings.TrimSpace(text), price)

	return price, nil
}


func (s *MarketScanner) CaptureItemName() (string, error) {
	if s.ItemNameRect.Dx() < 5 || s.ItemNameRect.Dy() < 5 {
		return "", fmt.Errorf("área de nome não calibrada")
	}

	img, err := screenshot.CaptureRect(s.ItemNameRect)
	if err != nil {
		return "", fmt.Errorf("falha ao capturar área do nome: %w", err)
	}
	if img == nil {
		return "", fmt.Errorf("captura do nome retornou nil")
	}

	// Inverte as cores: Dofus usa texto branco/amarelo em fundo escuro.
	// Tesseract funciona melhor com texto escuro em fundo claro.
	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			inverted.SetRGBA(x, y, struct{ R, G, B, A uint8 }{
				R: uint8(255 - r>>8),
				G: uint8(255 - g>>8),
				B: uint8(255 - b>>8),
				A: uint8(a >> 8),
			})
		}
	}

	// Atualiza preview na UI com imagem original
	mainthread.Call(func() {
		pricePreviewTexture.SetSurfaceFromRGBA(img, false)
	})
	g.Update()

	// Salva versão invertida para OCR e debug
	debugPath := "/Volumes/ssd/www/Pessoal/dofus/dofushunt/debug_name.png"
	f, _ := os.Create(debugPath)
	if f != nil {
		png.Encode(f, inverted)
		f.Close()
	}

	// OCR configurado para linha única e texto livre
	client := gosseract.NewClient()
	defer client.Close()
	client.SetImage(debugPath)
	// PSM 7 = linha única de texto; OEM 1 = LSTM neural net
	client.SetPageSegMode(gosseract.PSM_SINGLE_LINE)

	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("OCR nome falhou: %w", err)
	}

	name := cleanItemName(strings.TrimSpace(text))
	log.Printf("Captured item name: '%s'", name)
	return name, nil
}

// cleanItemName remove ruído comum do OCR em nomes de itens do Dofus
func cleanItemName(s string) string {
	// Remove linhas vazias e pega primeira linha não-vazia
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= 2 {
			return line
		}
	}
	return s
}


// parsePriceData extrai quantidade e preço do texto OCR do Dofus.
//
// Estratégia:
//  1. Divide por espaços → tokens individuais (ex: ["1", "8.976", "4"])
//  2. Remove pontos/vírgulas DENTRO de cada token (separadores de milhar do Dofus)
//     → ["1", "8976", "4"]
//  3. Interpreta: primeiro=qty, meio=preço, último=ruído do ícone de kamas
//
// Exemplos:
//
//	"8.976"       → qty=1  price=8976
//	"1 1790 4"    → qty=1  price=1790
//	"6 594.897 4" → qty=6  price=594897
func parsePriceData(text string) (qty int64, price int64) {
	text = strings.ToLower(strings.TrimSpace(text))

	isKilo := strings.Contains(text, "k")

	// Divide por whitespace
	rawTokens := strings.Fields(text)

	reDigits := regexp.MustCompile(`[^0-9]`)
	var numbers []int64
	for _, tok := range rawTokens {
		// Remove separadores de milhar (. e ,)
		tok = strings.ReplaceAll(tok, ".", "")
		tok = strings.ReplaceAll(tok, ",", "")
		tok = reDigits.ReplaceAllString(tok, "")
		if tok == "" {
			continue
		}
		val, err := strconv.ParseInt(tok, 10, 64)
		if err != nil || val == 0 {
			continue
		}
		numbers = append(numbers, val)
	}

	applyKilo := func(v int64) int64 {
		if isKilo {
			return v * 1000
		}
		return v
	}

	// Remove trailing noise: único dígito ≤ 9 é sempre ruído do ícone de kamas
	for len(numbers) >= 2 && numbers[len(numbers)-1] <= 9 {
		numbers = numbers[:len(numbers)-1]
	}

	switch len(numbers) {
	case 0:
		return 1, 0
	case 1:
		// Só preço (ex: "8976" vindo de "8.976", ou "974" vindo de "974 4")
		return 1, applyKilo(numbers[0])
	case 2:
		// qty + preço
		return numbers[0], applyKilo(numbers[1])
	default:
		// qty + múltiplos tokens de preço (raro)
		q := numbers[0]
		midStr := ""
		for _, n := range numbers[1:] {
			midStr += strconv.FormatInt(n, 10)
		}
		v, _ := strconv.ParseInt(midStr, 10, 64)
		return q, applyKilo(v)
	}
}

// parsePriceOnly extrai apenas o preço do texto OCR, ignorando a quantidade.
// Pega o último número válido da linha (que é sempre o preço no layout do Dofus).
// Exemplos:
//
//	"1 99 K"      → 99000
//	"10\n244"     → 244
//	"3 57"        → 57   (qty=3 é ruído de OCR do "1")
//	"0\n289"      → 289  (0 descartado por ser zero)
//	"1.000 27.496"→ 27496
func parsePriceOnly(text string) int64 {
	text = strings.ToLower(strings.TrimSpace(text))
	isKilo := strings.Contains(text, "k")

	reDigits := regexp.MustCompile(`[^0-9]`)
	var numbers []int64
	for _, tok := range strings.Fields(text) {
		tok = strings.ReplaceAll(tok, ".", "")
		tok = strings.ReplaceAll(tok, ",", "")
		tok = reDigits.ReplaceAllString(tok, "")
		if tok == "" {
			continue
		}
		val, err := strconv.ParseInt(tok, 10, 64)
		if err != nil || val == 0 {
			continue
		}
		numbers = append(numbers, val)
	}

	// Remove trailing noise: único dígito ≤ 9 é ruído do ícone de kamas
	for len(numbers) >= 2 && numbers[len(numbers)-1] <= 9 {
		numbers = numbers[:len(numbers)-1]
	}

	if len(numbers) == 0 {
		return 0
	}
	// O preço é sempre o último número (o primeiro pode ser a qty impressa na célula)
	price := numbers[len(numbers)-1]
	if isKilo {
		price *= 1000
	}
	return price
}


func (s *MarketScanner) CalibrateFirstResult() {
	log.Println(">>> CALIBRANDO PRIMEIRO RESULTADO EM 3 SEGUNDOS... Posicione o mouse!")
	time.Sleep(3 * time.Second)
	x, y := robotgo.GetMousePos()
	fmt.Print("\a") // Bell sound
	s.FirstResult = image.Point{x, y}
	log.Printf("!!! PRIMEIRO RESULTADO CALIBRADO: %v", s.FirstResult)
	SaveConfig()
}

func (s *MarketScanner) CalibrateSecondResult() {
	log.Println(">>> CALIBRANDO SEGUNDO RESULTADO EM 3 SEGUNDOS... Posicione o mouse!")
	time.Sleep(3 * time.Second)
	x, y := robotgo.GetMousePos()
	fmt.Print("\a") // Bell sound
	s.SecondResult = image.Point{x, y}
	s.HasSecondResult = true
	log.Printf("!!! SEGUNDO RESULTADO CALIBRADO: %v", s.SecondResult)
	SaveConfig()
}

// scaleImage aumenta a imagem pelo fator dado usando interpolação bilinear.
// Tesseract reconhece muito melhor fontes pequenas quando a imagem é ampliada.
func scaleImage(src image.Image, factor int) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*factor, b.Dy()*factor))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// PricePadX e PricePadY definem o tamanho do retângulo capturado ao redor do ponto central
const PricePadX = 120
const PricePadY = 30

func (s *MarketScanner) CalibratePriceArea() {
	log.Println("--- Calibrando Área de Preço ---")
	log.Printf("Posicione o mouse NO CENTRO do preço. Aguarde 3 seg... (será capturado um retângulo de %dx%d ao redor)", PricePadX*2, PricePadY*2)
	time.Sleep(3 * time.Second)
	cx, cy := robotgo.GetMousePos()
	fmt.Print("\a")
	log.Printf("Ponto central definido: (%d, %d)", cx, cy)

	s.PriceAreaRect = image.Rect(cx-PricePadX, cy-PricePadY, cx+PricePadX, cy+PricePadY)
	s.IsCalibrated = true
	log.Printf("CALIBRAÇÃO COMPLETA: Área de Preço: %v (%dx%d pixels)", s.PriceAreaRect, s.PriceAreaRect.Dx(), s.PriceAreaRect.Dy())

	// Tenta capturar e exibir imediatamente para debug
	price, err := s.CapturePrice()
	if err != nil {
		log.Printf("AVISO: Captura retornou erro: %v", err)
	} else if price <= 0 {
		log.Printf("AVISO: Nenhum preço detectado ainda (OCR retornou 0). Verifique a imagem de debug.")
	} else {
		log.Printf("Preço detectado na calibração: %d kamas", price)
	}
}
