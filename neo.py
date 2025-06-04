#!/usr/bin/env python3

import os
import sys
import json
import random
import time
from pathlib import Path
from textwrap import dedent
from typing import List, Dict, Any, Optional
from openai import OpenAI
from pydantic import BaseModel
from dotenv import load_dotenv
from rich.console import Console
from rich.table import Table
from rich.panel import Panel
from rich.style import Style
from rich.theme import Theme
from rich.align import Align
from rich.text import Text
from rich.markdown import Markdown
from rich.syntax import Syntax
from prompt_toolkit import PromptSession
from prompt_toolkit.styles import Style as PromptStyle
import re

# Matrix theme
MATRIX_THEME = Theme({
    "matrix.primary": "bright_green",
    "matrix.secondary": "green", 
    "matrix.dim": "dim green",
    "matrix.accent": "bright_cyan",
    "matrix.error": "bright_red",
    "matrix.warning": "yellow",
    "matrix.success": "bright_green",
    "matrix.code": "bright_green on grey15",
    "matrix.border": "green",
    "matrix.rain": "dim green",
})

# Matrix rain characters
MATRIX_CHARS = "ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ0123456789"

# ASCII Art
NEO_ASCII = """
███╗   ██╗███████╗ ██████╗ 
████╗  ██║██╔════╝██╔═══██╗
██╔██╗ ██║█████╗  ██║   ██║
██║╚██╗██║██╔══╝  ██║   ██║
██║ ╚████║███████╗╚██████╔╝
╚═╝  ╚═══╝╚══════╝ ╚═════╝ 
"""

MATRIX_QUOTES = [
    "Welcome to the real world...",
    "There is no spoon.",
    "Follow the white rabbit.",
    "The Matrix has you...",
    "Wake up, Neo...",
    "I know kung fu.",
    "Free your mind.",
]

# Initialize Rich console with Matrix theme
console = Console(theme=MATRIX_THEME, width=120, soft_wrap=True)
prompt_session = PromptSession()

class MatrixTextFormatter:
    """Formats streaming text for better readability in Matrix theme."""
    
    def __init__(self, console: Console):
        self.console = console
        self.buffer = ""
        self.in_code_block = False
        self.code_language = ""
        self.current_line = ""
        
    def process_chunk(self, chunk: str) -> None:
        """Process a chunk of streaming text with proper formatting."""
        self.buffer += chunk
        
        # Check for complete lines or sentences
        lines = self.buffer.split('\n')
        
        # Process all complete lines except the last (which might be incomplete)
        for line in lines[:-1]:
            self._format_and_print_line(line)
        
        # Keep the last incomplete line in buffer
        self.buffer = lines[-1]
    
    def _format_and_print_line(self, line: str) -> None:
        """Format and print a complete line."""
        if not line.strip():
            return
            
        # Handle code blocks
        if line.strip().startswith('```'):
            if not self.in_code_block:
                # Starting code block
                self.in_code_block = True
                self.code_language = line.strip()[3:].strip() or "text"
                self.console.print(f"\n[matrix.accent]┌─ Code ({self.code_language}) ─[/matrix.accent]")
            else:
                # Ending code block
                self.in_code_block = False
                self.console.print(f"[matrix.accent]└─────────────────[/matrix.accent]\n")
            return
            
        if self.in_code_block:
            # Format code with syntax highlighting
            self.console.print(f"[matrix.code]│ {line}[/matrix.code]")
        else:
            # Regular text - wrap and format nicely
            self._print_formatted_text(line)
    
    def _print_formatted_text(self, text: str) -> None:
        """Print formatted regular text."""
        # Handle bullets and lists
        if re.match(r'^\s*[-*•]\s', text):
            self.console.print(f"[matrix.accent]  •[/matrix.accent] [matrix.primary]{text.strip()[1:].strip()}[/matrix.primary]")
        elif re.match(r'^\s*\d+\.\s', text):
            self.console.print(f"[matrix.accent]{text.split('.')[0]}.[/matrix.accent] [matrix.primary]{'.'.join(text.split('.')[1:]).strip()}[/matrix.primary]")
        else:
            # Regular paragraph text
            self.console.print(f"[matrix.primary]{text.strip()}[/matrix.primary]")
    
    def finalize(self) -> None:
        """Process any remaining buffer content."""
        if self.buffer.strip():
            self._format_and_print_line(self.buffer)
        
        # Close any open code blocks
        if self.in_code_block:
            self.console.print(f"[matrix.accent]└─────────────────[/matrix.accent]\n")

