package main

import (
	"fmt"
	"log"
	"github.com/joho/godotenv"
	"os"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env:", err)
	}
	
	fmt.Println("AI_ENABLED:", os.Getenv("AI_ENABLED"))
	fmt.Println("AI_MODEL:", os.Getenv("AI_MODEL"))
	fmt.Println("AI_BASE_URL:", os.Getenv("AI_BASE_URL"))
	fmt.Println("AI_API_KEY:", os.Getenv("AI_API_KEY")[:10] + "...")
	fmt.Println("AI_TIMEOUT_SECONDS:", os.Getenv("AI_TIMEOUT_SECONDS"))
	fmt.Println("AI_QUEST_TIMEOUT_SECONDS:", os.Getenv("AI_QUEST_TIMEOUT_SECONDS"))
	fmt.Println("AI_QUEST_MAX_TOKENS:", os.Getenv("AI_QUEST_MAX_TOKENS"))
}
