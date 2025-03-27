package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/joho/godotenv"
)

// MeterProurl用の構造体
type MeterProurl struct {
	Version     string `json:"version"`
	Temperature float32    `json:"temperature"`
	Battery     int    `json:"battery"`
	Humidity    int    `json:"humidity"`
	CO2         int    `json:"CO2"`
	DeviceId    string `json:"deviceId"`
	DeviceType  string `json:"deviceType"`
	HubDeviceId string `json:"hubDeviceId"`
}

// WoIOSensor用の構造体
type WoIOSensor struct {
	Version     string `json:"version"`
	Temperature float32    `json:"temperature"`
	Battery     int    `json:"battery"`
	Humidity    int    `json:"humidity"`
	DeviceId    string `json:"deviceId"`
	DeviceType  string `json:"deviceType"`
	HubDeviceId string `json:"hubDeviceId"`
}

func main() {
	err := godotenv.Load(".env")
	var MeterProurl string = os.Getenv("MeterProurl")
	var WoIOSensor string = os.Getenv("WoIOSensor")
	var switchbottoken string = os.Getenv("switchbottoken")
	var switchbotsecret string = os.Getenv("switchbotsecret")

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			responseData, err := parseJSONMeterPro(MeterProurl, switchbottoken, switchbotsecret)
			responseDatawoi, err := parseJSONWoIOSensor(WoIOSensor, switchbottoken, switchbotsecret)
			if err != nil {
				return
			}
			connectToInfluxDB(responseData)
			connectToInfluxDBwoi(responseDatawoi)
		}
	}
}

func parseJSONMeterPro(url, token, secret string)(MeterProurl, error){
	body,err := getJSONResponse(url,token,secret)
	if err != nil {
		log.Fatalf("JSON marshaling failed: %s", err)
	}
	// MeterProurl構造体にデータを格納
	var meterData MeterProurl
	
	if err := json.Unmarshal(body, &meterData); err != nil {
		log.Fatalf("JSON unmarshaling failed: %s", err)
	}

	return meterData, nil
}

func parseJSONWoIOSensor(url, token, secret string)(WoIOSensor, error){
	body,err := getJSONResponse(url,token,secret)
	if err != nil {
		log.Fatalf("JSON marshaling failed: %s", err)
	}
	// MeterProurl構造体にデータを格納
	var woiosData WoIOSensor
	
	if err := json.Unmarshal(body, &woiosData); err != nil {
		log.Fatalf("JSON unmarshaling failed: %s", err)
	}

	return woiosData, nil
}

func getJSONResponse(url, token, secret string) ([]byte, error) {
	// UUIDを生成
	uuidV1, err := uuid.NewRandom()
	if err != nil {
		return []byte{}, fmt.Errorf("failed to generate UUID: %v", err)
	}
	nonce := uuidV1.String()

	// 現在のタイムスタンプ
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())

	// HMAC-SHA256署名の作成
	data := token + timestamp + nonce
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	signature := b64.StdEncoding.EncodeToString(mac.Sum(nil))

	// HTTPリクエスト作成
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("sign", signature)
	req.Header.Set("nonce", nonce)
	req.Header.Set("t", timestamp)

	// HTTPクライアントでリクエストを送信
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// レスポンスのボディ(JSON部分)を取得
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(response, &m); err != nil {
		log.Fatalf("JSON unmarshaling failed: %s", err)
	}

	body, ok := m["body"].(map[string]interface{})
	if !ok {
		log.Fatal("body not found or not a map")
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
			return []byte{}, fmt.Errorf("failed to marshal body to JSON: %v", err)
	}

	return bodyBytes, nil
}

func connectToInfluxDB(data MeterProurl) {
	dbToken := os.Getenv("INFLUXDB_TOKEN")
	dbURL := os.Getenv("INFLUXDB_URL")
	dbORG := os.Getenv("INFLUXDB_ORG")
	dbBucket := os.Getenv("INFLUXDB_Bucket")
	client := influxdb2.NewClient(dbURL, dbToken)
	defer client.Close()
	writeAPI := client.WriteAPIBlocking(dbORG, dbBucket)

	p := influxdb2.NewPointWithMeasurement("sensor_data").
		AddTag("device_id", data.DeviceId).
		AddField("temperature", data.Temperature).
		AddField("humidity", data.Humidity).
		AddField("co2", data.CO2).
		SetTime(time.Now())
	err := writeAPI.WritePoint(context.Background(), p)
	if err != nil {
		panic(err)
	}
}

func connectToInfluxDBwoi(data WoIOSensor) {
	dbToken := os.Getenv("INFLUXDB_TOKEN")
	dbURL := os.Getenv("INFLUXDB_URL")
	dbORG := os.Getenv("INFLUXDB_ORG")
	dbBucket := os.Getenv("INFLUXDB_Bucket")
	client := influxdb2.NewClient(dbURL, dbToken)
	defer client.Close()
	writeAPI := client.WriteAPIBlocking(dbORG, dbBucket)

	p := influxdb2.NewPointWithMeasurement("sensor_data").
		AddTag("device_id", data.DeviceId).
		AddField("temperature", data.Temperature).
		AddField("humidity", data.Humidity).
		AddField("battery", data.Battery).
		SetTime(time.Now())
	err := writeAPI.WritePoint(context.Background(), p)
	if err != nil {
		panic(err)
	}
}

//デバイス一覧を取得する関数
func getDeviceList(token, secret string) (string,error){
	// UUIDを生成
	uuidV1, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %v", err)
	}
	nonce := uuidV1.String()

	// 現在のタイムスタンプ
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())

	// HMAC-SHA256署名の作成
	data := token + timestamp + nonce
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	signature := b64.StdEncoding.EncodeToString(mac.Sum(nil))

	// HTTPリクエスト作成
	req, err := http.NewRequest("GET", "https://api.switch-bot.com/v1.1/devices", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("sign", signature)
	req.Header.Set("nonce", nonce)
	req.Header.Set("t", timestamp)

	// HTTPクライアントでリクエストを送信
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}
	return string(body), nil
}