package main

import (
	"context"
	"log"
	"os"

	"telegrambot_supabase/internal/bot"
	"telegrambot_supabase/internal/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Загрузить .env
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ .env не найден, продолжаем без него")
	}

	// Подключение к Supabase
	conn := db.InitDB()
	defer conn.Close(context.Background())

	// Telegram API
	botToken := os.Getenv("TELEGRAM_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_TOKEN не указан в .env")
	}

	botAPI, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	botAPI.Debug = true
	log.Printf("✅ Авторизован как %s", botAPI.Self.UserName)

	// Установить команды в Telegram
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Начать работу"},
		{Command: "students", Description: "Список учеников и управление"},
	}

	_, err = botAPI.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Printf("⚠️ Не удалось установить команды: %v", err)
	}

	// Получение обновлений
	updateCfg := tgbotapi.NewUpdate(0)
	updateCfg.Timeout = 60

	updates := botAPI.GetUpdatesChan(updateCfg)

	for update := range updates {
		if update.CallbackQuery != nil {
			bot.HandleCallback(botAPI, update, conn)
		} else if update.Message != nil {
			bot.HandleUpdate(botAPI, update, conn)
		}
	}
}
