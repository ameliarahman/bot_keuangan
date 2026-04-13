package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gopkg.in/telebot.v3"
)

type Config struct {
	SpreadsheetID string
	Token         string
	AdminID       int64
}

type BotHandler struct {
	srv    *sheets.Service
	config Config
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("File .env tidak ditemukan")
	}

	// admin_id is telegram user id
	adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_ID"), 10, 64)
	cfg := Config{
		SpreadsheetID: os.Getenv("SPREADSHEET_ID"),
		Token:         os.Getenv("TOKEN"),
		AdminID:       adminID,
	}

	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithAuthCredentialsFile(option.ServiceAccount, "credential.json"))
	if err != nil {
		log.Fatalf("❌ Gagal inisialisasi Sheets: %v", err)
	}

	h := &BotHandler{srv: srv, config: cfg}

	b, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.Token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	// --- Handlers ---

	private := b.Group()
	private.Use(authMiddleware(cfg.AdminID))

	private.Handle("/start", func(c telebot.Context) error {
		return c.Send("📊 *Bot Keuangan Multi-Sheet*\n\n"+
			"1️⃣ *Buat Sheet:* `/sheet [nama]`\n"+
			"2️⃣ *Pemasukan:* `/masuk [nama_sheet] [nominal] [ket]`\n"+
			"3️⃣ *Pengeluaran:* `/keluar [nama_sheet] [nominal] [kategori] [ket]`\n\n"+
			"Contoh: `/masuk Tabungan 50000 Gaji`", telebot.ModeMarkdown)
	})

	// Command untuk membuat sheet baru

	private.Handle("/sheet", h.handleCreateSheet)
	private.Handle("/masuk", h.handleEntry(true))
	private.Handle("/keluar", h.handleEntry(false))

	fmt.Println("🚀 Bot Berjalan...")
	b.Start()
}

func authMiddleware(adminID int64) telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			if c.Sender().ID != adminID {
				log.Printf("Akses ditolak untuk: %s (%d)", c.Sender().Username, c.Sender().ID)
				return nil
			}
			return next(c)
		}
	}
}

func (h *BotHandler) handleCreateSheet(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return c.Send("❌ Gunakan format: `/buat [nama_sheet]`")
	}

	sheetName := args[0]

	addSheetReq := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{Title: sheetName},
				},
			},
		},
	}

	_, err := h.srv.Spreadsheets.BatchUpdate(h.config.SpreadsheetID, addSheetReq).Do()
	if err != nil {
		return c.Send("❌ Gagal membuat sheet (mungkin sudah ada atau nama tidak valid).")
	}

	header := []interface{}{"Tanggal", "Kategori", "Keterangan", "Masuk", "Keluar"}
	valueRange := &sheets.ValueRange{Values: [][]interface{}{header}}
	h.srv.Spreadsheets.Values.Append(h.config.SpreadsheetID, sheetName+"!A1", valueRange).ValueInputOption("USER_ENTERED").Do()

	return c.Send(fmt.Sprintf("✅ Sheet *%s* berhasil dibuat!", sheetName), telebot.ModeMarkdown)
}

func (h *BotHandler) handleEntry(isIncome bool) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		args := c.Args()

		// Validasi argumen (NamaSheet + Nominal + Sisanya)
		minArgs := 3 // /masuk [sheet] [nominal] [ket]
		if !isIncome {
			minArgs = 4 // /keluar [sheet] [nominal] [kat] [ket]
		}

		if len(args) < minArgs {
			if isIncome {
				return c.Send("❌ Format: `/masuk [nama_sheet] [nominal] [keterangan]`")
			}
			return c.Send("❌ Format: `/keluar [nama_sheet] [nominal] [kategori] [keterangan]`")
		}

		sheetName := args[0]
		cleanAmount := strings.NewReplacer(".", "", ",", "").Replace(args[1])
		amount, err := strconv.ParseFloat(cleanAmount, 64)
		if err != nil {
			return c.Send("❌ Nominal harus berupa angka.")
		}

		var category, desc string
		if isIncome {
			category = "Pemasukan"
			desc = strings.Join(args[2:], " ")
		} else {
			category = args[2]
			desc = strings.Join(args[3:], " ")
		}

		// Input Data
		date := time.Now().Format("02-01-2006 15:04")
		row := []interface{}{date, category, desc, 0, 0}
		if isIncome {
			row[3] = amount
		} else {
			row[4] = amount
		}

		valueRange := &sheets.ValueRange{Values: [][]interface{}{row}}
		_, err = h.srv.Spreadsheets.Values.Append(h.config.SpreadsheetID, sheetName+"!A1", valueRange).ValueInputOption("USER_ENTERED").Do()

		if err != nil {
			return c.Send("❌ Gagal mencatat. Pastikan nama sheet benar (sudah dibuat dengan /sheet).")
		}

		return c.Send(h.generateSummary(sheetName), telebot.ModeMarkdown)
	}
}

func (h *BotHandler) generateSummary(sheetName string) string {
	// Get all rows from A2 to E (skipping header)
	resp, err := h.srv.Spreadsheets.Values.Get(h.config.SpreadsheetID, sheetName+"!A2:E").Do()
	if err != nil || len(resp.Values) == 0 {
		return "✅ Catatan berhasil ditambahkan."
	}

	var totalIn, totalOut float64
	categoryBreakdown := make(map[string]float64)

	for _, row := range resp.Values {
		// Ensure the row has enough columns
		if len(row) < 5 {
			continue
		}

		// Parse Masuk (Col D / Index 3) and Keluar (Col E / Index 4)
		inStr := strings.NewReplacer(".", "", ",", "").Replace(fmt.Sprint(row[3]))
		outStr := strings.NewReplacer(".", "", ",", "").Replace(fmt.Sprint(row[4]))

		in, _ := strconv.ParseFloat(inStr, 64)
		out, _ := strconv.ParseFloat(outStr, 64)

		totalIn += in
		totalOut += out

		// Group by Category (Col B / Index 1) if it's an expense
		if out > 0 {
			catName := fmt.Sprint(row[1])
			if catName == "" {
				catName = "Lain-lain"
			}
			categoryBreakdown[catName] += out
		}
	}

	// Format the final message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 *RINGKASAN: %s*\n", strings.ToUpper(sheetName)))
	sb.WriteString(fmt.Sprintf("───────────────────\n"))
	sb.WriteString(fmt.Sprintf("📥 *Masuk:* Rp %.0f\n", totalIn))
	sb.WriteString(fmt.Sprintf("📤 *Keluar:* Rp %.0f\n", totalOut))
	sb.WriteString(fmt.Sprintf("⚖️ *Sisa:* Rp %.0f\n", totalIn-totalOut))
	sb.WriteString(fmt.Sprintf("───────────────────\n"))
	sb.WriteString("*Detail Per Kategori:*\n")

	if len(categoryBreakdown) == 0 {
		sb.WriteString("_Belum ada data pengeluaran._")
	} else {
		for cat, total := range categoryBreakdown {
			sb.WriteString(fmt.Sprintf("• %s: Rp %.0f\n", cat, total))
		}
	}

	return sb.String()
}
