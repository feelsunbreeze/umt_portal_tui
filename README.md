# UMT Portal TUI

> A Natural Language Processing-powered Terminal User Interface for the University of Management and Technology (UMT Lahore) student portal

![preview](https://raw.githubusercontent.com/feelsunbreeze/umt_portal_tui/refs/heads/main/tui.gif)

## 🌟 Overview

UMT Portal TUI transforms the complex UMT student portal into an intelligent, conversational terminal interface. Built for an NLP university project, it demonstrates practical applications of natural language processing in improving user experience.

## ✨ Key Features

### 🤖 AI-Powered Chatbot
- **Natural Language Understanding**: Ask questions in plain English
- **Intent Classification**: Automatically understands what you want (CGPA, attendance, grades, etc.)
- **Entity Extraction**: Identifies courses, semesters, and query specifics from your questions
- **Smart Responses**: Context-aware conversation with helpful feedback

### 🎯 NLP Techniques Implemented
- Intent classification with confidence scoring
- Keyword-based pattern matching
- Fuzzy string matching (Levenshtein distance) for typo tolerance
- Named entity extraction (courses, semesters)
- Query-specific information filtering

### 🚀 Performance Enhancements
- **Smart Caching**: Unlike the original portal, we cache transcripts and attendance locally
- **Retry Logic**: Automatically retries failed requests (up to 10 times with 2-second delays)
- **Faster Access**: Cached data loads instantly

### 📊 Portal Features
- 🔐 Secure login with optional credential storage
- 📚 View all enrolled courses with complete details
- 📊 Check attendance with lecture-by-lecture breakdown
- 📝 View assessments and marks
- 📄 Complete academic transcript with SGPA/CGPA
- 👨‍🏫 Faculty information with decoded emails

## 🛠️ Technical Stack

- **Language**: Go 1.24.5
- **TUI Framework**: [Bubbletea](https://github.com/charmbracelet/bubbletea) (Elm Architecture)
- **Styling**: [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Web Scraping**: [GoQuery](https://github.com/PuerkitoBio/goquery)
- **NLP**: Custom intent matcher with fuzzy matching

## 📖 Documentation

For comprehensive documentation covering NLP techniques, architecture, and implementation details, see:

**[📚 Complete Documentation](DOCUMENTATION.md)**

## 🚀 Quick Start

### Prerequisites
- Go 1.24 or higher
- UMT student credentials
- Internet connection

### Installation

```bash
# Navigate to project directory
cd umt_portal_tui

# Install dependencies
go mod download

# Build
go build -o umt_tui.exe ./cmd/umt_portal_tui

# Run
./umt_tui.exe
```

## 💬 Chat Examples

```
"What's my CGPA?"
"Show my attendance for Database Systems"
"Tell me about my third semester"
"Who teaches Data Structures?"
"Check my assessments"
"What's my SGPA for Fall 2023?"
```

## ⌨️ Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `c` | Open AI chat assistant |
| `t` | View transcript |
| `r` | Refresh current view |
| `l` | Logout |
| `↑/↓` or `j/k` | Navigate |
| `Enter` | Select |
| `Esc` | Go back |
| `q` | Quit |

## 🎓 Academic Context

This project was developed as part of a Natural Language Processing course to demonstrate:
- Practical application of intent classification
- Entity extraction in academic contexts
- Fuzzy matching for user input tolerance
- Conversation management and context handling
- Integration of NLP with real-world systems

## 🔮 Future Enhancements

- [ ] Machine learning-based intent classifier
- [ ] Multi-turn conversation with context memory
- [ ] Payment history & fee voucher generation
- [ ] PRS (Program Registration) requests
- [ ] Add/drop course functionality
- [ ] Grade prediction based on current assessments
- [ ] Attendance alerts and deadline reminders

## 👨‍💻 Author

**Sunbreeze**  
Natural Language Processing Project  
University of Management and Technology (UMT Lahore)

## 📄 License

This project is for educational purposes.