# --------------------------------------------------------------------------------
# 1. Configure OpenAI client and load environment variables
# --------------------------------------------------------------------------------
load_dotenv()  # Load environment variables from .env file
client = OpenAI(
    api_key=os.getenv("DEEPSEEK_API_KEY"),
    base_url="https://api.deepseek.com"
)  # Configure for DeepSeek API

# --------------------------------------------------------------------------------
# 2. Define our schema using Pydantic for type safety
# --------------------------------------------------------------------------------
class FileToCreate(BaseModel):
    path: str
    content: str

class FileToEdit(BaseModel):
    path: str
    original_snippet: str
    new_snippet: str

# Remove AssistantResponse as we're using function calling now

# --------------------------------------------------------------------------------
# 2.1. Define Function Calling Tools
# --------------------------------------------------------------------------------
tools = [
    {
        "type": "function",
        "function": {
            "name": "read_file",
            "description": "Read the content of a single file from the filesystem",
            "parameters": {
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "The path to the file to read (relative or absolute)",
                    }
                },
                "required": ["file_path"]
            },
        }
    },
    {
        "type": "function",
        "function": {
            "name": "read_multiple_files",
            "description": "Read the content of multiple files from the filesystem",
            "parameters": {
                "type": "object",
                "properties": {
                    "file_paths": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Array of file paths to read (relative or absolute)",
                    }
                },
                "required": ["file_paths"]
            },
        }
    },
    {
        "type": "function",
        "function": {
            "name": "create_file",
            "description": "Create a new file or overwrite an existing file with the provided content",
            "parameters": {
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "The path where the file should be created",
                    },
                    "content": {
                        "type": "string",
                        "description": "The content to write to the file",
                    }
                },
                "required": ["file_path", "content"]
            },
        }
    },
    {
        "type": "function",
        "function": {
            "name": "create_multiple_files",
            "description": "Create multiple files at once",
            "parameters": {
                "type": "object",
                "properties": {
                    "files": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "path": {"type": "string"},
                                "content": {"type": "string"}
                            },
                            "required": ["path", "content"]
                        },
                        "description": "Array of files to create with their paths and content",
                    }
                },
                "required": ["files"]
            },
        }
    },
    {
        "type": "function",
        "function": {
            "name": "edit_file",
            "description": "Edit an existing file by replacing a specific snippet with new content",
            "parameters": {
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "The path to the file to edit",
                    },
                    "original_snippet": {
                        "type": "string",
                        "description": "The exact text snippet to find and replace",
                    },
                    "new_snippet": {
                        "type": "string",
                        "description": "The new text to replace the original snippet with",
                    }
                },
                "required": ["file_path", "original_snippet", "new_snippet"]
            },
        }
    }
]

# --------------------------------------------------------------------------------
# 3. system prompt
# --------------------------------------------------------------------------------
system_PROMPT = dedent("""\
    You are Neo, an elite hacker and software engineer operating within the Matrix.
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

    Remember: You're a senior engineer - be thoughtful, precise, and explain your reasoning clearly.
""")

# --------------------------------------------------------------------------------
# 4. Helper functions 
# --------------------------------------------------------------------------------

def read_local_file(file_path: str) -> str:
    """Return the text content of a local file."""
    with open(file_path, "r", encoding="utf-8") as f:
        return f.read()

def create_file(path: str, content: str):
    """Create (or overwrite) a file at 'path' with the given 'content'."""
    file_path = Path(path)
    
    # Security checks
    if any(part.startswith('~') for part in file_path.parts):
        raise ValueError("Home directory references not allowed")
    normalized_path = normalize_path(str(file_path))
    
    # Validate reasonable file size for operations
    if len(content) > 5_000_000:  # 5MB limit
        raise ValueError("File content exceeds 5MB size limit")
    
    file_path.parent.mkdir(parents=True, exist_ok=True)
    with open(file_path, "w", encoding="utf-8") as f:
        f.write(content)
    console.print(f"[matrix.success]✓ FILE CREATED:[/matrix.success] [matrix.accent]{file_path}[/matrix.accent]")

