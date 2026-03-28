package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	_ "github.com/lib/pq"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var globalDB *sql.DB

type dbItem struct {
	ID         int
	Name       string
	Normalized string
}

// Servidores disponíveis agrupados por tipo.
var ServerGroups = []struct {
	Label   string
	Servers []string
}{
	{"Pioneiro", []string{"Salar", "Brial", "Rafal"}},
	{"Pioneiro Monoconta", []string{"Kourial", "Dakal", "Mikhal"}},
	{"Clássico", []string{"Tal Kasha", "Hell Mina", "Imagiro", "Orukam", "Tylezia"}},
	{"Clássico Monoconta", []string{"Draconiros"}},
	{"Épico", []string{"Sombra"}},
}

// ServerList é a lista plana para o Combo, no formato "Grupo: Servidor".
var ServerList = func() []string {
	var list []string
	for _, g := range ServerGroups {
		for _, s := range g.Servers {
			list = append(list, g.Label+": "+s)
		}
	}
	return list
}()

// ServerName extrai o nome do servidor a partir da entrada do Combo ("Grupo: Servidor").
func ServerName(comboEntry string) string {
	parts := strings.SplitN(comboEntry, ": ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return comboEntry
}

// loadDatabaseURL lê DATABASE_URL do .env ou variável de ambiente.
func loadDatabaseURL() string {
	envPath := envFilePath()
	f, err := os.Open(envPath)
	if err != nil {
		return os.Getenv("DATABASE_URL")
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "DATABASE_URL=") {
			val := strings.TrimPrefix(line, "DATABASE_URL=")
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return os.Getenv("DATABASE_URL")
}

func envFilePath() string {
	// 1. Diretório de trabalho atual (funciona com go run e binário no mesmo dir)
	if cwd, err := os.Getwd(); err == nil {
		if p := filepath.Join(cwd, ".env"); fileExists(p) {
			return p
		}
	}
	// 2. Diretório do executável (binário instalado em outro lugar)
	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), ".env"); fileExists(p) {
			return p
		}
	}
	return ".env"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// stripPrismaParams remove parâmetros de query específicos do Prisma (ex: schema=public)
// que o driver pq não reconhece.
func stripPrismaParams(dsn string) string {
	idx := strings.Index(dsn, "?")
	if idx < 0 {
		return dsn
	}
	base := dsn[:idx]
	query := dsn[idx+1:]

	var kept []string
	for _, param := range strings.Split(query, "&") {
		key := param
		if eq := strings.Index(param, "="); eq >= 0 {
			key = param[:eq]
		}
		switch key {
		case "schema": // Prisma-only
			continue
		default:
			kept = append(kept, param)
		}
	}
	if len(kept) == 0 {
		return base
	}
	return base + "?" + strings.Join(kept, "&")
}

// initDB inicializa a conexão com o banco de dados (lazy, só conecta uma vez).
func initDB() error {
	if globalDB != nil {
		return nil
	}
	url := stripPrismaParams(loadDatabaseURL())
	if url == "" {
		return fmt.Errorf("DATABASE_URL não encontrado no .env")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		return fmt.Errorf("erro ao abrir conexão: %w", err)
	}
	if err := db.Ping(); err != nil {
		return fmt.Errorf("erro ao conectar ao banco: %w", err)
	}
	globalDB = db
	log.Printf("DB: conectado ao PostgreSQL")
	return nil
}

// normalizeForMatch remove acentos e converte para minúsculas.
func normalizeForMatch(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r)
	}), norm.NFC)
	result, _, _ := transform.String(t, strings.ToLower(strings.TrimSpace(s)))
	return result
}

// queryCandidates busca até 50 itens cujo nome contenha o termo mais longo do nome normalizado.
// Fallback: tenta com o termo original completo se nenhuma palavra isolar resultado.
func queryCandidates(normalized string) ([]dbItem, error) {
	// Escolhe a palavra mais longa (≥3 chars) como chave de busca
	searchTerm := normalized
	for _, w := range strings.Fields(normalized) {
		if len(w) > len(searchTerm) {
			searchTerm = w
		}
	}
	if len(searchTerm) < 3 {
		searchTerm = normalized
	}

	rows, err := globalDB.Query(
		`SELECT id, name FROM "Item" WHERE name ILIKE $1 LIMIT 50`,
		"%"+searchTerm+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("erro na query de candidatos: %w", err)
	}
	defer rows.Close()

	var candidates []dbItem
	for rows.Next() {
		var item dbItem
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			continue
		}
		item.Normalized = normalizeForMatch(item.Name)
		candidates = append(candidates, item)
	}

	// Fallback: se não achou nada com o termo longo, tenta com a primeira palavra
	if len(candidates) == 0 {
		words := strings.Fields(normalized)
		if len(words) > 0 && words[0] != searchTerm {
			return queryCandidates(words[0])
		}
	}

	return candidates, nil
}

