package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"
)

var pendingPayment = make(map[int64]int)
var pendingNewStudent = make(map[int64]bool)
var pendingScheduleStudent = make(map[int64]int)
var pendingScheduleDay = make(map[int64]int)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *pgx.Conn) {
	chatID := update.Message.Chat.ID
	text := update.Message.Text

	if pendingNewStudent[chatID] {
		delete(pendingNewStudent, chatID)
		name := strings.TrimSpace(text)
		if name == "" {
			bot.Send(tgbotapi.NewMessage(chatID, "Имя не может быть пустым."))
			return
		}
		_, err := db.Exec(context.Background(), "INSERT INTO students (name, telegram_id) VALUES ($1, $2)", name, chatID)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Ученик %s добавлен!", name)))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении ученика."))
		}
		return
	}

	if studentID, ok := pendingPayment[chatID]; ok {
		delete(pendingPayment, chatID)
		parts := strings.Fields(text)
		if len(parts) != 2 {
			bot.Send(tgbotapi.NewMessage(chatID, "Пример: 3000 5"))
			return
		}
		_, err := db.Exec(context.Background(), "UPDATE students SET total_paid_lessons = total_paid_lessons + $1, remaining_lessons = remaining_lessons + $2 WHERE id = $3", parts[0], parts[1], studentID)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "Оплата добавлена."))
		}
		return
	}

	if text == "/start" || text == "/students" {
		bot.Send(tgbotapi.NewMessage(chatID, "Список учеников:"))

		addBtn := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("\u2795 Добавить ученика", "add_new_student"),
			),
		)
		msg := tgbotapi.NewMessage(chatID, "Управление:")
		msg.ReplyMarkup = addBtn
		bot.Send(msg)

		rows, _ := db.Query(context.Background(), "SELECT id, name FROM students WHERE telegram_id = $1", chatID)
		for rows.Next() {
			var id int
			var name string
			rows.Scan(&id, &name)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("👤 %s", name))
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("📅 Расписание", fmt.Sprintf("schedule_%d", id)),
					tgbotapi.NewInlineKeyboardButtonData("💰 Оплата", fmt.Sprintf("payment_%d", id)),
					tgbotapi.NewInlineKeyboardButtonData("❌ Удалить", fmt.Sprintf("delete_%d", id)),
				),
			)
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
		}
		return
	}
}

