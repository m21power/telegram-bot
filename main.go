package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
)

var bot *tgbotapi.BotAPI
var db *sql.DB

func connectDB() (*sql.DB, error) {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbHost, dbPort, dbName)
	return sql.Open("mysql", dsn)
}

func initDB() {
	var err error
	db, err = connectDB()
	if err != nil {
		log.Fatalln("Error connecting to the database:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalln("Error pinging the database:", err)
	}

	// Create `users` table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		referrer BIGINT,
		referred BOOLEAN DEFAULT false,
		username VARCHAR(255),
		referral_link VARCHAR(255),
		referral_count INT DEFAULT 0
	);`)
	if err != nil {
		log.Fatalln("Error creating users table:", err)
	}

	// Create `UserID` table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS user_checks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id int,
    referrerID int
);`)
	if err != nil {
		log.Fatalln("Error creating user_checks table:", err)
	}

	log.Println("Database initialized successfully.")
}

func generateReferralLink(userID int) string {
	return fmt.Sprintf("https://t.me/Meme_receiver_bot?start=%d", userID)
}

func checkIfUserJoinedChannel(userID int) bool {
	channelUsername := "@CNCSMEMES"
	memberStatus, err := bot.GetChatMember(tgbotapi.ChatConfigWithUser{
		SuperGroupUsername: channelUsername,
		UserID:             userID,
	})
	if err != nil {
		log.Println("Error checking if user joined the channel:", err)
		return false
	}
	return memberStatus.Status == "member" || memberStatus.Status == "creator"
}

func handleStart(update tgbotapi.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.UserName
	referralID := strings.TrimSpace(update.Message.CommandArguments())

	var referrerID int
	if referralID != "" {
		var err error
		referrerID, err = strconv.Atoi(referralID)
		if err != nil {
			log.Printf("Invalid referral ID: %s. Error: %v\n", referralID, err)
			referrerID = 0
		}
	}
	// if referreID == 0 but userID not joined the channel
	//if referredId == 0 but user id joined the channel

	//if referredId != 0 but user id joined the channel

	// if referreID != 0 but userID not joined the channel
	if referrerID != 0 && checkIfUserJoinedChannel(userID) {
		return
	}
	// Check if user joined the channel
	if !checkIfUserJoinedChannel(userID) {
		if referrerID != 0 {
			query := "INSERT INTO user_checks(user_id, referrerID) VALUES(?, ?)"
			_, err := db.Exec(query, userID, referrerID)
			if err != nil {
				log.Println("Error inserting user ID:", err)
			}
		}
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Please join our channel @CNCSMEMES before using the bot. After joining, send any message here."))
		return
	}

	// Check if user already exists in the `users` table
	var userExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&userExists)
	if err != nil {
		log.Printf("Error checking if user exists in DB: %v\n", err)
		return
	}

	if !userExists {
		var referrerID int
		query := "SELECT referrerID FROM user_checks WHERE user_id = ?"
		row := db.QueryRow(query, userID)

		err := row.Scan(&referrerID)
		if err != nil {
			if err == sql.ErrNoRows {
				log.Println("No referrerID found for this user_id.")
				referrerID = 0 // Default value, or handle as per your logic
			} else {
				log.Printf("Error querying referrerID: %v\n", err)
				// Handle unexpected errors, e.g., database issues
			}
		}
		referralLink := generateReferralLink(userID)
		if referrerID > 0 {
			_, err := db.Exec("INSERT INTO users (id, username, referred, referral_link, referrer) VALUES (?, ?, ?, ?, ?)", userID, username, true, referralLink, referrerID)
			if err != nil {
				log.Printf("Error inserting user with referrer: %v\n", err)
				return
			}
			_, err = db.Exec("UPDATE users SET referral_count = referral_count + 1 WHERE id = ?", referrerID)
			if err != nil {
				log.Printf("Error updating referral count: %v\n", err)
			}
		} else {
			_, err := db.Exec("INSERT INTO users (id, username, referred, referral_link) VALUES (?, ?, ?, ?)", userID, username, false, referralLink)
			if err != nil {
				log.Printf("Error inserting user without referrer: %v\n", err)
			}
		}
	}

	referralLink := generateReferralLink(userID)
	message := "🎉 Welcome to the CNCS(4K) MEMES! 🚀 Ready to unleash some epic laughs? Share this link with your friends, enemies who always laughs too loud: \n\n" + referralLink + "\n\n😂 Let's see who can bring in the most recruits! More memes, more fun!"
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, message))
}

func handleMyReferrals(update tgbotapi.Update) {
	userID := update.Message.From.ID
	var referralCount int

	err := db.QueryRow("SELECT referral_count FROM users WHERE id = ?", userID).Scan(&referralCount)
	if err != nil {
		log.Printf("Error retrieving referral count: %v\n", err)
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Error retrieving referral count."))
		return
	}

	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("You have referred %d people.", referralCount)))
}

func handleStats(update tgbotapi.Update) {
	userID := update.Message.From.ID

	// Retrieve the user's stats
	var referralCount int
	var username string
	err := db.QueryRow("SELECT referral_count, username FROM users WHERE id = ?", userID).Scan(&referralCount, &username)
	if err != nil {
		log.Println("Error retrieving user stats:", err)
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred while retrieving your stats."))
		return
	}

	// Retrieve top 10 referrers
	rows, err := db.Query("SELECT id, username, referral_count FROM users ORDER BY referral_count DESC LIMIT 10")
	if err != nil {
		log.Println("Error retrieving top referrers:", err)
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred while retrieving top referrers."))
		return
	}
	defer rows.Close()

	// Format the leaderboard
	var message strings.Builder
	message.WriteString(fmt.Sprintf("%-5s %-20s %-10s\n", "Rank", "Username", "Referrals"))
	for i := 1; rows.Next(); i++ {
		var id int
		var name string
		var count int
		err := rows.Scan(&id, &name, &count)
		if err != nil {
			log.Println("Error scanning leaderboard row:", err)
			continue
		}
		message.WriteString(fmt.Sprintf("%-5d %-20s %-10d\n", i, name, count))
	}

	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, message.String()))
}
func main() {
	var err error
	err = godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatalln("TELEGRAM_BOT_TOKEN is not set.")
	}

	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalln("Error starting bot:", err)
	}
	initDB()
	_, err = bot.RemoveWebhook()
	if err != nil {
		log.Fatalf("Error removing webhook: %v\n", err)
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalln("Error getting updates:", err)
	}

	for update := range updates {
		if update.Message != nil && update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				handleStart(update)
			case "myreferrals":
				handleMyReferrals(update)
			case "stats":
				handleStats(update)
			}
		}
	}
}