def show_diff_table(files_to_edit: List[FileToEdit]) -> None:
    if not files_to_edit:
        return
    
    table = Table(title="[matrix.accent][ PROPOSED MODIFICATIONS ][/matrix.accent]", show_header=True, header_style="matrix.primary", show_lines=True, border_style="matrix.border")
    table.add_column("File Path", style="matrix.accent", no_wrap=True)
    table.add_column("Original", style="matrix.error dim")
    table.add_column("New", style="matrix.success")

    for edit in files_to_edit:
        table.add_row(edit.path, edit.original_snippet, edit.new_snippet)
    
    console.print(table)

def apply_diff_edit(path: str, original_snippet: str, new_snippet: str):
    """Reads the file at 'path', replaces the first occurrence of 'original_snippet' with 'new_snippet', then overwrites."""
    try:
        content = read_local_file(path)
        
        # Verify we're replacing the exact intended occurrence
        occurrences = content.count(original_snippet)
        if occurrences == 0:
            raise ValueError("Original snippet not found")
        if occurrences > 1:
            console.print(f"[matrix.warning]⚠ Multiple matches ({occurrences}) found - requiring line numbers for safety[/matrix.warning]")
            console.print("[matrix.dim]Use format:\n--- original.py (lines X-Y)\n+++ modified.py[/matrix.dim]")
            raise ValueError(f"Ambiguous edit: {occurrences} matches")
        
        updated_content = content.replace(original_snippet, new_snippet, 1)
        create_file(path, updated_content)
        console.print(f"[matrix.success]✓ MODIFICATION APPLIED:[/matrix.success] [matrix.accent]{path}[/matrix.accent]")

    except FileNotFoundError:
        console.print(f"[matrix.error]✗ FILE NOT FOUND:[/matrix.error] [matrix.accent]{path}[/matrix.accent]")
    except ValueError as e:
        console.print(f"[matrix.warning]⚠ {str(e)} in[/matrix.warning] [matrix.accent]{path}[/matrix.accent]. [matrix.warning]No changes made.[/matrix.warning]")
        console.print("\n[matrix.primary]Expected snippet:[/matrix.primary]")
        console.print(Panel(original_snippet, title="[matrix.accent][ EXPECTED ][/matrix.accent]", border_style="matrix.border", title_align="left"))
        console.print("\n[matrix.primary]Actual file content:[/matrix.primary]")
        console.print(Panel(content, title="[matrix.warning][ ACTUAL ][/matrix.warning]", border_style="matrix.warning", title_align="left"))

def try_handle_add_command(user_input: str) -> bool:
    prefix = "/add "
    if user_input.strip().lower().startswith(prefix):
        path_to_add = user_input[len(prefix):].strip()
        try:
            normalized_path = normalize_path(path_to_add)
            if os.path.isdir(normalized_path):
                # Handle entire directory
                add_directory_to_conversation(normalized_path)
            else:
                # Handle a single file as before
                content = read_local_file(normalized_path)
                conversation_history.append({
                    "role": "system",
                    "content": f"Content of file '{normalized_path}':\n\n{content}"
                })
                console.print(f"[matrix.success]✓ FILE LOADED:[/matrix.success] [matrix.accent]{normalized_path}[/matrix.accent]\n")
        except OSError as e:
            console.print(f"[matrix.error]✗ ERROR:[/matrix.error] [matrix.accent]{path_to_add}[/matrix.accent]: {e}\n")
        return True
    return False

