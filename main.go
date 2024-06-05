package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// Buffer представляет буфер для хранения данных и их последующей отправки на API
type Buffer struct {
	data      []map[string]string
	mu        sync.Mutex
	cond      *sync.Cond
	url       string
	token     string
	isSending bool
	wg        *sync.WaitGroup
}

// Messages представляет структуру сообщений в ответе API
type Messages struct {
	Error   interface{} `json:"error"`
	Warning interface{} `json:"warning"`
	Info    []string    `json:"info"`
}

// Data представляет структуру данных в ответе API
type Data struct {
	IndicatorToMoFactID int `json:"indicator_to_mo_fact_id"`
}

// APIResponse представляет структуру ответа API
type APIResponse struct {
	Messages Messages `json:"MESSAGES"`
	Data     Data     `json:"DATA"`
	Status   string   `json:"STATUS"`
}

// NewBuffer создает новый буфер с заданными URL и токеном
func NewBuffer(apiURL, token string) *Buffer {
	b := &Buffer{
		data:  make([]map[string]string, 0),
		url:   apiURL,
		token: token,
		wg:    &sync.WaitGroup{},
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Add добавляет новый элемент в буфер и запускает отправку данных, если она не выполняется
func (b *Buffer) Add(item map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, item)
	if !b.isSending {
		b.isSending = true
		b.wg.Add(1)
		go b.sendData()
	}
}

// sendData отправляет данные из буфера на API
func (b *Buffer) sendData() {
	defer b.wg.Done()
	b.mu.Lock()
	if len(b.data) == 0 {
		b.isSending = false
		b.mu.Unlock()
		return
	}
	item := b.data[0]
	b.data = b.data[1:]
	b.mu.Unlock()

	b.sendToAPI(item)

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) == 0 {
		b.isSending = false
	} else {
		b.wg.Add(1)
		go b.sendData()
	}
}

// sendToAPI выполняет отправку одного элемента данных на API
func (b *Buffer) sendToAPI(data map[string]string) {
	client := &http.Client{}
	formData := url.Values{}
	for key, value := range data {
		formData.Set(key, value)
	}

	req, err := http.NewRequest("POST", b.url, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+b.token)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	var response APIResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error response from API:", resp.Status)
		return
	}

	fmt.Println("Data successfully sent to API", string(body))
}

// getFacts отправляет запрос на получение данных с сервера
func getFacts(apiURL, token string, params map[string]string) {
	client := &http.Client{}
	formData := url.Values{}
	for key, value := range params {
		formData.Set(key, value)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error response from API:", resp.Status)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println(string(body))
}

// main функция программы
func main() {
	buffer := NewBuffer("https://development.kpi-drive.ru/_api/facts/save_fact", "48ab34464a5573519725deb5865cc74c")

	// Добавление 10 записей в буфер
	for i := 0; i < 10; i++ {
		data := map[string]string{
			"period_start":            "2024-05-01",
			"period_end":              "2024-05-31",
			"period_key":              "month",
			"indicator_to_mo_id":      "227373",
			"indicator_to_mo_fact_id": "0",
			"value":                   fmt.Sprintf("%d", i+1),
			"fact_time":               "2024-05-31",
			"is_plan":                 "0",
			"auth_user_id":            "40",
			"comment":                 "buffer Last_name",
		}
		buffer.Add(data)
	}

	// Параметры для запроса получения данных
	// params := map[string]string{
	// 	"period_start":       "2024-05-01",
	// 	"period_end":         "2024-05-31",
	// 	"period_key":         "month",
	// 	"indicator_to_mo_id": "227373",
	// }

	// Выполнение запроса получения данных
	// getFacts("https://development.kpi-drive.ru/_api/indicators/get_facts", "48ab34464a5573519725deb5865cc74c", params)

	// Ожидание завершения всех отправок данных
	buffer.wg.Wait()

	// Ожидание ввода пользователя для завершения программы
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Введите 'exit' для завершения программы:")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "exit" {
			fmt.Println("Завершение программы...")
			break
		}
	}
}
