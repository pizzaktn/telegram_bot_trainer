package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"telegrambot_supabase/internal/bot"
	"telegrambot_supabase/internal/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	conn := db.InitDB()
	defer conn.Close(context.Background())

	botToken := os.Getenv("TELEGRAM_TOKEN")
	publicURL := os.Getenv("PUBLIC_URL") // например: https://your-app.onrender.com

	botAPI, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	// Устанавливаем webhook
	webhookURL := publicURL + "/webhook"
	webhook, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		log.Fatalf("Ошибка установки Webhook: %v", err)
	}
	_, err = botAPI.Request(webhook)
	if err != nil {
		log.Fatalf("Ошибка установки Webhook: %v", err)
	}

	// HTTP handler
	updates := botAPI.ListenForWebhook("/webhook")

	go func() {
		log.Println("Запускаем сервер на :8080...")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}()

	for update := range updates {
		if update.CallbackQuery != nil {
			bot.HandleCallback(botAPI, update, conn)
		} else if update.Message != nil {
			bot.HandleUpdate(botAPI, update, conn)
		}
	}
}
