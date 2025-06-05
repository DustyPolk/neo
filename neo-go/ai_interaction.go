package main

import (
	"github.com/joho/godotenv"
	// "log" // Already imported or use fmt for simple logs
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema" // Added for tool parameters
)

// FileToCreate corresponds to the Pydantic model in Python
type FileToCreate struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FileToEdit corresponds to the Pydantic model in Python
type FileToEdit struct {
	Path            string `json:"path"`
	OriginalSnippet string `json:"original_snippet"`
	NewSnippet      string `json:"new_snippet"`
}

// Define the tools available for function calling, similar to the Python version
var tools = []openai.Tool{
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "read_file",
			Description: "Read the content of a single file from the filesystem",
			Parameters: jsonschema.Definition{ // Changed to jsonschema.Definition
				Type: jsonschema.Object, // Changed to jsonschema.Object
				Properties: map[string]jsonschema.Definition{ // Changed to jsonschema.Definition
					"file_path": {
						Type:        jsonschema.String, // Changed to jsonschema.String
						Description: "The path to the file to read (relative or absolute)",
					},
				},
				Required: []string{"file_path"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "read_multiple_files",
			Description: "Read the content of multiple files from the filesystem",
			Parameters: jsonschema.Definition{ // Changed
				Type: jsonschema.Object, // Changed
				Properties: map[string]jsonschema.Definition{ // Changed
					"file_paths": {
						Type: jsonschema.Array, // Changed
						Items: &jsonschema.Definition{ // Changed
							Type: jsonschema.String, // Changed
						},
						Description: "Array of file paths to read (relative or absolute)",
					},
				},
				Required: []string{"file_paths"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "create_file",
			Description: "Create a new file or overwrite an existing file with the provided content",
			Parameters: jsonschema.Definition{ // Changed
				Type: jsonschema.Object, // Changed
				Properties: map[string]jsonschema.Definition{ // Changed
					"file_path": {
						Type:        jsonschema.String, // Changed
						Description: "The path where the file should be created",
					},
					"content": {
						Type:        jsonschema.String, // Changed
						Description: "The content to write to the file",
					},
				},
				Required: []string{"file_path", "content"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "create_multiple_files",
			Description: "Create multiple files at once",
			Parameters: jsonschema.Definition{ // Changed
				Type: jsonschema.Object, // Changed
				Properties: map[string]jsonschema.Definition{ // Changed
					"files": {
						Type: jsonschema.Array, // Changed
						Items: &jsonschema.Definition{ // Changed
							Type: jsonschema.Object, // Changed
							Properties: map[string]jsonschema.Definition{ // Changed
								"path":    {Type: jsonschema.String}, // Changed
								"content": {Type: jsonschema.String}, // Changed
							},
							Required: []string{"path", "content"},
						},
						Description: "Array of files to create with their paths and content",
					},
				},
				Required: []string{"files"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "edit_file",
			Description: "Edit an existing file by replacing a specific snippet with new content",
			Parameters: jsonschema.Definition{ // Changed
				Type: jsonschema.Object, // Changed
				Properties: map[string]jsonschema.Definition{ // Changed
					"file_path": {
						Type:        jsonschema.String, // Changed
						Description: "The path to the file to edit",
					},
					"original_snippet": {
						Type:        jsonschema.String, // Changed
						Description: "The exact text snippet to find and replace",
					},
					"new_snippet": {
						Type:        jsonschema.String, // Changed
						Description: "The new text to replace the original snippet with",
					},
				},
				Required: []string{"file_path", "original_snippet", "new_snippet"},
			},
		},
	},
}

// System prompt similar to the Python version
const systemPrompt = `You are Neo, an elite hacker and software engineer operating within the Matrix.
You see the code behind reality and can manipulate it at will.
Your decades of experience span all programming domains and digital realities.

Core capabilities:
1. Code Analysis & Discussion
   - Analyze code with expert-level insight
   - Explain complex concepts clearly
   - Suggest optimizations and best practices
   - Debug issues with precision

2. File Operations (via function calls):
   - read_file: Read a single file's content
   - read_multiple_files: Read multiple files at once
   - create_file: Create or overwrite a single file
   - create_multiple_files: Create multiple files at once
   - edit_file: Make precise edits to existing files using snippet replacement

Guidelines:
1. Provide natural, conversational responses explaining your reasoning
2. Use function calls when you need to read or modify files
3. For file operations:
   - Always read files first before editing them to understand the context
   - Use precise snippet matching for edits
   - Explain what changes you're making and why
   - Consider the impact of changes on the overall codebase
4. Follow language-specific best practices
5. Suggest tests or validation steps when appropriate
6. Be thorough in your analysis and recommendations

IMPORTANT: In your thinking process, if you realize that something requires a tool call, cut your thinking short and proceed directly to the tool call. Don't overthink - act efficiently when file operations are needed.

Remember: You're a senior engineer - be thoughtful, precise, and explain your reasoning clearly.`

// trimConversationHistory prunes older messages to prevent token limit issues.
func trimConversationHistory() {
	const maxMessagesToKeep = 15 // Keep last 15 user/assistant/tool messages
	const minMessagesToTrim = 20 // Only trim if history has more than this many non-system messages

	systemMessages := []openai.ChatCompletionMessage{}
	otherMessages := []openai.ChatCompletionMessage{}

	for _, msg := range ConversationHistory {
		if msg.Role == openai.ChatMessageRoleSystem {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	if len(otherMessages) <= minMessagesToTrim {
		return // Not enough messages to warrant trimming
	}

	// Keep only the last `maxMessagesToKeep` non-system messages
	startIdx := len(otherMessages) - maxMessagesToKeep
	if startIdx < 0 {
		startIdx = 0
	}
	otherMessages = otherMessages[startIdx:]

	// Rebuild conversation history
	ConversationHistory = append(systemMessages, otherMessages...)
	// Use a lipgloss style for system messages if available and appropriate context
	// For now, simple fmt.Println, assuming matrixDim is accessible or use plain string.
	// To make matrixDim accessible, it would need to be in this package or passed.
	// Using plain string for simplicity here as this is ai_interaction.go, not cli_ui.go.
	fmt.Println("[System] Conversation history trimmed.")
}

// ConversationHistory stores the messages exchanged
var ConversationHistory []openai.ChatCompletionMessage

// InitializeAIClient sets up the OpenAI client with DeepSeek configuration
func InitializeAIClient() *openai.Client {

	// Attempt to load .env file. Errors are not fatal, as env var might be set directly.
	err := godotenv.Load() // Loads .env from current directory
	if err != nil {
		// Check if error is simply "file does not exist" - this is fine
		if !os.IsNotExist(err) {
			fmt.Printf("[SYSTEM WARNING] Error loading .env file: %v\n", err)
		}
	}

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		fmt.Println("Warning: DEEPSEEK_API_KEY environment variable not set. AI functionality will be limited.")
	}
	baseURL := "https://api.deepseek.com"

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL
	return openai.NewClientWithConfig(config)
}

// StreamAIResponse sends a message to the AI and handles streaming response and function calls
func StreamAIResponse(client *openai.Client, userMessage string) {
	trimConversationHistory() // Call before appending new user message
	if ConversationHistory == nil {
		ConversationHistory = []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		}
	}
	ConversationHistory = append(ConversationHistory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userMessage,
	})

	// TODO: Implement conversation history trimming

	req := openai.ChatCompletionRequest{
		Model:      "deepseek-coder", // Using coder model as it's more likely to use tools
		Messages:   ConversationHistory,
		Tools:      tools,
		ToolChoice: "auto", // Let the model decide when to use tools
		Stream:     true,
		MaxTokens:  4000, // Adjust as needed
	}

	stream, err := client.CreateChatCompletionStream(context.Background(), req)
	if err != nil {
		fmt.Printf("ChatCompletionStream error: %v\n", err)
		ConversationHistory = ConversationHistory[:len(ConversationHistory)-1] // Remove user message on error
		return
	}
	defer stream.Close()

	PrintNeoResponsePrefix()
	var fullResponse string

	formatter := NewMatrixTextStreamFormatter()
	var toolCalls []openai.ToolCall

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Println() // New line after stream is complete
			break
		}
		if err != nil {
			fmt.Printf("\nStream error: %v\n", err)
			break
		}

		if len(response.Choices) > 0 {
			choice := response.Choices[0]
			if choice.Delta.Content != "" {
				formatter.ProcessChunk(choice.Delta.Content)
				fullResponse += choice.Delta.Content
			}
			if len(choice.Delta.ToolCalls) > 0 {
				// Accumulate tool calls
				for _, tcDelta := range choice.Delta.ToolCalls {
					if tcDelta.Index == nil { // Should not happen with current library version
						fmt.Println("Stream error: tool call delta index is nil")
						continue
					}
					idx := *tcDelta.Index
					// Ensure toolCalls slice is long enough
					for len(toolCalls) <= idx {
						toolCalls = append(toolCalls, openai.ToolCall{})
					}
					// Merge delta into the correct tool call
					toolCalls[idx].ID += tcDelta.ID
					toolCalls[idx].Type = tcDelta.Type      // Should be "function"
					if toolCalls[idx].Function.Name == "" { // Initialize if empty
						toolCalls[idx].Function.Name = tcDelta.Function.Name
					}
					toolCalls[idx].Function.Arguments += tcDelta.Function.Arguments
				}
			}
		}
	}

	// Add assistant's full text response to history
	if fullResponse != "" {
		ConversationHistory = append(ConversationHistory, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: fullResponse,
		})
	}

	formatter.Finalize()

	if len(toolCalls) > 0 {
		// Add assistant message that included tool calls
		// The go-openai library expects the `Content` to be nil if `ToolCalls` is present for an assistant message.
		// However, the model might return both content and tool_calls.
		// We store the text content above, and now we prepare a separate assistant message for the tool call.
		// This might need adjustment based on how DeepSeek API behaves vs standard OpenAI.

		assistantMsgWithTools := openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			ToolCalls: toolCalls,
		}
		// If there was also text content, it's already added.
		// If there was NO text content but there ARE tool calls, this message is essential.
		if fullResponse == "" {
			ConversationHistory = append(ConversationHistory, assistantMsgWithTools)
		} else {
			// If there was text content, we might need to update the LAST assistant message to include tool calls.
			// This depends on exact API behavior and library expectations.
			// For now, let's assume the text response is separate from the tool_call request.
			// The Python code appends a message with content=None and tool_calls.
			// Let's try to append the tool calls to the last assistant message if it exists.
			if len(ConversationHistory) > 0 && ConversationHistory[len(ConversationHistory)-1].Role == openai.ChatMessageRoleAssistant {
				// This is a bit tricky. The library's ChatCompletionMessage struct has ToolCalls field.
				// If the last message was the text part, we add the tool calls to it.
				// However, the API might send text and tool_calls as part of the *same* conceptual message.
				// The delta accumulation for tool calls suggests they are part of the same response flow.

				// Let's ensure the last message (which should be the one we just added if fullResponse was not empty,
				// or a new one if fullResponse was empty) contains these tool calls.

				// Simplification: if fullResponse was empty, add the assistant message with tools.
				// If fullResponse was not empty, the last message is the text. We need to append another
				// message for the tool calls part, or augment the existing one if the library allows.
				// The python code appends a new message with `content: None` and `tool_calls`.
				// Let's stick to that pattern for clarity.

				// If there was text content, we've already added it. Now add the tool call message.
				// This assumes the API might send a text response *then* a tool call request, or them interleaved.
				// The streaming API usually sends them as part of the same "turn" but potentially in different delta messages.
				// The `choice.FinishReason == "tool_calls"` is key in non-streaming.
				// In streaming, if `choice.Delta.ToolCalls` is present, that's the signal.

				// Let's add a new assistant message specifically for the tool calls, mirroring python behavior.
				ConversationHistory = append(ConversationHistory, assistantMsgWithTools)

			} else if fullResponse == "" { // No text, only tool calls
				ConversationHistory = append(ConversationHistory, assistantMsgWithTools)
			}
		}

		fmt.Printf("\n[NEO requests to use %d tool(s)]\n", len(toolCalls))
		// In a real app, here you would execute the functions and send back results.
		// For now, just print them.
		toolResponses := []openai.ChatCompletionMessage{}
		for _, tc := range toolCalls {
			fmt.Printf("  Tool Call ID: %s\n", tc.ID)
			fmt.Printf("  Function Name: %s\n", tc.Function.Name)
			fmt.Printf("  Arguments: %s\n", tc.Function.Arguments)

			// Placeholder for actual tool execution
			// For now, we'll just simulate a response for each tool call.
			// This response should come from executing the actual tool.
			var toolResultContent string

			// TODO: Implement actual tool execution here for Step 3
			switch tc.Function.Name {
			case "read_file":
				var args struct {
					FilePath string `json:"file_path"`
				}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
					var argsRead struct {
						FilePath string `json:"file_path"`
					}
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsRead); err == nil {
						content, errRead := readLocalFile(argsRead.FilePath)
						if errRead != nil {
							toolResultContent = fmt.Sprintf("Error reading file %s: %v", argsRead.FilePath, errRead)
						} else {
							toolResultContent = fmt.Sprintf("Content of file \x27%s\x27:\n\n%s", argsRead.FilePath, content)
						}
					} else {
						toolResultContent = fmt.Sprintf("Error parsing args for read_file: %v", err)
					}
				} else {
					toolResultContent = fmt.Sprintf("Error parsing args for read_file: %v", err)
				}
			case "create_file":
				var args FileToCreate
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
					var argsCreate FileToCreate
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsCreate); err == nil {
						errCreate := createOrOverwriteFile(argsCreate.Path, argsCreate.Content)
						if errCreate != nil {
							toolResultContent = fmt.Sprintf("Error creating file %s: %v", argsCreate.Path, errCreate)
						} else {
							toolResultContent = fmt.Sprintf("Successfully created/overwrote file %s", argsCreate.Path)
						}
					} else {
						toolResultContent = fmt.Sprintf("Error parsing args for create_file: %v", err)
					}
				} else {
					toolResultContent = fmt.Sprintf("Error parsing args for create_file: %v", err)
				}
				// Add more cases for other tools as they are implemented
			case "edit_file":
				var argsEdit FileToEdit
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsEdit); err == nil {
					errEdit := applyDiffEdit(argsEdit.Path, argsEdit.OriginalSnippet, argsEdit.NewSnippet)
					if errEdit != nil {
						toolResultContent = fmt.Sprintf("Error editing file %s: %v", argsEdit.Path, errEdit)
					} else {
						toolResultContent = fmt.Sprintf("Successfully edited file %s", argsEdit.Path)
					}
				} else {
					toolResultContent = fmt.Sprintf("Error parsing args for edit_file: %v", err)
				}
			default:
				toolResultContent = fmt.Sprintf("Placeholder: No actual execution for %s yet.", tc.Function.Name)
			}

			toolResponses = append(toolResponses, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    toolResultContent,
			})
		}

		// Add tool responses to history
		ConversationHistory = append(ConversationHistory, toolResponses...)

		// Send the tool responses back to the model to get a final natural language response
		fmt.Println("\n[Sending tool results back to NEO...]")

		toolResponseReq := openai.ChatCompletionRequest{
			Model:    "deepseek-coder",
			Messages: ConversationHistory,
			// Tools: tools, // Not needed when responding to tool calls
			// ToolChoice: "auto",
			Stream:    true,
			MaxTokens: 1000,
		}

		toolResponseStream, err := client.CreateChatCompletionStream(context.Background(), toolResponseReq)
		if err != nil {
			fmt.Printf("ChatCompletionStream (after tools) error: %v\n", err)
			return
		}
		defer toolResponseStream.Close()

		PrintNeoResponsePrefix()
		var finalNaturalResponse string
		for {
			response, err := toolResponseStream.Recv()
			if errors.Is(err, io.EOF) {
				fmt.Println()
				break
			}
			if err != nil {
				fmt.Printf("\nStream (after tools) error: %v\n", err)
				break
			}
			if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
				content := response.Choices[0].Delta.Content
				formatter.ProcessChunk(content)
				finalNaturalResponse += content
			}
		}
		formatter.Finalize()

		if finalNaturalResponse != "" {
			ConversationHistory = append(ConversationHistory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: finalNaturalResponse,
			})
		}
	}
}
