// pi-ai is a CLI tool for managing OAuth credentials for AI providers.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"pi-ai-go/utils/oauth"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch os.Args[1] {
	case "login":
		handleLogin(ctx)
	case "list":
		handleList()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: pi-ai <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  login [provider]  Login to an OAuth provider")
	fmt.Println("  list              List available OAuth providers")
}

func handleList() {
	providers := oauth.List()
	if len(providers) == 0 {
		fmt.Println("No OAuth providers registered.")
		return
	}

	fmt.Println("Available OAuth providers:")
	for _, id := range providers {
		p, err := oauth.Get(id)
		if err != nil {
			continue
		}
		fmt.Printf("  %-20s %s\n", id, p.Name)
	}
}

func handleLogin(ctx context.Context) {
	providerID := ""
	if len(os.Args) > 2 {
		providerID = os.Args[2]
	}

	if providerID == "" {
		providers := oauth.List()
		if len(providers) == 0 {
			fmt.Fprintln(os.Stderr, "No OAuth providers available.")
			os.Exit(1)
		}

		fmt.Println("Available providers:")
		for i, id := range providers {
			p, _ := oauth.Get(id)
			fmt.Printf("  %d) %s (%s)\n", i+1, p.Name, id)
		}
		fmt.Print("\nSelect provider (number): ")

		var choice int
		fmt.Scanln(&choice)
		if choice < 1 || choice > len(providers) {
			fmt.Fprintln(os.Stderr, "Invalid choice.")
			os.Exit(1)
		}
		providerID = providers[choice-1]
	}

	provider, err := oauth.Get(providerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Logging in to %s...\n", provider.Name)

	creds, err := provider.Login(ctx, oauth.LoginCallbacks{
		OnAuth: func(url string) {
			fmt.Printf("\nOpen this URL in your browser:\n\n  %s\n\n", url)
		},
		OnDeviceCode: func(code, uri string) {
			fmt.Printf("\nVisit: %s\nEnter code: %s\n\n", uri, code)
		},
		OnPrompt: func(msg string) {
			fmt.Println(msg)
		},
		OnProgress: func(msg string) {
			fmt.Print(".")
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nLogin failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nLogin successful!\n")

	// Save credentials
	if err := saveCredentials(providerID, creds); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save credentials: %v\n", err)
	} else {
		fmt.Println("Credentials saved.")
	}
}

func saveCredentials(providerID string, creds oauth.Credentials) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := home + "/.pi-ai"
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	type savedCredentials struct {
		Provider string `json:"provider"`
		oauth.Credentials
	}

	data, err := json.MarshalIndent(savedCredentials{
		Provider:    providerID,
		Credentials: creds,
	}, "", "  ")
	if err != nil {
		return err
	}

	path := dir + "/auth.json"
	return os.WriteFile(path, data, 0600)
}