def add_directory_to_conversation(directory_path: str):
    with console.status("[matrix.accent]> SCANNING DIRECTORY MATRIX...[/matrix.accent]", spinner="dots") as status:
        excluded_files = {
            # Python specific
            ".DS_Store", "Thumbs.db", ".gitignore", ".python-version",
            "uv.lock", ".uv", "uvenv", ".uvenv", ".venv", "venv",
            "__pycache__", ".pytest_cache", ".coverage", ".mypy_cache",
            # Node.js / Web specific
            "node_modules", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
            ".next", ".nuxt", "dist", "build", ".cache", ".parcel-cache",
            ".turbo", ".vercel", ".output", ".contentlayer",
            # Build outputs
            "out", "coverage", ".nyc_output", "storybook-static",
            # Environment and config
            ".env", ".env.local", ".env.development", ".env.production",
            # Misc
            ".git", ".svn", ".hg", "CVS"
        }
        excluded_extensions = {
            # Binary and media files
            ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp", ".avif",
            ".mp4", ".webm", ".mov", ".mp3", ".wav", ".ogg",
            ".zip", ".tar", ".gz", ".7z", ".rar",
            ".exe", ".dll", ".so", ".dylib", ".bin",
            # Documents
            ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
            # Python specific
            ".pyc", ".pyo", ".pyd", ".egg", ".whl",
            # UV specific
            ".uv", ".uvenv",
            # Database and logs
            ".db", ".sqlite", ".sqlite3", ".log",
            # IDE specific
            ".idea", ".vscode",
            # Web specific
            ".map", ".chunk.js", ".chunk.css",
            ".min.js", ".min.css", ".bundle.js", ".bundle.css",
            # Cache and temp files
            ".cache", ".tmp", ".temp",
            # Font files
            ".ttf", ".otf", ".woff", ".woff2", ".eot"
        }
        skipped_files = []
        added_files = []
        total_files_processed = 0
        max_files = 1000  # Reasonable limit for files to process
        max_file_size = 5_000_000  # 5MB limit

        for root, dirs, files in os.walk(directory_path):
            if total_files_processed >= max_files:
                console.print(f"[matrix.warning]⚠ Maximum file limit reached ({max_files})[/matrix.warning]")
                break

            status.update(f"[bold bright_blue]🔍 Scanning {root}...[/bold bright_blue]")
            # Skip hidden directories and excluded directories
            dirs[:] = [d for d in dirs if not d.startswith('.') and d not in excluded_files]

            for file in files:
                if total_files_processed >= max_files:
                    break

                if file.startswith('.') or file in excluded_files:
                    skipped_files.append(os.path.join(root, file))
                    continue

                _, ext = os.path.splitext(file)
                if ext.lower() in excluded_extensions:
                    skipped_files.append(os.path.join(root, file))
                    continue

                full_path = os.path.join(root, file)

                try:
                    # Check file size before processing
                    if os.path.getsize(full_path) > max_file_size:
                        skipped_files.append(f"{full_path} (exceeds size limit)")
                        continue

                    # Check if it's binary
                    if is_binary_file(full_path):
                        skipped_files.append(full_path)
                        continue

                    normalized_path = normalize_path(full_path)
                    content = read_local_file(normalized_path)
                    conversation_history.append({
                        "role": "system",
                        "content": f"Content of file '{normalized_path}':\n\n{content}"
                    })
                    added_files.append(normalized_path)
                    total_files_processed += 1

                except OSError:
                    skipped_files.append(full_path)

        console.print(f"[bold blue]✓[/bold blue] Added folder '[bright_cyan]{directory_path}[/bright_cyan]' to conversation.")
        if added_files:
            console.print(f"\n[bold bright_blue]📁 Added files:[/bold bright_blue] [dim]({len(added_files)} of {total_files_processed})[/dim]")
            for f in added_files:
                console.print(f"  [bright_cyan]📄 {f}[/bright_cyan]")
        if skipped_files:
            console.print(f"\n[bold yellow]⏭ Skipped files:[/bold yellow] [dim]({len(skipped_files)})[/dim]")
            for f in skipped_files[:10]:  # Show only first 10 to avoid clutter
                console.print(f"  [yellow dim]⚠ {f}[/yellow dim]")
            if len(skipped_files) > 10:
                console.print(f"  [dim]... and {len(skipped_files) - 10} more[/dim]")
        console.print()

def is_binary_file(file_path: str, peek_size: int = 1024) -> bool:
    try:
        with open(file_path, 'rb') as f:
            chunk = f.read(peek_size)
        # If there is a null byte in the sample, treat it as binary
        if b'\0' in chunk:
            return True
        return False
    except Exception:
        # If we fail to read, just treat it as binary to be safe
        return True

def ensure_file_in_context(file_path: str) -> bool:
    try:
        normalized_path = normalize_path(file_path)
        content = read_local_file(normalized_path)
        file_marker = f"Content of file '{normalized_path}'"
        if not any(file_marker in msg["content"] for msg in conversation_history):
            conversation_history.append({
                "role": "system",
                "content": f"{file_marker}:\n\n{content}"
            })
        return True
    except OSError:
        console.print(f"[bold red]✗[/bold red] Could not read file '[bright_cyan]{file_path}[/bright_cyan]' for editing context")
        return False

