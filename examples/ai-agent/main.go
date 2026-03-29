package main

// Example: AI Agent integration with WeChat
// This example demonstrates how to integrate an AI assistant with WeChat using the SDK.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/SpellingDragon/wechat-robot-go/wechat"
)

// AIAgent represents an AI service integration.
// In production, you would use services like OpenAI, Claude, or local LLMs.
type AIAgent struct {
	// Add your AI client here (OpenAI, Anthropic, etc.)
}

// GenerateResponse generates a response using the AI service.
// Replace this with your actual AI integration.
func (a *AIAgent) GenerateResponse(ctx context.Context, userMessage string) (string, error) {
	// Placeholder implementation - replace with actual AI call
	// Example integration points:
	// - OpenAI: client.CreateChatCompletion()
	// - Claude: client.Messages.Create()
	// - Local LLM: ollama.Generate()

	return fmt.Sprintf("AI Echo: %s", userMessage), nil
}

func main() {
	// Create bot with debug logging
	bot := wechat.NewBot(
		wechat.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))),
	)

	// Initialize AI agent
	aiAgent := &AIAgent{}

	// Setup context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Login
	fmt.Println("Logging in...")
	err := bot.Login(ctx, func(qrCode string) {
		fmt.Println("Please scan the QR code with WeChat:")
		fmt.Println(qrCode)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Login successful! AI Agent is now online.")

	// Register message handler with AI integration
	bot.OnMessage(func(ctx context.Context, msg *wechat.Message) error {
		// Handle text messages
		text := msg.Text()
		if text == "" {
			return nil
		}

		// Show typing indicator
		_ = bot.SendTyping(ctx, msg.FromUserID)

		// Generate AI response
		response, err := aiAgent.GenerateResponse(ctx, text)
		if err != nil {
			slog.Error("AI generation failed", "error", err)
			// Send error message
			return bot.Reply(ctx, msg, "Sorry, I encountered an error. Please try again.")
		}

		// Stop typing and send response
		_ = bot.StopTyping(ctx, msg.FromUserID)

		// Use SendLongText for potentially long AI responses
		token, _ := bot.GetContextToken(msg.FromUserID)
		if token != "" {
			_, err = wechat.SendLongText(ctx, bot.Client(), bot.Media(), msg.FromUserID, response, token)
			if err != nil {
				return bot.Reply(ctx, msg, response)
			}
			return nil
		}

		return bot.Reply(ctx, msg, response)
	})

	// Run the bot
	fmt.Println("AI Agent is running. Press Ctrl+C to stop.")
	if err := bot.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Bot stopped with error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Bot stopped gracefully.")
}
