package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	prompt "github.com/c-bata/go-prompt"
	openai "github.com/sashabaranov/go-openai"
)

var p *prompt.Prompt
var AIPromptClient *openai.Client

func executor(in string) {
	userInput := strings.TrimSpace(in)
	if userInput == "" {
		return
	}

	lowerUserInput := strings.ToLower(userInput)

	if lowerUserInput == "exit" || lowerUserInput == "quit" || lowerUserInput == "/exit" || lowerUserInput == "/quit" {
		fmt.Println(matrixDim.Render("Disconnecting from the Matrix..."))
		DisplayMatrixExit() // Call our new exit animation
		os.Exit(0)
	} else if lowerUserInput == "/clear" {
		ClearScreen()
		if len(ConversationHistory) > 0 && ConversationHistory[0].Role == openai.ChatMessageRoleSystem {
			originalSystemPrompt := ConversationHistory[0]
			ConversationHistory = []openai.ChatCompletionMessage{originalSystemPrompt}
		} else {
			ConversationHistory = []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: systemPrompt}}
		}
		fmt.Println(matrixPrimary.Render("Memory wiped. You are free."))
		fmt.Println(matrixDim.Render("(System prompt preserved)"))
		return
	} else if lowerUserInput == "/red_pill" {
		fmt.Println(matrixError.Render("> You take the red pill..."))
		time.Sleep(1 * time.Second)
		fmt.Println(matrixPrimary.Render("> Welcome to the desert of the real."))
		return
	} else if lowerUserInput == "/blue_pill" {
		fmt.Println(matrixAccent.Render("> You take the blue pill..."))
		time.Sleep(1 * time.Second)
		fmt.Println(matrixDim.Render("> Wake up. Believe whatever you want to believe."))
		return
	} else if strings.HasPrefix(lowerUserInput, "/add ") {
		pathToAdd := strings.TrimSpace(strings.TrimPrefix(userInput, "/add ")) // Use original userInput for case-sensitive path
		if pathToAdd == "" {
			fmt.Println(matrixError.Render("Usage: /add <file_or_directory_path>"))
			return
		}
		fmt.Println(matrixDim.Render(fmt.Sprintf("Scanning %s...", pathToAdd)))
		addedContents, skippedPaths, err := addDirectoryToConversationHelper(pathToAdd)
		if err != nil {
			fmt.Println(matrixError.Render(fmt.Sprintf("Error processing %s: %v", pathToAdd, err)))
			return
		}
		fmt.Println()

		if len(addedContents) > 0 {
			fmt.Println(matrixAccent.Render("--- Files Added to Context ---"))
			for relPath, content := range addedContents {
				fullPath := filepath.Join(pathToAdd, relPath)
				contextMsg := fmt.Sprintf("Content of file '%s':\n\n%s", fullPath, content)
				ConversationHistory = append(ConversationHistory, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: contextMsg})
				fmt.Println(matrixPrimary.Render(fmt.Sprintf("  ✓ %s", fullPath)))
			}
		}
		fmt.Println()

		if len(skippedPaths) > 0 {
			fmt.Println(matrixDim.Render("--- Files Skipped ---"))
			for i, sPath := range skippedPaths {
				if i < 10 {
					fmt.Println(matrixDim.Render(fmt.Sprintf("  ✗ %s", sPath)))
				}
			}
			if len(skippedPaths) > 10 {
				fmt.Println(matrixDim.Render(fmt.Sprintf("  ...and %d more.", len(skippedPaths)-10)))
			}
		}
		fmt.Println(matrixAccent.Render("--- End of /add operation ---"))
		return
	} else {
		// Default case: send to AI
		StreamAIResponse(AIPromptClient, userInput)
	}
}

func completer(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "/exit", Description: "Exit Neo"},
		{Text: "/quit", Description: "Exit Neo"},
		{Text: "/clear", Description: "Clear conversation history"},
		{Text: "/add ", Description: "Add file/directory to context (/add path/to/file)"},
		{Text: "/red_pill", Description: "See the truth"},
		{Text: "/blue_pill", Description: "Remain in blissful ignorance"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func main() {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		fmt.Println(matrixError.Render("Error: DEEPSEEK_API_KEY environment variable not set."))
		fmt.Println("Please set it before running the application.")
		return
	}

	AIPromptClient = InitializeAIClient()
	DisplayInitialScreen()

	if len(ConversationHistory) == 0 {
		ConversationHistory = []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		}
	}

	p = prompt.New(
		executor,
		completer,
		prompt.OptionPrefix(PromptPrefix),
		prompt.OptionTitle("neo-ai"),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
	)
	fmt.Println(matrixDim.Render("Type '/exit' or 'quit' to disconnect from the Matrix."))
	p.Run()
}