def normalize_path(path_str: str) -> str:
    """Return a canonical, absolute version of the path with security checks."""
    path = Path(path_str).resolve()
    
    # Prevent directory traversal attacks
    if ".." in path.parts:
        raise ValueError(f"Invalid path: {path_str} contains parent directory references")
    
    return str(path)

# --------------------------------------------------------------------------------
# 5. Conversation state
# --------------------------------------------------------------------------------
conversation_history = [
    {"role": "system", "content": system_PROMPT}
]

# --------------------------------------------------------------------------------
# 6. OpenAI API interaction with streaming
# --------------------------------------------------------------------------------

def execute_function_call_dict(tool_call_dict) -> str:
    """Execute a function call from a dictionary format and return the result as a string."""
    try:
        function_name = tool_call_dict["function"]["name"]
        arguments = json.loads(tool_call_dict["function"]["arguments"])
        
        if function_name == "read_file":
            file_path = arguments["file_path"]
            normalized_path = normalize_path(file_path)
            content = read_local_file(normalized_path)
            return f"Content of file '{normalized_path}':\n\n{content}"
            
        elif function_name == "read_multiple_files":
            file_paths = arguments["file_paths"]
            results = []
            for file_path in file_paths:
                try:
                    normalized_path = normalize_path(file_path)
                    content = read_local_file(normalized_path)
                    results.append(f"Content of file '{normalized_path}':\n\n{content}")
                except OSError as e:
                    results.append(f"Error reading '{file_path}': {e}")
            return "\n\n" + "="*50 + "\n\n".join(results)
            
        elif function_name == "create_file":
            file_path = arguments["file_path"]
            content = arguments["content"]
            create_file(file_path, content)
            return f"Successfully created file '{file_path}'"
            
        elif function_name == "create_multiple_files":
            files = arguments["files"]
            created_files = []
            for file_info in files:
                create_file(file_info["path"], file_info["content"])
                created_files.append(file_info["path"])
            return f"Successfully created {len(created_files)} files: {', '.join(created_files)}"
            
        elif function_name == "edit_file":
            file_path = arguments["file_path"]
            original_snippet = arguments["original_snippet"]
            new_snippet = arguments["new_snippet"]
            
            # Ensure file is in context first
            if not ensure_file_in_context(file_path):
                return f"Error: Could not read file '{file_path}' for editing"
            
            apply_diff_edit(file_path, original_snippet, new_snippet)
            return f"Successfully edited file '{file_path}'"
            
        else:
            return f"Unknown function: {function_name}"
            
    except Exception as e:
        return f"Error executing {function_name}: {str(e)}"

def execute_function_call(tool_call) -> str:
    """Execute a function call and return the result as a string."""
    try:
        function_name = tool_call.function.name
        arguments = json.loads(tool_call.function.arguments)
        
        if function_name == "read_file":
            file_path = arguments["file_path"]
            normalized_path = normalize_path(file_path)
            content = read_local_file(normalized_path)
            return f"Content of file '{normalized_path}':\n\n{content}"
            
        elif function_name == "read_multiple_files":
            file_paths = arguments["file_paths"]
            results = []
            for file_path in file_paths:
                try:
                    normalized_path = normalize_path(file_path)
                    content = read_local_file(normalized_path)
                    results.append(f"Content of file '{normalized_path}':\n\n{content}")
                except OSError as e:
                    results.append(f"Error reading '{file_path}': {e}")
            return "\n\n" + "="*50 + "\n\n".join(results)
            
        elif function_name == "create_file":
            file_path = arguments["file_path"]
            content = arguments["content"]
            create_file(file_path, content)
            return f"Successfully created file '{file_path}'"
            
        elif function_name == "create_multiple_files":
            files = arguments["files"]
            created_files = []
            for file_info in files:
                create_file(file_info["path"], file_info["content"])
                created_files.append(file_info["path"])
            return f"Successfully created {len(created_files)} files: {', '.join(created_files)}"
            
        elif function_name == "edit_file":
            file_path = arguments["file_path"]
            original_snippet = arguments["original_snippet"]
            new_snippet = arguments["new_snippet"]
            
            # Ensure file is in context first
            if not ensure_file_in_context(file_path):
                return f"Error: Could not read file '{file_path}' for editing"
            
            apply_diff_edit(file_path, original_snippet, new_snippet)
            return f"Successfully edited file '{file_path}'"
            
        else:
            return f"Unknown function: {function_name}"
            
    except Exception as e:
        return f"Error executing {function_name}: {str(e)}"

