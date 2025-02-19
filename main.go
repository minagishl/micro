package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

//────────────────────────────
// ENV & Configuration
//────────────────────────────

type Env struct {
	RunMode               string
	DiscordWebhookURL     string
	DiscordMentionEnabled bool
	TargetPrefectures     []string
	EnableLogger          bool
}

var env Env

func loadEnv() {
	// Load .env file if exists (otherwise ignore)
	_ = godotenv.Load()
	env.RunMode = os.Getenv("RUN_MODE")
	env.DiscordWebhookURL = os.Getenv("DISCORD_WEBHOOK_URL")
	env.DiscordMentionEnabled = os.Getenv("DISCORD_MENTION_ENABLED") == "true"
	target := os.Getenv("TARGET_PREFECTURES")
	if target != "" {
		parts := strings.Split(target, ",")
		for i, s := range parts {
			parts[i] = strings.TrimSpace(s)
		}
		env.TargetPrefectures = parts
	}
	enableLogger := os.Getenv("ENABLE_LOGGER")
	if enableLogger == "" {
		env.EnableLogger = true
	} else {
		env.EnableLogger = enableLogger == "true"
	}
}

//────────────────────────────
// Type Definitions (Basic Data, Earthquake Info, Discord Messages, etc.)
//────────────────────────────

type BasicData struct {
	ID   string `json:"id"`
	Code int    `json:"code"`
	Time string `json:"time"`
}

type Issue struct {
	Source  string `json:"source,omitempty"`
	Time    string `json:"time"`
	Type    string `json:"type"`
	Correct string `json:"correct,omitempty"`
}

