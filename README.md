# 📸 Spy Server (Golang)
## 🧩 Deskripsi
Proyek ini adalah server berbasis Golang yang berfungsi untuk menampilkan halaman landing sementara, meminta izin kamera dari pengguna (depan dan belakang), mengambil foto, lalu mengirim hasil foto beserta informasi lokasi dan IP publik ke Telegram melalui Bot API.

Server juga menggunakan layanan [IP2Location](https://www.ip2location.io/) untuk mendapatkan informasi geolokasi dari alamat IP pengguna.

## ⚙️ Fitur Utama
- 🌐 Menyajikan landing page interaktif dengan animasi "loading".
- 📸 Mengambil gambar otomatis dari kamera depan atau belakang pengguna.
- 🧭 Mendapatkan data lokasi pengguna (negara, kota, koordinat GPS) menggunakan IP publik.
- 🗺️ Menyertakan tautan Google Maps berdasarkan koordinat lokasi pengguna.
- 🤖 Mengirimkan foto dan data lokasi ke Telegram secara otomatis.
- 🔐 Menggunakan variabel .env untuk menjaga keamanan token dan konfigurasi.

## 🧾 Persyaratan
Pastikan sistem kamu sudah terpasang:
- [Go 1.18+](https://go.dev/dl/)
- (Git)[https://git-scm.com/]
- Akses internet
- Akun dan Bot Telegram aktif
- Token API dari [IP2Location](https://www.ip2location.io/)

## ⚙️ Instalasi dan Konfigurasi
1. Clone Repository
```bash
git clone https://github.com/username/project-name.git
cd project-name
```
2. Instal Dependensi
```bash
go mod tidy
```
3. Buat File .env
Buat file bernama .env di root folder proyek, contohnya ada pada `env_example`

## 🚀 Menjalankan Server
```bash
go run main.go
```

## 🌍 Cara Menggunakan
1. Jalankan server (go run main.go)
2. Buka browser dan akses URL seperti `http://localhost:8080/?url=https://google.com`
3. Halaman akan menampilkan pesan “Sedang Memuat...”, meminta izin kamera, lalu:
   - Mengambil foto pengguna.
   - Mengirim foto + data lokasi ke bot Telegram kamu.
   - Mengarahkan pengguna ke URL asli (contoh: Google).

## 📬 Pesan Telegram yang dikirim
```bash
IP Address   : 123.45.67.89
From URL     : https://google.com
URL Host     : http://yourserver.com/?url=https://google.com

Location
Country Code : ID
Country Name : Indonesia
Region Name  : Jawa Timur
City Name    : Surabaya
Latitude     : -7.2575
Longitude    : 112.7521
Maps         : https://www.google.co.id/maps/place/-7.2575,112.7521

Network
AS/ISP Number: AS12345
AS/ISP       : PT Telkom Indonesia
Is Proxy     : false
```

## 🧰 Dependensi Go
- [github.com/joho/godotenv](https://github.com/joho/godotenv) untuk membaca file .env
- Paket bawaan Go `net/http, encoding/json, mime/multipart, log, os, io, time, dll.`

## ⚠️ Catatan Penting
### 📜 Disclaimer:
Script ini hanya untuk tujuan pembelajaran dan penelitian seputar interaksi web dan kamera browser.
Dilarang menggunakan kode ini untuk aktivitas yang melanggar privasi atau hukum.

## 🧑‍💻 Lisensi
Lisensi: MIT License
Kamu bebas menggunakan, memodifikasi, dan mendistribusikan kode ini dengan menyertakan atribusi kepada pembuatnya.