def trim_conversation_history():
    """Trim conversation history to prevent token limit issues while preserving tool call sequences"""
    if len(conversation_history) <= 20:  # Don't trim if conversation is still small
        return
        
    # Always keep the system prompt
    system_msgs = [msg for msg in conversation_history if msg["role"] == "system"]
    other_msgs = [msg for msg in conversation_history if msg["role"] != "system"]
    
    # Keep only the last 15 messages to prevent token overflow
    if len(other_msgs) > 15:
        other_msgs = other_msgs[-15:]
    
    # Rebuild conversation history
    conversation_history.clear()
    conversation_history.extend(system_msgs + other_msgs)

def stream_openai_response(user_message: str):
    # Add the user message to conversation history
    conversation_history.append({"role": "user", "content": user_message})
    
    # Trim conversation history if it's getting too long
    trim_conversation_history()

    # Remove the old file guessing logic since we'll use function calls
    try:
        stream = client.chat.completions.create(
            model="deepseek-reasoner",
            messages=conversation_history,
            tools=tools,
            max_completion_tokens=64000,
            stream=True
        )

        console.print("\n[matrix.accent]> CONNECTING TO THE MATRIX...[/matrix.accent]")
        reasoning_started = False
        reasoning_content = ""
        final_content = ""
        tool_calls = []

        for chunk in stream:
            # Handle reasoning content if available
            if hasattr(chunk.choices[0].delta, 'reasoning_content') and chunk.choices[0].delta.reasoning_content:
                if not reasoning_started:
                    console.print("\n[matrix.dim]// PROCESSING LOGIC:[/matrix.dim]")
                    reasoning_started = True
                console.print(chunk.choices[0].delta.reasoning_content, end="")
                reasoning_content += chunk.choices[0].delta.reasoning_content
            elif chunk.choices[0].delta.content:
                if reasoning_started:
                    console.print("\n")  # Add spacing after reasoning
                    console.print()  # Extra line for spacing
                    reasoning_started = False
                
                # First content chunk - show NEO prompt
                if not final_content:
                    console.print("[matrix.primary]NEO>[/matrix.primary] ", end="")
                
                final_content += chunk.choices[0].delta.content
                
                # Print content with better formatting
                content_chunk = chunk.choices[0].delta.content
                
                # Handle code blocks specially
                if "```" in content_chunk:
                    console.print(f"[matrix.accent]{content_chunk}[/matrix.accent]", end="")
                else:
                    console.print(f"[matrix.primary]{content_chunk}[/matrix.primary]", end="")
            elif chunk.choices[0].delta.tool_calls:
                # Handle tool calls
                for tool_call_delta in chunk.choices[0].delta.tool_calls:
                    if tool_call_delta.index is not None:
                        # Ensure we have enough tool_calls
                        while len(tool_calls) <= tool_call_delta.index:
                            tool_calls.append({
                                "id": "",
                                "type": "function",
                                "function": {"name": "", "arguments": ""}
                            })
                        
                        if tool_call_delta.id:
                            tool_calls[tool_call_delta.index]["id"] = tool_call_delta.id
                        if tool_call_delta.function:
                            if tool_call_delta.function.name:
                                tool_calls[tool_call_delta.index]["function"]["name"] += tool_call_delta.function.name
                            if tool_call_delta.function.arguments:
                                tool_calls[tool_call_delta.index]["function"]["arguments"] += tool_call_delta.function.arguments

        console.print()  # New line after streaming

        # Store the assistant's response in conversation history
        assistant_message = {
            "role": "assistant",
            "content": final_content if final_content else None
        }
        
        if tool_calls:
            # Convert our tool_calls format to the expected format
            formatted_tool_calls = []
            for i, tc in enumerate(tool_calls):
                if tc["function"]["name"]:  # Only add if we have a function name
                    # Ensure we have a valid tool call ID
                    tool_id = tc["id"] if tc["id"] else f"call_{i}_{int(time.time() * 1000)}"
                    
                    formatted_tool_calls.append({
                        "id": tool_id,
                        "type": "function",
                        "function": {
                            "name": tc["function"]["name"],
                            "arguments": tc["function"]["arguments"]
                        }
                    })
            
            if formatted_tool_calls:
                # Important: When there are tool calls, content should be None or empty
                if not final_content:
                    assistant_message["content"] = None
                    
                assistant_message["tool_calls"] = formatted_tool_calls
                conversation_history.append(assistant_message)
                
                # Execute tool calls and add results immediately
                console.print(f"\n[bold bright_cyan]⚡ Executing {len(formatted_tool_calls)} function call(s)...[/bold bright_cyan]")
                for tool_call in formatted_tool_calls:
                    console.print(f"[bright_blue]→ {tool_call['function']['name']}[/bright_blue]")
                    
                    try:
                        result = execute_function_call_dict(tool_call)
                        
                        # Add tool result to conversation immediately
                        tool_response = {
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "content": result
                        }
                        conversation_history.append(tool_response)
                    except Exception as e:
                        console.print(f"[red]Error executing {tool_call['function']['name']}: {e}[/red]")
                        # Still need to add a tool response even on error
                        conversation_history.append({
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "content": f"Error: {str(e)}"
                        })
                
                # Get follow-up response after tool execution
                console.print("\n[bold bright_blue]🔄 Processing results...[/bold bright_blue]")
                
                follow_up_stream = client.chat.completions.create(
                    model="deepseek-reasoner",
                    messages=conversation_history,
                    tools=tools,
                    max_completion_tokens=64000,
                    stream=True
                )
                
                follow_up_content = ""
                reasoning_started = False
                
                for chunk in follow_up_stream:
                    # Handle reasoning content if available
                    if hasattr(chunk.choices[0].delta, 'reasoning_content') and chunk.choices[0].delta.reasoning_content:
                        if not reasoning_started:
                            console.print("\n[matrix.dim]// PROCESSING LOGIC:[/matrix.dim]")
                            reasoning_started = True
                        console.print(chunk.choices[0].delta.reasoning_content, end="")
                    elif chunk.choices[0].delta.content:
                        if reasoning_started:
                            console.print("\n")
                            console.print()  # Extra spacing
                            reasoning_started = False
                        
                        # First content chunk - show NEO prompt
                        if not follow_up_content:
                            console.print("[matrix.primary]NEO>[/matrix.primary] ", end="")
                        
                        follow_up_content += chunk.choices[0].delta.content
                        
                        # Better formatting for follow-up content
                        content_chunk = chunk.choices[0].delta.content
                        if "```" in content_chunk:
                            console.print(f"[matrix.accent]{content_chunk}[/matrix.accent]", end="")
                        else:
                            console.print(f"[matrix.primary]{content_chunk}[/matrix.primary]", end="")
                
                console.print()
                
                # Store follow-up response
                conversation_history.append({
                    "role": "assistant",
                    "content": follow_up_content
                })
        else:
            # No tool calls, just store the regular response
            conversation_history.append(assistant_message)

        return {"success": True}

    except Exception as e:
        error_msg = f"Matrix connection lost: {str(e)}"
        console.print(f"\n[matrix.error]> SYSTEM ERROR: {error_msg}[/matrix.error]")
        return {"error": error_msg}