func HandleCallback(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *pgx.Conn) {
	data := update.CallbackQuery.Data
	chatID := update.CallbackQuery.Message.Chat.ID
	msgID := update.CallbackQuery.Message.MessageID

	if data == "add_new_student" {
		pendingNewStudent[chatID] = true
		bot.Send(tgbotapi.NewMessage(chatID, "Введите имя нового ученика:"))
		return
	}

	if strings.HasPrefix(data, "delete_") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "delete_"))
		db.Exec(context.Background(), "DELETE FROM students WHERE id = $1", id)
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Ученик #%d удалён.", id)))
		bot.Request(tgbotapi.NewDeleteMessage(chatID, msgID))
		return
	}

	if strings.HasPrefix(data, "payment_") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "payment_"))
		pendingPayment[chatID] = id
		bot.Send(tgbotapi.NewMessage(chatID, "Введите оплату: сумма и количество занятий (например: 3000 8)"))
		return
	}

	if strings.HasPrefix(data, "schedule_") {
		studentID, _ := strconv.Atoi(strings.TrimPrefix(data, "schedule_"))
		rows, err := db.Query(context.Background(), "SELECT id, weekday, time FROM schedules WHERE student_id = $1 ORDER BY weekday, time", studentID)
		if err == nil {
			var count int
			for rows.Next() {
				var id int
				var weekday int
				var timeStr string
				rows.Scan(&id, &weekday, &timeStr)
				txt := fmt.Sprintf("%s %s", dayName(weekday), timeStr)
				btn := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("delete_lesson_%d", id)),
					),
				)
				m := tgbotapi.NewMessage(chatID, txt)
				m.ReplyMarkup = btn
				bot.Send(m)
				count++
			}
			if count == 0 {
				bot.Send(tgbotapi.NewMessage(chatID, "У этого ученика пока нет расписания."))
			}
		}

		addBtn := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("\u2795 Добавить занятие", fmt.Sprintf("schedule_add_%d", studentID)),
			),
		)
		m := tgbotapi.NewMessage(chatID, "Что сделать дальше?")
		m.ReplyMarkup = addBtn
		bot.Send(m)
		return
	}

	if strings.HasPrefix(data, "schedule_add_") {
		studentID, _ := strconv.Atoi(strings.TrimPrefix(data, "schedule_add_"))
		pendingScheduleStudent[chatID] = studentID
		days := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Пн", "day_1"),
			tgbotapi.NewInlineKeyboardButtonData("Вт", "day_2"),
			tgbotapi.NewInlineKeyboardButtonData("Ср", "day_3"),
			tgbotapi.NewInlineKeyboardButtonData("Чт", "day_4"),
			tgbotapi.NewInlineKeyboardButtonData("Пт", "day_5"),
			tgbotapi.NewInlineKeyboardButtonData("Сб", "day_6"),
			tgbotapi.NewInlineKeyboardButtonData("Вс", "day_0"),
		}
		rows := [][]tgbotapi.InlineKeyboardButton{}
		for i := 0; i < len(days); i += 4 {
			end := i + 4
			if end > len(days) {
				end = len(days)
			}
			rows = append(rows, days[i:end])
		}
		markup := tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg := tgbotapi.NewMessage(chatID, "Выберите день недели:")
		msg.ReplyMarkup = markup
		bot.Send(msg)
		return
	}

	if strings.HasPrefix(data, "day_") {
		day, _ := strconv.Atoi(strings.TrimPrefix(data, "day_"))
		pendingScheduleDay[chatID] = day
		timeButtons := []tgbotapi.InlineKeyboardButton{}
		for h := 8; h <= 20; h++ {
			label := fmt.Sprintf("%02d:00", h)
			timeButtons = append(timeButtons, tgbotapi.NewInlineKeyboardButtonData(label, "time_"+label))
		}
		rows := [][]tgbotapi.InlineKeyboardButton{}
		for i := 0; i < len(timeButtons); i += 4 {
			end := i + 4
			if end > len(timeButtons) {
				end = len(timeButtons)
			}
			rows = append(rows, timeButtons[i:end])
		}
		markup := tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg := tgbotapi.NewMessage(chatID, "Выберите время занятия:")
		msg.ReplyMarkup = markup
		bot.Send(msg)
		return
	}

	if strings.HasPrefix(data, "time_") {
		timeStr := strings.TrimPrefix(data, "time_")
		studentID := pendingScheduleStudent[chatID]
		day := pendingScheduleDay[chatID]
		delete(pendingScheduleStudent, chatID)
		delete(pendingScheduleDay, chatID)
		_, err := db.Exec(context.Background(), "INSERT INTO schedules (student_id, weekday, time) VALUES ($1, $2, $3)", studentID, day, timeStr)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Занятие добавлено: %s в %s", dayName(day), timeStr)))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении расписания."))
		}
		return
	}

	if strings.HasPrefix(data, "delete_lesson_") {
		lessonID, _ := strconv.Atoi(strings.TrimPrefix(data, "delete_lesson_"))
		_, err := db.Exec(context.Background(), "DELETE FROM schedules WHERE id = $1", lessonID)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "Занятие удалено."))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при удалении."))
		}
		return
	}
}

func dayName(n int) string {
	days := map[int]string{
		0: "Воскресенье", 1: "Понедельник", 2: "Вторник", 3: "Среда",
		4: "Четверг", 5: "Пятница", 6: "Суббота",
	}
	return days[n]
}