// findItemID busca o ID do item mais parecido via query no banco + scoring local.
// Retorna (0, false) se nenhum candidato atingir similaridade ≥ 0.6.
func findItemID(name string) (int, bool) {
	normalized := normalizeForMatch(name)

	candidates, err := queryCandidates(normalized)
	if err != nil {
		log.Printf("DB: erro ao buscar candidatos para '%s': %v", name, err)
		return 0, false
	}
	if len(candidates) == 0 {
		log.Printf("DB: nenhum candidato encontrado para '%s'", name)
		return 0, false
	}

	bestScore := 0.0
	bestID := 0
	bestName := ""
	for _, item := range candidates {
		score := nameSimilarity(normalized, item.Normalized)
		if score > bestScore {
			bestScore = score
			bestID = item.ID
			bestName = item.Name
		}
	}

	if bestScore < 0.6 {
		log.Printf("DB: score insuficiente para '%s' (melhor: '%s' score=%.2f)", name, bestName, bestScore)
		return 0, false
	}
	log.Printf("DB: item '%s' → '%s' (id=%d, score=%.2f)", name, bestName, bestID, bestScore)
	return bestID, true
}

// nameSimilarity retorna um valor entre 0 e 1.
func nameSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	if strings.Contains(b, a) || strings.Contains(a, b) {
		shorter, longer := len(a), len(b)
		if shorter > longer {
			shorter, longer = longer, shorter
		}
		return float64(shorter) / float64(longer)
	}
	return levenshteinSimilarity(a, b)
}

func levenshteinSimilarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)

	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = minInt(d[i-1][j]+1, minInt(d[i][j-1]+1, d[i-1][j-1]+cost))
		}
	}
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	return 1.0 - float64(d[la][lb])/float64(maxLen)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SavePricesToDB salva os preços capturados na tabela ItemServerPrice.
func SavePricesToDB(server, itemName string, tiers []PriceTier) {
	if server == "" {
		log.Printf("DB: servidor não definido, pulando save para '%s'", itemName)
		return
	}
	if err := initDB(); err != nil {
		log.Printf("DB: erro ao inicializar: %v", err)
		return
	}

	itemID, ok := findItemID(itemName)
	if !ok {
		log.Printf("DB: item não encontrado no banco para '%s'", itemName)
		return
	}

	var price1, price10, price100, price1000 sql.NullInt64
	for _, t := range tiers {
		switch t.Qty {
		case 1:
			price1 = sql.NullInt64{Int64: t.Price, Valid: true}
		case 10:
			price10 = sql.NullInt64{Int64: t.Price, Valid: true}
		case 100:
			price100 = sql.NullInt64{Int64: t.Price, Valid: true}
		case 1000:
			price1000 = sql.NullInt64{Int64: t.Price, Valid: true}
		}
	}

	res, err := globalDB.Exec(`
		UPDATE "ItemServerPrice"
		SET price1=$3, price10=$4, price100=$5, price1000=$6, "updatedAt"=NOW()
		WHERE "itemId"=$1 AND server=$2`,
		itemID, server, price1, price10, price100, price1000,
	)
	if err != nil {
		log.Printf("DB: erro ao atualizar preço para '%s': %v", itemName, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_, err = globalDB.Exec(`
			INSERT INTO "ItemServerPrice" ("itemId", server, price1, price10, price100, price1000, "createdAt", "updatedAt")
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			itemID, server, price1, price10, price100, price1000,
		)
		if err != nil {
			log.Printf("DB: erro ao inserir preço para '%s': %v", itemName, err)
			return
		}
	}
	log.Printf("DB: preços salvos — '%s' server=%s id=%d", itemName, server, itemID)
}