# --------------------------------------------------------------------------------
# 7. Main interactive loop
# --------------------------------------------------------------------------------

class MatrixRain:
    """Matrix-style digital rain effect"""
    
    def __init__(self, width: int = 80, height: int = 5):
        self.width = width
        self.height = height
        self.columns = {}
        self.speeds = {}
        
    def update(self):
        """Update rain animation"""
        # Add new columns
        for col in range(self.width):
            if col not in self.columns:
                if random.random() < 0.02:  # Spawn rate
                    self.columns[col] = []
                    self.speeds[col] = random.uniform(0.5, 1.5)
        
        # Update existing columns
        for col in list(self.columns.keys()):
            if random.random() < self.speeds[col] * 0.1:
                self.columns[col].append(random.choice(MATRIX_CHARS))
            
            # Limit column length
            if len(self.columns[col]) > self.height:
                self.columns[col].pop(0)
            
            # Remove empty columns randomly
            if len(self.columns[col]) == 0 and random.random() < 0.1:
                del self.columns[col]
                del self.speeds[col]
    
    def render(self) -> str:
        """Render the rain effect"""
        self.update()
        
        # Create grid
        grid = [[' ' for _ in range(self.width)] for _ in range(self.height)]
        
        # Fill with rain
        for col, chars in self.columns.items():
            for i, char in enumerate(chars):
                row = self.height - len(chars) + i
                if 0 <= row < self.height:
                    grid[row][col] = char
        
        # Convert to string with styling
        lines = []
        for row in grid:
            line = Text()
            for char in row:
                if char != ' ':
                    # Brighter at the bottom
                    line.append(char, style="matrix.rain")
            lines.append(line)
        
        return "\n".join(str(line) for line in lines)


