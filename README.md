# bot_keuangan

Bot Telegram pribadi untuk mencatat pemasukan dan pengeluaran ke Google Sheets, lalu menampilkan ringkasan keuangan secara langsung.

## Fitur

- Membuat sheet baru dengan command `/sheet`
- Mencatat pemasukan dengan command `/masuk`
- Mencatat pengeluaran dengan command `/keluar`
- Membatasi akses hanya untuk `ADMIN_ID`
- Menampilkan ringkasan total masuk, keluar, sisa, dan breakdown kategori

## Skema Arsitektur

```mermaid
flowchart LR
    A[User / Admin Telegram] -->|/start /sheet /masuk /keluar| B[Telegram Bot]
    B --> C[Middleware Validasi ADMIN_ID]
    C --> D[Bot Handler Golang]
    D -->|Buat sheet / tambah transaksi| E[Google Sheets API]
    E --> F[(Spreadsheet Keuangan)]
    D -->|Generate summary| F
    D -->|Balas hasil & ringkasan| A
```

## Skema Alur Aplikasi

```mermaid
flowchart TD
    A[Bot dijalankan] --> B[Load .env]
    B --> C[Inisialisasi Google Sheets Service]
    C --> D[Inisialisasi Telegram Bot]
    D --> E[Menunggu command dari user]

    E --> F{User = ADMIN_ID?}
    F -->|Tidak| G[Akses ditolak]
    F -->|Ya| H{Jenis command}

    H -->|/sheet| I[Buat sheet baru]
    I --> J[Isi header kolom]
    J --> K[Balas sukses ke Telegram]

    H -->|/masuk| L[Validasi input pemasukan]
    L --> M[Append data ke Google Sheets]
    M --> N[Hitung ulang ringkasan]
    N --> O[Balas ringkasan ke Telegram]

    H -->|/keluar| P[Validasi input pengeluaran]
    P --> Q[Append data ke Google Sheets]
    Q --> R[Hitung ulang ringkasan]
    R --> S[Balas ringkasan ke Telegram]
```

## Struktur Data Google Sheets

Setiap sheet yang dibuat akan memiliki header berikut:

| Tanggal | Kategori | Keterangan | Masuk | Keluar |
|---|---|---|---:|---:|
| 13-04-2026 09:30 | Pemasukan | Gaji | 5000000 | 0 |
| 13-04-2026 12:15 | Makan | Makan siang | 0 | 25000 |

## Command Bot

- `/start` menampilkan bantuan penggunaan bot
- `/sheet [nama_sheet]` membuat sheet baru
- `/masuk [nama_sheet] [nominal] [keterangan]` mencatat pemasukan
- `/keluar [nama_sheet] [nominal] [kategori] [keterangan]` mencatat pengeluaran

Contoh:

```bash
/sheet Tabungan
/masuk Tabungan 50000 Gaji
/keluar Tabungan 15000 Makan Nasi padang
```

## Environment Variable

Gunakan file `env-example` sebagai referensi:

```env
SPREADSHEET_ID=xxxxxxxx
TOKEN=xxxxxx
ADMIN_ID=xxxxx
```

## Teknologi

- Golang
- `gopkg.in/telebot.v3`
- Google Sheets API
- `github.com/joho/godotenv`
