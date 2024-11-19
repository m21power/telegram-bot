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
)

func connectDB() (*sql.DB, error) {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbHost, dbPort, dbName)
	return sql.Open("mysql", dsn)
}

// Global variables
var bot *tgbotapi.BotAPI
var db *sql.DB

// Initialize the database connection
func initDB() {
	var err error
	// Open MySQL database
	// Use environment variables for security
	db, err := connectDB()
	if err != nil {
		log.Fatalln("Error connecting to the database:", err)
	}

	// Check if the database is reachable
	err = db.Ping()
	if err != nil {
		log.Fatalln("Error pinging the database:", err)
	}

	// Ensure the users table exists
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
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS UserID (
		user_id BIGINT,
		referrerID BIGINT,
		UNIQUE(referrerID)
	);`)
	if err != nil {
		log.Fatalln("Error creating UserID table:", err)
	}
	log.Println("Database initialized successfully.")
}

// Generate a referral link for each user
func generateReferralLink(userID int) string {
	return fmt.Sprintf("https://t.me/CNCSMEMERECIEVERbot?start=%d", userID)
}

// Check if the user has joined the channel
func checkIfUserJoinedChannel(userID int) bool {
	chatUsername := "@CNCSMEMES"

	memberStatus, err := bot.GetChatMember(tgbotapi.ChatConfigWithUser{
		SuperGroupUsername: chatUsername,
		UserID:             userID,
	})
	if err != nil {
		log.Println("Error checking if user joined the channel:", err)
		return false
	}
	return memberStatus.Status == "member" || memberStatus.Status == "creator"
}

// Handle the /start command
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
	if referrerID != 0 && checkIfUserJoinedChannel(userID) {
		return
	}
	//userID --> referrerID
	if !checkIfUserJoinedChannel(userID) {
		query := "INSERT INTO UserID(user_id,referrerID) VALUES(?,?)"
		_, err := db.Exec(query, userID, referrerID)
		if err != nil {
			log.Println("error inserting user id", err)
			return
		}
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
			"Please join our channel @CNCSMEMES before using the bot. After joining, send any message here."))
		return
	}
	var userExist bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM UserID WHERE user_id = ?)", userID).Scan(&userExist)
	if err != nil {
		log.Printf("Error checking if user exists in DB: %v\n", err)
		return
	}
	if userExist {
		query := "SELECT referrerID FROM UserID WHERE user_id = ?"
		err := db.QueryRow(query, userID).Scan(&referrerID)
		if err != nil {
			log.Println("error getting refferer id", err)
			return
		}
	}
	// Add user to the database if they don't exist
	var userExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&userExists)
	if err != nil {
		log.Printf("Error checking if user exists in DB: %v\n", err)
		return
	}
	if !userExists {
		referralLink := generateReferralLink(userID)
		if referrerID > 0 {
			_, err := db.Exec("INSERT INTO users (id, username, referred, referral_link, referrer) VALUES (?, ?, ?, ?, ?)",
				userID, username, true, referralLink, referrerID)
			if err != nil {
				log.Printf("Error inserting user with referrer: %v\n", err)
				return
			}
			_, err = db.Exec("UPDATE users SET referral_count = referral_count + 1 WHERE id = ?", referrerID)
			if err != nil {
				log.Printf("Error updating referral count: %v\n", err)
			}
		} else {
			_, err := db.Exec("INSERT INTO users (id, username, referred, referral_link) VALUES (?, ?, ?, ?)",
				userID, username, false, referralLink)
			if err != nil {
				log.Printf("Error inserting user without referrer: %v\n", err)
			}
		}
	}
	// Send the referral link to the user
	referralLink := generateReferralLink(userID)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome! Use this referral link to invite others: "+referralLink))
}

// Handle /myreferrals command
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

// Handle the /stats command to show user statistics
func handleStats(update tgbotapi.Update) {
	userID := update.Message.From.ID

	// Retrieve the referral count and username from the database
	var referralCount int
	var username string
	err := db.QueryRow("SELECT referral_count, username FROM users WHERE id = ?", userID).Scan(&referralCount, &username)
	if err != nil {
		log.Println("Error retrieving user stats:", err)
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred while retrieving your stats."))
		return
	}

	// Query the top 10 referrers based on referral count
	rows, err := db.Query(`
		SELECT id, username, referral_count
		FROM users
		ORDER BY referral_count DESC
		LIMIT 10
	`)
	if err != nil {
		log.Println("Error retrieving top 10 referrers:", err)
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred while retrieving the top referrers."))
		return
	}
	defer rows.Close()

	// Prepare the message for the top 10 referrers
	var message string
	var userRank int
	isInTop10 := false

	// Table header
	message += fmt.Sprintf("%-5s %-20s %-15s\n", "Rank", "Username", "Referrals")
	message += fmt.Sprintf("%-5s %-20s %-15s\n", "-----", "--------------------", "---------------")

	// Add top 10 referrers to the table
	for i := 1; rows.Next(); i++ {
		var topUserID int
		var topUsername string
		var topReferralCount int
		if err := rows.Scan(&topUserID, &topUsername, &topReferralCount); err != nil {
			log.Println("Error scanning top referrer:", err)
			continue
		}

		// Add the referrer to the table format
		message += fmt.Sprintf("%-5d %-20s %-15d\n", i, topUsername, topReferralCount)

		// Check if the current user is in the top 10
		if userID == topUserID {
			userRank = i
			isInTop10 = true
		}
	}

	// If the user isn't in the top 10, display their rank at the bottom
	if !isInTop10 {
		message += fmt.Sprintf("\nYour rank: %d (out of top 10)", userRank)
	}

	// Send the message
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, message))
}

// Main function
func main() {
	var err error
	bot, err = tgbotapi.NewBotAPI("7518737960:AAHLzn8XRc31fJ69kMo3ZMQEKqqkmXpTDPE")
	if err != nil {
		log.Fatalln("Error starting bot:", err)
	}

	initDB()
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalln("Error getting updates:", err)
	}

	for update := range updates {

		// Handle commands
		if update.Message.IsCommand() {
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
