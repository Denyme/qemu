package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"

	libvirt "libvirt.org/libvirt-go"
)

// Структура для хранения информации о домене
type DomainInfo struct {
	DomName string `json:"dom_name"`
	Status  string `json:"status"`
}

// Структура для JSON в сервер
type JsonInfo struct {
	DomainInfo []DomainInfo `json:"domain_info"` // переиспользуем DomainInfo для отправки в controlplane
	AgentName  string       `json:"agent_name"`
}

// Структура для конфигурации (YAML)
type Config struct {
	AgentName string `yaml:"agent_name"`
}

// Функция для получения списка всех доменов
func GetAllDomains() ([]DomainInfo, error) {
	// Подключение к локальному гипервизору
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %w", err)
	}
	defer conn.Close()

	// Получение всех доменов (активных и неактивных)
	flags := libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE
	domains, err := conn.ListAllDomains(flags)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	// Создание списка для хранения информации о доменах
	var domainList []DomainInfo

	for _, domain := range domains {
		name, err := domain.GetName()
		if err != nil {
			log.Printf("Failed to get domain name: %v", err)
			continue
		}

		// Проверка состояния домена
		state, _, err := domain.GetState()
		if err != nil {
			log.Printf("Failed to get domain state for %s: %v", name, err)
			continue
		}

		status := "inactive"
		if state == libvirt.DOMAIN_RUNNING || state == libvirt.DOMAIN_PAUSED || state == libvirt.DOMAIN_NOSTATE {
			status = "active"
		}

		// Добавление домена в список
		domainList = append(domainList, DomainInfo{
			DomName: name,
			Status:  status,
		})

		domain.Free()
	}

	return domainList, nil
}

// Функция для считывания конфигурации из YAML-файла
func LoadConfig(filePath string) (Config, error) {
	// Чтение файла с помощью os.ReadFile
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	// Декодирование YAML
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// Обработчик для GET /domainList
func domainListHandler(configFile string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		// Считываем конфиг
		config, err := LoadConfig(configFile)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
			return
		}

		// Получение списка доменов
		domains, err := GetAllDomains()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch domains: %v", err), http.StatusInternalServerError)
			return
		}

		// Создаем JSON-ответ
		response := JsonInfo{
			DomainInfo: domains,
			AgentName:  config.AgentName,
		}

		// Устанавливаем заголовки и отправляем JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func main() {
	// Определение флага -c для пути к конфигурационному файлу
	configFile := flag.String("c", "", "Path to the configuration file")
	flag.Parse()

	// Проверяем, что флаг -c был указан
	if *configFile == "" {
		log.Fatalf("Configuration file path is required. Use -c <path>")
	}

	// Проверяем существование файла конфигурации
	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		log.Fatalf("Config file '%s' does not exist", *configFile)
	}

	// Регистрируем маршрут с передачей пути к конфигурации
	http.HandleFunc("/domainList", domainListHandler(*configFile))

	// Запускаем HTTP-сервер
	log.Println("Starting server on :8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
