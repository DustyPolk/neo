![image](https://github.com/user-attachments/assets/16389c5c-c1bd-4051-926b-458a997d0bc4)

## Meet Neo. An AI coding agent based on the Matrix.

## Prerequisites

- Python 3.11+
- [uv](https://github.com/astral-sh/uv) (Python package installer and virtual environment manager)

## Setup and Running the Project

1.  **Clone the repository (if you haven't already):**
    ```bash
    git clone <repository-url>
    cd neo
    ```

2.  **Create a virtual environment using `uv`:**
    ```bash
    uv venv
    ```

3.  **Activate the virtual environment:**
    -   On macOS and Linux:
        ```bash
        source .venv/bin/activate
        ```
    -   On Windows (Git Bash or WSL):
        ```bash
        source .venv/Scripts/activate
        ```
    -   On Windows (Command Prompt):
        ```bash
        .venv\Scripts\activate.bat
        ```
    -   On Windows (PowerShell):
        ```bash
        .venv\Scripts\Activate.ps1
        ```
    Your terminal prompt should change to indicate that the virtual environment is active (e.g., `(.venv) user@host:...$`).

4.  **Install dependencies using `uv`:**
    ```bash
    uv pip install -r requirements.txt
    ```

5.  **Run the application:**
    The main script is `neo.py`. You can run it using:
    ```bash
    python3 neo.py
    ```
    Alternatively, you can use `uv run`:
    ```bash
    uv run neo.py
    ```

## Environment Variables

This project uses a `.env` file for environment variables. If the project requires specific API keys or configurations, create a `.env` file in the root of the project and add them there. For example:

```
OPENAI_API_KEY="your_api_key_here"
# Add other environment variables as needed
```
**Note:** The `.env` file is included in `.gitignore` and should not be committed to version control.

## Contributing

[Information about how to contribute to the project, if applicable]

## License

[Specify project license, if applicable] 