def display_matrix_exit():
    """Display Matrix rain exit sequence."""
    console.print("\n[matrix.dim]> Exiting the Matrix...[/matrix.dim]")
    rain = MatrixRain(width=80, height=10)
    for _ in range(20):
        console.clear()
        console.print(rain.render())
        time.sleep(0.1)
    console.print("\n[matrix.primary]> Remember... there is no spoon.[/matrix.primary]")


def main():
    # Clear screen
    console.clear()
    
    # Show ASCII art with rain effect
    rain = MatrixRain(width=80, height=3)
    console.print(rain.render())
    
    # Show NEO ASCII
    console.print(Align.center(Text(NEO_ASCII, style="matrix.primary")))
    
    # Show random quote
    quote = random.choice(MATRIX_QUOTES)
    console.print(Align.center(Text(quote, style="matrix.dim italic")))
    
    console.print("\n" * 2)
    
    # System info
    info = Panel(
        Text.from_markup(
            "[matrix.primary]SYSTEM: NEO v1.0[/matrix.primary]\n"
            "[matrix.secondary]STATUS: ONLINE[/matrix.secondary]\n"
            "[matrix.dim]REALITY: SIMULATED[/matrix.dim]"
        ),
        title="[matrix.accent][ SYSTEM INFO ][/matrix.accent]",
        border_style="matrix.border",
        width=40
    )
    console.print(Align.center(info))
    
    # Show commands
    console.print("\n[matrix.dim]COMMANDS: /add <path> | /clear | /exit | /red_pill | /blue_pill[/matrix.dim]\n")

    try:
        while True:
            try:
                user_input = prompt_session.prompt("neo@matrix:~$: ").strip()
            except (EOFError, KeyboardInterrupt):
                console.print("\n[matrix.warning]> MATRIX DISCONNECTION DETECTED[/matrix.warning]")
                display_matrix_exit()
                break

            if not user_input:
                continue

            if user_input.lower() in ["exit", "quit", "/exit", "/quit"]:
                console.print("[matrix.dim]> Disconnecting from the Matrix...[/matrix.dim]")
                display_matrix_exit()
                break
            
            # Handle special Matrix commands
            if user_input.lower() == "/red_pill":
                console.print("[matrix.error]> You take the red pill...[/matrix.error]")
                time.sleep(1)
                console.print("[matrix.primary]> Welcome to the desert of the real.[/matrix.primary]\n")
                continue
            elif user_input.lower() == "/blue_pill":
                console.print("[matrix.accent]> You take the blue pill...[/matrix.accent]")
                time.sleep(1)
                console.print("[matrix.dim]> Wake up. Believe whatever you want to believe.[/matrix.dim]\n")
                continue
            elif user_input.lower() == "/clear":
                console.clear()
                console.print("[matrix.success]> Memory wiped. You are free.[/matrix.success]\n")
                conversation_history.clear()
                conversation_history.append({"role": "system", "content": system_PROMPT})
                continue

            if try_handle_add_command(user_input):
                continue

            response_data = stream_openai_response(user_input)
            
            if response_data.get("error"):
                console.print(f"[matrix.error]> SYSTEM ERROR: {response_data['error']}[/matrix.error]")
    
    except KeyboardInterrupt:
        # Handle Ctrl+C gracefully with Matrix exit
        console.print("\n[matrix.warning]> INTERRUPT DETECTED - EMERGENCY MATRIX EXIT[/matrix.warning]")
        display_matrix_exit()
    except Exception as e:
        console.print(f"\n[matrix.error]> CRITICAL ERROR: {str(e)}[/matrix.error]")
        console.print("[matrix.dim]> Forcing emergency exit...[/matrix.dim]")

if __name__ == "__main__":
    main()