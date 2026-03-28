package main

import (
	"encoding/json"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

type ScannerConfig struct {
	SearchBarRect   *RectJSON  `json:"search_bar_rect,omitempty"`
	FirstResult     *PointJSON `json:"first_result,omitempty"`
	SecondResult    *PointJSON `json:"second_result,omitempty"`
	CloseItem       *PointJSON `json:"close_item,omitempty"`
	PriceArea       *RectJSON  `json:"price_area,omitempty"`
	QtyColRect      *RectJSON  `json:"qty_col_rect,omitempty"`
	PriceColRect    *RectJSON  `json:"price_col_rect,omitempty"`
	ItemNameRect    *RectJSON  `json:"item_name_rect,omitempty"`
	IsCalibrated    bool       `json:"is_calibrated"`
	HasNameCalib    bool       `json:"has_name_calib"`
	HasSecondResult bool       `json:"has_second_result"`
	HasSplitCalib   bool       `json:"has_split_calib"`
	HasCloseItem    bool       `json:"has_close_item"`
	HasSearchBar    bool       `json:"has_search_bar"`
}

type PointJSON struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type RectJSON struct {
	X1 int `json:"x1"`
	Y1 int `json:"y1"`
	X2 int `json:"x2"`
	Y2 int `json:"y2"`
}

func configPath() string {
	// Salva ao lado do executável
	exe, err := os.Executable()
	if err != nil {
		// Fallback: diretório de trabalho
		return "dofhunt_config.json"
	}
	// Se rodando via "go run .", o executável está em /tmp — usa cwd
	if runtime.GOOS != "windows" {
		dir := filepath.Dir(exe)
		if filepath.Base(dir) == "dofhunt" || filepath.Base(dir) == "." {
			return filepath.Join(dir, "dofhunt_config.json")
		}
	}
	// Verifica se estamos num temp dir (go run .)
	dir := filepath.Dir(exe)
	if isTemp(dir) {
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, "dofhunt_config.json")
	}
	return filepath.Join(dir, "dofhunt_config.json")
}

func isTemp(dir string) bool {
	tmp := os.TempDir()
	return len(dir) >= len(tmp) && dir[:len(tmp)] == tmp
}

func SaveConfig() {
	cfg := ScannerConfig{
		IsCalibrated: GlobalScanner.IsCalibrated,
		HasNameCalib: GlobalScanner.HasNameCalib,
	}

	if GlobalScanner.HasSearchBar {
		r := GlobalScanner.SearchBarRect
		cfg.SearchBarRect = &RectJSON{X1: r.Min.X, Y1: r.Min.Y, X2: r.Max.X, Y2: r.Max.Y}
		cfg.HasSearchBar = true
	}
	if GlobalScanner.FirstResult != (image.Point{}) {
		cfg.FirstResult = &PointJSON{X: GlobalScanner.FirstResult.X, Y: GlobalScanner.FirstResult.Y}
	}
	if GlobalScanner.HasSecondResult {
		cfg.SecondResult = &PointJSON{X: GlobalScanner.SecondResult.X, Y: GlobalScanner.SecondResult.Y}
		cfg.HasSecondResult = true
	}
	if GlobalScanner.HasCloseItem {
		cfg.CloseItem = &PointJSON{X: GlobalScanner.CloseItem.X, Y: GlobalScanner.CloseItem.Y}
		cfg.HasCloseItem = true
	}
	if GlobalScanner.IsCalibrated {
		r := GlobalScanner.PriceAreaRect
		cfg.PriceArea = &RectJSON{X1: r.Min.X, Y1: r.Min.Y, X2: r.Max.X, Y2: r.Max.Y}
	}
	if GlobalScanner.HasSplitCalib {
		r := GlobalScanner.QtyColRect
		cfg.QtyColRect = &RectJSON{X1: r.Min.X, Y1: r.Min.Y, X2: r.Max.X, Y2: r.Max.Y}
		r = GlobalScanner.PriceColRect
		cfg.PriceColRect = &RectJSON{X1: r.Min.X, Y1: r.Min.Y, X2: r.Max.X, Y2: r.Max.Y}
		cfg.HasSplitCalib = true
	}
	if GlobalScanner.HasNameCalib {
		r := GlobalScanner.ItemNameRect
		cfg.ItemNameRect = &RectJSON{X1: r.Min.X, Y1: r.Min.Y, X2: r.Max.X, Y2: r.Max.Y}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("Erro ao serializar config: %v", err)
		return
	}

	path := configPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Erro ao salvar config em %s: %v", path, err)
		return
	}
	log.Printf("Config salvo: %s", path)
}

func LoadConfig() {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Erro ao ler config: %v", err)
		}
		return
	}

	var cfg ScannerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Erro ao parsear config: %v", err)
		return
	}

	if cfg.SearchBarRect != nil {
		GlobalScanner.SearchBarRect = image.Rect(cfg.SearchBarRect.X1, cfg.SearchBarRect.Y1, cfg.SearchBarRect.X2, cfg.SearchBarRect.Y2)
	}
	if cfg.FirstResult != nil {
		GlobalScanner.FirstResult = image.Point{X: cfg.FirstResult.X, Y: cfg.FirstResult.Y}
	}
	if cfg.SecondResult != nil {
		GlobalScanner.SecondResult = image.Point{X: cfg.SecondResult.X, Y: cfg.SecondResult.Y}
	}
	if cfg.CloseItem != nil {
		GlobalScanner.CloseItem = image.Point{X: cfg.CloseItem.X, Y: cfg.CloseItem.Y}
	}
	if cfg.PriceArea != nil {
		GlobalScanner.PriceAreaRect = image.Rect(cfg.PriceArea.X1, cfg.PriceArea.Y1, cfg.PriceArea.X2, cfg.PriceArea.Y2)
	}
	if cfg.ItemNameRect != nil {
		GlobalScanner.ItemNameRect = image.Rect(cfg.ItemNameRect.X1, cfg.ItemNameRect.Y1, cfg.ItemNameRect.X2, cfg.ItemNameRect.Y2)
	}
	if cfg.QtyColRect != nil {
		GlobalScanner.QtyColRect = image.Rect(cfg.QtyColRect.X1, cfg.QtyColRect.Y1, cfg.QtyColRect.X2, cfg.QtyColRect.Y2)
	}
	if cfg.PriceColRect != nil {
		GlobalScanner.PriceColRect = image.Rect(cfg.PriceColRect.X1, cfg.PriceColRect.Y1, cfg.PriceColRect.X2, cfg.PriceColRect.Y2)
	}
	GlobalScanner.IsCalibrated = cfg.IsCalibrated
	GlobalScanner.HasNameCalib = cfg.HasNameCalib
	GlobalScanner.HasSecondResult = cfg.HasSecondResult
	GlobalScanner.HasSplitCalib = cfg.HasSplitCalib
	GlobalScanner.HasCloseItem = cfg.HasCloseItem
	GlobalScanner.HasSearchBar = cfg.HasSearchBar

	log.Printf("Config carregado: %s (price=%v name=%v)", path, cfg.IsCalibrated, cfg.HasNameCalib)
}