type Hypocenter struct {
	Name      string  `json:"name,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Depth     float64 `json:"depth,omitempty"`
	Magnitude float64 `json:"magnitude,omitempty"`
}

type Earthquake struct {
	Time            string      `json:"time"`
	Hypocenter      *Hypocenter `json:"hypocenter,omitempty"`
	MaxScale        int         `json:"maxScale"`
	DomesticTsunami string      `json:"domesticTsunami,omitempty"`
	ForeignTsunami  string      `json:"foreignTsunami,omitempty"`
}

type Point struct {
	Pref   string `json:"pref"`
	Addr   string `json:"addr"`
	IsArea bool   `json:"isArea"`
	Scale  int    `json:"scale"`
}

type JMAQuake struct {
	BasicData
	Issue      Issue      `json:"issue"`
	Earthquake Earthquake `json:"earthquake"`
	Points     []Point    `json:"points"`
}

type JMATsunami struct {
	BasicData
	Cancelled bool  `json:"cancelled"`
	Issue     Issue `json:"issue"`
	Areas     []struct {
		Grade       string `json:"grade,omitempty"`
		Immediate   bool   `json:"immediate,omitempty"`
		Name        string `json:"name,omitempty"`
		FirstHeight *struct {
			ArrivalTime string `json:"arrivalTime,omitempty"`
			Condition   string `json:"condition,omitempty"`
		} `json:"firstHeight,omitempty"`
		MaxHeight *struct {
			Description string  `json:"description,omitempty"`
			Value       float64 `json:"value,omitempty"`
		} `json:"maxHeight,omitempty"`
	} `json:"areas"`
}

// Discord message struct
type MessageField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type MessageBody struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Fields      []MessageField `json:"fields"`
	Color       int            `json:"color"`
}

type WebhookPayload struct {
	Content string        `json:"content,omitempty"`
	Embeds  []MessageBody `json:"embeds"`
}

// Result of grouping (highest intensity in each prefecture)
type PointGroup struct {
	ScaleInt int
	ScaleStr string
	Regions  []string
}

//────────────────────────────
// Parser & Conversion Functions
//────────────────────────────

var scaleMap = map[int]string{
	10: "1",
	20: "2",
	30: "3",
	40: "4",
	45: "5 weak",
	50: "5 strong",
	55: "6 weak",
	60: "6 strong",
	70: "7",
}

func parseScale(scale int) (string, bool) {
	s, ok := scaleMap[scale]
	return s, ok
}

// Simple translation (Prefecture name: Japanese → English)
var translateMap = map[string]string{
	"北海道":  "Hokkaido",
	"青森県":  "Aomori",
	"岩手県":  "Iwate",
	"宮城県":  "Miyagi",
	"秋田県":  "Akita",
	"山形県":  "Yamagata",
	"福島県":  "Fukushima",
	"茨城県":  "Ibaraki",
	"栃木県":  "Tochigi",
	"群馬県":  "Gunma",
	"埼玉県":  "Saitama",
	"千葉県":  "Chiba",
	"東京都":  "Tokyo",
	"神奈川県": "Kanagawa",
	"新潟県":  "Niigata",
	"富山県":  "Toyama",
	"石川県":  "Ishikawa",
	"福井県":  "Fukui",
	"山梨県":  "Yamanashi",
	"長野県":  "Nagano",
	"岐阜県":  "Gifu",
	"静岡県":  "Shizuoka",
	"愛知県":  "Aichi",
	"三重県":  "Mie",
	"滋賀県":  "Shiga",
	"京都府":  "Kyoto",
	"大阪府":  "Osaka",
	"兵庫県":  "Hyogo",
	"奈良県":  "Nara",
	"和歌山県": "Wakayama",
	"鳥取県":  "Tottori",
	"島根県":  "Shimane",
	"岡山県":  "Okayama",
	"広島県":  "Hiroshima",
	"山口県":  "Yamaguchi",
	"徳島県":  "Tokushima",
	"香川県":  "Kagawa",
	"愛媛県":  "Ehime",
	"高知県":  "Kochi",
	"福岡県":  "Fukuoka",
	"佐賀県":  "Saga",
	"長崎県":  "Nagasaki",
	"熊本県":  "Kumamoto",
	"大分県":  "Oita",
	"宮崎県":  "Miyazaki",
	"鹿児島県": "Kagoshima",
	"沖縄県":  "Okinawa",
}

func translate(pref string) string {
	if t, ok := translateMap[pref]; ok {
		return t
	}
	return pref
}

func parsePoints(points []Point) []PointGroup {
	// Record the highest scale received in each prefecture
	highest := make(map[string]int)
	for _, p := range points {
		if _, ok := parseScale(p.Scale); ok {
			if cur, exists := highest[p.Pref]; !exists || p.Scale > cur {
				highest[p.Pref] = p.Scale
			}
		}
	}

	// Grouping translated prefecture names by scale
	groupsMap := make(map[int][]string)
	for pref, scaleVal := range highest {
		groupsMap[scaleVal] = append(groupsMap[scaleVal], translate(pref))
	}
	var groups []PointGroup
	for scaleVal, regions := range groupsMap {
		scaleStr, _ := parseScale(scaleVal)
		groups = append(groups, PointGroup{ScaleInt: scaleVal, ScaleStr: scaleStr, Regions: regions})
	}

	// Sort by intensity (e.g. low intensity -> high intensity)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ScaleInt < groups[j].ScaleInt
	})
	return groups
}

//────────────────────────────
// Discord Message Creation & Sending Functions
//────────────────────────────

func createEarthquakeMessage(timeStr, scale string, groups []PointGroup, isDev bool) MessageBody {
	t, err := time.Parse("2006/01/02 15:04:05", timeStr)
	if err != nil {
		t = time.Now()
	}
	formattedTime := fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
	formattedDate := fmt.Sprintf("%04d/%02d/%02d", t.Year(), t.Month(), t.Day())
	prefix := ""
	if isDev {
		prefix = "This information is a test distribution\n"
	}
	description := fmt.Sprintf("%sMaximum intensity %s was received at %s on %s.", prefix, scale, formattedTime, formattedDate)
	var fields []MessageField

	// Sort region names in each group alphabetically
	for _, g := range groups {
		sort.Strings(g.Regions)
		fields = append(fields, MessageField{
			Name:   fmt.Sprintf("Seismic Intensity %s", g.ScaleStr),
			Value:  strings.Join(g.Regions, ", "),
			Inline: true,
		})
	}

	return MessageBody{
		Title:       "Earthquake Information",
		Description: description,
		Fields:      fields,
		Color:       2264063,
	}
}

func sendWebhook(body MessageBody, urlStr string) bool {
	payload := WebhookPayload{
		Embeds: []MessageBody{body},
	}
	if env.DiscordMentionEnabled {
		payload.Content = "@everyone"
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshalling payload:", err)
		return false
	}
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(data))
	if err != nil {
		log.Println("Error creating request:", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending webhook request:", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Println("Webhook error, status code:", resp.StatusCode)
		return false
	}
	return true
}

func sendMessage(body MessageBody) error {
	if env.DiscordWebhookURL == "" {
		return nil
	}
	webhookUrls := strings.Split(env.DiscordWebhookURL, ",")
	// If target prefectures are set, check if the message contains any of them
	if len(env.TargetPrefectures) > 0 {
		var affected []string
		for _, field := range body.Fields {
			parts := strings.Split(field.Value, ", ")
			affected = append(affected, parts...)
		}
		shouldSend := false
		for _, target := range env.TargetPrefectures {
			for _, a := range affected {
				if a == target {
					shouldSend = true
					break
				}
			}
			if shouldSend {
				break
			}
		}
		if !shouldSend {
			if env.EnableLogger {
				log.Println("No target prefectures affected, skipping webhook")
			}
			return nil
		}
	}
	successCount := 0
	for _, url := range webhookUrls {
		url = strings.TrimSpace(url)
		if !sendWebhook(body, url) {
			log.Println("Failed to send webhook:", url)
		} else {
			successCount++
		}
	}
	if env.EnableLogger {
		log.Printf("Webhook sent (%d/%d)\n", successCount, len(webhookUrls))
	}
	return nil
}

func handleEarthquake(eq JMAQuake, isDev bool) {
	groups := parsePoints(eq.Points)
	t := eq.Earthquake.Time
	scale, ok := parseScale(eq.Earthquake.MaxScale)
	if !ok {
		log.Println("Earthquake scale is undefined.")
		return
	}
	body := createEarthquakeMessage(t, scale, groups, isDev)
	if err := sendMessage(body); err != nil {
		log.Println("Error sending message:", err)
	} else if env.EnableLogger {
		log.Println("Earthquake alert received and posted successfully.")
	}
}

//────────────────────────────
// WebSocket Connection & Reconnection Handler
//────────────────────────────

func onMessage(message []byte, isDev bool) {
	if isDev {
		log.Println("Message received from server.")
	}
	// Parse to a generic map once to check the code
	var data map[string]interface{}
	if err := json.Unmarshal(message, &data); err != nil {
		log.Println("Error parsing message:", err)
		return
	}
	code, ok := data["code"].(float64)
	if !ok {
		log.Println("Message does not contain a valid code")
		return
	}
	if int(code) == 551 {
		var quake JMAQuake
		if err := json.Unmarshal(message, &quake); err != nil {
			log.Println("Error parsing earthquake message:", err)
			return
		}
		handleEarthquake(quake, isDev)
	} else {
		if isDev {
			log.Println("Unknown message code:", code)
		}
	}
}

func connectAndHandle(isDev bool) error {
	var wsURL string
	if isDev {
		wsURL = "wss://api-realtime-sandbox.p2pquake.net/v2/ws"
	} else {
		wsURL = "wss://api.p2pquake.net/v2/ws"
	}

	log.Println("Connecting to", wsURL)
	c, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)

	if err != nil {
		if resp != nil {
			return fmt.Errorf("%v (HTTP %d)", err, resp.StatusCode)
		}
		return err
	}

	defer c.Close()
	log.Println("WebSocket connection opened.")

	// Loop to receive messages
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			return err
		}
		// Process each message in a separate goroutine
		go onMessage(msg, isDev)
	}
}

func main() {
	loadEnv()

	// Check DISCORD_WEBHOOK_URL
	if env.DiscordWebhookURL == "" {
		log.Fatal("DISCORD_WEBHOOK_URL is not set.")
	} else {
		valid := true
		urls := strings.Split(env.DiscordWebhookURL, ",")
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if !strings.HasPrefix(u, "https://discord.com/api/webhooks/") {
				valid = false
				break
			}
		}
		if !valid {
			log.Fatal("DISCORD_WEBHOOK_URL is not valid.")
		}
	}

	isDev := env.RunMode == "development"
	log.Printf("Now running in %s mode.\n", func() string {
		if isDev {
			return "development"
		}
		return "production"
	}())

	reconnectAttempts := 0
	baseReconnectDelay := 5 * time.Second
	maxReconnectDelay := 30 * time.Second

	// WebSocket connection and reconnection loop
	for {
		err := connectAndHandle(isDev)
		if err != nil {
			log.Println("WebSocket connection error:", err)
		}
		// Exponential backoff
		delay := time.Duration(float64(baseReconnectDelay) * math.Pow(2, float64(reconnectAttempts)))
		if delay > maxReconnectDelay {
			delay = maxReconnectDelay
		}
		log.Printf("Reconnecting in %v...\n", delay)
		time.Sleep(delay)
		reconnectAttempts++
		log.Println("Attempting to reconnect...")
	}
}
