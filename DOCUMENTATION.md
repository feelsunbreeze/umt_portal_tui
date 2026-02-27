---
marp: true
---

# UMT Portal TUI - Natural Language Processing Project Documentation

## Table of Contents
1. [Project Overview](#project-overview)
2. [Natural Language Processing Components](#natural-language-processing-components)
3. [Technical Architecture](#technical-architecture)
4. [Features](#features)
5. [Installation & Usage](#installation--usage)
6. [Technical Implementation Details](#technical-implementation-details)
7. [Future Enhancements](#future-enhancements)

---

## Project Overview

**UMT Portal TUI** is a Terminal User Interface (TUI) application designed to provide an intelligent, NLP-powered interface to the University of Management and Technology (UMT Lahore) student portal. This project demonstrates the application of Natural Language Processing techniques to transform complex web-based interactions into an intuitive conversational interface.

### Key Highlights

- **NLP-Powered Chatbot**: Uses natural language understanding to interpret user queries and execute portal actions
- **Intent Classification**: Implements custom intent recognition for academic queries
- **Entity Extraction**: Automatically extracts courses, semesters, and query specifics from natural language
- **Intelligent Caching**: Implements a caching layer not present in the original portal for improved performance
- **Robust Error Handling**: Includes retry mechanisms to handle unreliable portal responses
- **Modern TUI**: Built with Charm's Bubbletea and Lipgloss for a beautiful terminal interface

### Technologies Used

- **Language**: Go 1.24.5
- **TUI Framework**: [Bubbletea](https://github.com/charmbracelet/bubbletea) - The Elm Architecture for Go
- **Styling**: [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for terminal UIs
- **Web Scraping**: [GoQuery](https://github.com/PuerkitoBio/goquery) - jQuery-like HTML parsing
- **NLP**: Custom implementation with keyword matching, fuzzy string matching, and intent classification

---

## Natural Language Processing Components

This project employs several NLP techniques to understand and process user queries. The NLP system is designed to handle academic-related questions in a university context.

### 2.1 Intent Classification System

The intent classification system identifies the user's goal from natural language input. It supports the following intents:

#### Intent Categories

| Intent | Description | Example Queries |
|--------|-------------|-----------------|
| `check_cgpa` | Query about CGPA/GPA | "What's my CGPA?", "Show me my grade point" |
| `attendance` | Check attendance for courses | "Show attendance", "How many lectures did I attend?" |
| `transcript` | View academic transcript | "Show my grades", "Display transcript", "What's my SGPA?" |
| `course_details` | Get information about enrolled courses | "Tell me about my courses", "Who teaches CS101?" |
| `assessment` | Check assessments/assignments | "Show my assessments", "What are my quiz marks?" |
| `greeting` | Conversational greetings | "Hi", "Hello", "How are you?" |
| `identity` | Questions about the bot | "Who are you?", "Who made you?" |

**Implementation**: [`intent_matcher.go`](file:///c:/Users/matth/OneDrive/Documents/Coding/NLP%20Final/umt_portal_tui/cmd/umt_portal_tui/intent_matcher.go)

```go
type IntentMatcher struct {
    keywords map[string][]string
}
```

The system maintains a keyword dictionary for each intent and uses a scoring mechanism to determine the best match.

### 2.2 Keyword Matching & Scoring Algorithm

The intent matcher uses a multi-tiered scoring system:

1. **Exact Phrase Match** (+10.0 points): The keyword phrase appears exactly in the query
2. **Word-Level Match** (+2.0 points): Individual words from keywords match query words
3. **Fuzzy Match** (variable score): Uses Levenshtein distance for typo tolerance

```go
// Scoring logic
if strings.Contains(query, normalizedKeyword) {
    matches[intentName] += 10.0  // Exact phrase match
}

// Word-level matching with fuzzy tolerance
dist := levenshtein(queryWord, keyPart)
if dist <= 2 && len(keyPart) > 3 {
    score := 1.0 - (float64(dist) / float64(len(keyPart)))
    matches[intentName] += score
}
```

### 2.3 Levenshtein Distance for Fuzzy Matching

The system implements the **Levenshtein distance algorithm** to handle typos and spelling variations:

```go
func levenshtein(s, t string) int {
    d := make([][]int, len(s)+1)
    for i := range d {
        d[i] = make([]int, len(t)+1)
    }
    // Dynamic programming implementation
    // Calculates minimum edit distance between two strings
}
```

**NLP Technique**: Edit distance algorithms are fundamental in spell checking, text similarity, and fuzzy string matching applications.

### 2.4 Entity Extraction

The system performs sophisticated entity extraction to identify:

#### Course Extraction

Extracts course information from queries using:
- **Code matching**: Detects course codes (e.g., "CS201", "MATH101")
- **Title matching**: Matches course titles with word-level similarity
- **Fuzzy matching**: Tolerates misspellings in course names

```go
type MatchResult struct {
    Intent            string
    Confidence        float64
    ExtractedCourse   *Course   // Extracted course entity
    ExtractedSemester int       // Extracted semester number
    SpecificQuery     string    // Specific sub-query (e.g., "sgpa", "cgpa")
}
```

#### Semester Extraction

Supports multiple representations:
- Numeric: "1", "2", "3", ..., "8"
- Ordinal: "1st", "2nd", "3rd", "4th", ...
- Textual: "first", "second", "third", ...

```go
semMap := map[string]int{
    "1st": 1, "first": 1, "one": 1, "1": 1,
    "2nd": 2, "second": 2, "two": 2, "2": 2,
    // ... and so on
}
```

#### Specific Query Detection

Within transcript queries, the system identifies specific information requests:
- **SGPA queries**: "Show me SGPA for 3rd semester"
- **CGPA queries**: "What was my CGPA in semester 2?"
- **Course list queries**: "Which courses did I take in Fall 2023?"

### 2.5 Confidence Scoring

The system normalizes confidence scores to a 0.0-1.0 range:

```go
confidence := maxScore / 5.0
if confidence > 1.0 {
    confidence = 1.0
}
```

Only matches with a score ≥ 1.0 are considered valid intents. This threshold ensures that at least some keyword overlap exists before classifying an intent.

### 2.6 Chat Handler & Intent Routing

**Implementation**: [`chat_handler.go`](file:///c:/Users/matth/OneDrive/Documents/Coding/NLP%20Final/umt_portal_tui/cmd/umt_portal_tui/chat_handler.go)

The chat handler routes classified intents to appropriate actions:

```go
func (m model) handleIntent(msg NLPClassificationMsg) (tea.Model, tea.Cmd) {
    switch msg.Intent {
    case "check_cgpa":
        // Display CGPA immediately
    case "attendance":
        // Fetch and display attendance
    case "transcript":
        // Handle transcript queries with semester filtering
    case "course_details":
        // Show course information
    case "assessment":
        // Display assessments
    // ... more intents
    }
}
```

### 2.7 Context-Aware Responses

The chatbot maintains conversation context through:
- **Chat history**: Stores recent conversation for context
- **Awaiting state**: Tracks when the bot expects specific input (e.g., course selection)
- **Last classification**: Remembers the previous query for follow-up questions

```go
type model struct {
    chatHistory             []string
    awaitingCourseSelection bool
    pendingAction           string
    lastClassification      *NLPClassificationMsg
}
```

---

## Technical Architecture

### 3.1 System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      User Interface (TUI)                    │
│  ┌───────────┐ ┌──────────┐ ┌────────────┐ ┌─────────────┐ │
│  │ Login     │ │ Courses  │ │ Transcript │ │ AI Chatbot  │ │
│  │ View      │ │ View     │ │ View       │ │ Interface   │ │
│  └───────────┘ └──────────┘ └────────────┘ └─────────────┘ │
└────────────────────┬────────────────────────────────────────┘
                     │ Bubbletea Elm Architecture
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                    Business Logic Layer                      │
│  ┌─────────────────┐  ┌──────────────────────────────────┐ │
│  │ Intent Matcher  │  │      Session Manager             │ │
│  │ (NLP Engine)    │  │   ┌───────────┐ ┌─────────────┐ │ │
│  │                 │  │   │ Auth      │ │ Cache       │ │ │
│  │ • Keyword Match │  │   │ Handler   │ │ Manager     │ │ │
│  │ • Entity Extract│  │   └───────────┘ └─────────────┘ │ │
│  │ • Fuzzy Match   │  │                                  │ │
│  └─────────────────┘  └──────────────────────────────────┘ │
└────────────────────┬───────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                      API Integration Layer                   │
│  ┌──────────────┐  ┌────────────┐  ┌────────────────────┐  │
│  │ HTTP Client  │  │ GoQuery    │  │ Retry Logic        │  │
│  │   + Cookies  │  │ HTML Parser│  │ (10 max retries)   │  │
│  └──────────────┘  └────────────┘  └────────────────────┘  │
└────────────────────┬───────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                  UMT Portal (online.umt.edu.pk)              │
│  • Authentication       • Attendance Reports                 │
│  • Course Information   • Assessment Data                    │
│  • Academic Transcripts                                      │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 File Structure

```
umt_portal_tui/
├── cmd/
│   └── umt_portal_tui/
│       ├── main.go              # Entry point
│       ├── ui.go                # TUI views and rendering (BubbleTea)
│       ├── api.go               # Portal API integration & web scraping
│       ├── logic.go             # Business logic & data structures
│       ├── nlp_classifier.go    # Python model integration (unused)
│       ├── intent_matcher.go    # NLP: Intent classification
│       └── chat_handler.go      # NLP: Chat routing & responses
├── go.mod                       # Dependencies
├── go.sum
├── README.md
└── tui.gif                      # Demo
```

### 3.3 Data Flow

#### Authentication Flow
```
User Input → Login Form → API (loginAPI) → Cookie Storage → Session Creation
```

#### Query Processing Flow
```
Natural Language Query 
    ↓
Intent Classification (IntentMatcher)
    ↓
Entity Extraction (Course, Semester)
    ↓
Intent Routing (handleIntent)
    ↓
API Call with Retry Logic
    ↓
Cache Update
    ↓
UI Update (Bubbletea)
```

---

## Features

### 4.1 Intelligent Chat Interface

The AI-powered chat interface understands natural language queries without requiring users to navigate complex menus.

**Examples:**
- "What's my CGPA?" → Instantly displays CGPA
- "Show attendance for data structures" → Finds course, fetches attendance
- "Tell me about my third semester" → Displays semester transcript
- "Who teaches linear algebra?" → Shows course faculty information

### 4.2 Smart Caching System

Unlike the original UMT portal, this application implements intelligent caching:

**Cached Data:**
- Academic transcripts (JSON format)
- User credentials (optional, encrypted)
- Course attendance (per-course)

**Cache Location:** `%LOCALAPPDATA%/umt_tui/`

**Benefits:**
- Faster data retrieval
- Reduced server load
- Offline access to cached data
- Improved user experience

**Implementation:**
```go
func saveTranscriptCache(s *Session) error {
    cacheDir, _ := os.UserCacheDir()
    appCacheDir := filepath.Join(cacheDir, "umt_tui")
    os.MkdirAll(appCacheDir, 0755)
    
    data, _ := json.MarshalIndent(transcript, "", "  ")
    cacheFile := filepath.Join(appCacheDir, "transcript.json")
    os.WriteFile(cacheFile, data, 0644)
}
```

### 4.3 Retry Mechanism

The portal often returns incomplete or failed responses. This application implements robust retry logic:

**Retry Parameters:**
- Maximum retries: 10
- Retry delay: 2 seconds
- Applies to: Attendance, Assessments, Transcript

**Implementation:**
```go
maxRetries := 10
for range maxRetries {
    // Attempt to fetch data
    if success {
        return nil
    }
    time.Sleep(time.Second * 2)
    continue
}
```

**Why This Matters:**
The original portal frequently fails to load attendance or transcript data on the first try. Our retry mechanism ensures data is eventually retrieved, significantly improving reliability.

### 4.4 Course Information

Displays comprehensive course details:
- Course code and title
- Credit hours
- Faculty name and email (decoded from obfuscated format)
- Section and semester
- Mode of delivery (Online/On-campus)

### 4.5 Attendance Tracking

Detailed attendance records:
- Lecture-by-lecture attendance status
- Total lectures delivered
- Attendance percentage
- Faculty who delivered each lecture
- Date of each lecture

### 4.6 Assessment Tracking

Complete assessment breakdown:
- Assignment/quiz names
- Obtained marks vs total marks
- Assigned dates
- Percentage calculations

### 4.7 Academic Transcript

Full transcript with:
- Semester-wise course breakdown
- Grades and grade points
- Credit hours earned per semester
- SGPA (Semester GPA) per semester
- CGPA progression
- Cumulative credit hours

---

## Installation & Usage

### 5.1 Prerequisites

- Go 1.24 or higher
- Internet connection
- Valid UMT student credentials

### 5.2 Installation

```bash
# Clone the repository
cd "c:/Users/matth/OneDrive/Documents/Coding/NLP Final"
cd umt_portal_tui

# Install dependencies
go mod download

# Build the application
go build -o umt_tui.exe ./cmd/umt_portal_tui

# Run the application
./umt_tui.exe
```

### 5.3 Usage Guide

#### Login
1. Enter your student ID
2. Enter your password
3. Optionally enable "Remember Me" for automatic login
4. Press Enter or click Login

#### Navigation
- **Courses View**: Use ↑/↓ or j/k to navigate courses
- **Chat Interface**: Press `c` from courses view
- **View Transcript**: Press `t` from courses view
- **Course Details**: Press Enter on a selected course

#### Chat Commands

Natural language is supported, but here are some examples:

```
"What's my CGPA?"
"Show my attendance"
"Tell me about Database Systems"
"Who teaches CS201?"
"Show my 3rd semester transcript"
"What's my SGPA for Fall 2023?"
"Check my assessments for Data Structures"
```

### 5.4 Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate lists |
| `Enter` | Select/Confirm |
| `Esc` | Go back / Toggle password visibility |
| `c` | Open chat interface |
| `t` | View transcript |
| `r` | Refresh current view |
| `l` | Logout |
| `q` or `Ctrl+C` | Quit application |

---

## Technical Implementation Details

### 6.1 Email Decoding Algorithm

The portal obfuscates faculty emails using CloudFlare's email protection. We decode them:

```go
func decodeFacultyEmail(email string) string {
    data, _ := hex.DecodeString(email)
    key := data[0]  // First byte is XOR key
    
    decoded := make([]byte, len(data)-1)
    for i := 1; i < len(data); i++ {
        decoded[i-1] = data[i] ^ key  // XOR decryption
    }
    return string(decoded)
}
```

### 6.2 Transcript Parsing

The transcript is embedded in a report viewer control with complex HTML structure:

```go
// Extract data from specific div classes
doc.Find("div.canGrowTextBoxInTablix.cannotShrinkTextBoxInTablix").Each(func(i int, s *goquery.Selection) {
    currentText := strings.TrimSpace(s.Text())
    extractedData = append(extractedData, currentText)
})

// Parse extracted data into structured format
func parseTranscript(s *Session, extractedData []string) error {
    // Identify semesters (Fall/Spring/Summer)
    // Extract course codes, titles, credit hours, grades
    // Calculate SGPA and CGPA
}
```

### 6.3 Session Management

Sessions maintain state across the application:

```go
type Session struct {
    loggedIn bool
    Student  Student
    Cookies  []*http.Cookie
}
```

Cookies are preserved across requests to maintain authentication.

### 6.4 Bubbletea Architecture

The application follows The Elm Architecture:

```go
type model struct {
    // State
    currentView    ViewType
    session        *Session
    courses        []Course
    chatHistory    []string
    // ... more state
}

func (m model) Init() tea.Cmd { /* Initialize */ }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { /* Handle messages */ }
func (m model) View() string { /* Render UI */ }
```

Messages trigger state updates, which trigger view re-renders.

### 6.5 Styling with Lipgloss

Beautiful terminal UI using Lipgloss:

```go
const (
    BLUE        = lipgloss.Color("#0043a8")
    GREEN       = lipgloss.Color("#50FA7B")
    LIGHT_BLUE  = lipgloss.Color("#8BE9FD")
    // ... more colors
)

titleStyle := lipgloss.NewStyle().
    Bold(true).
    Foreground(LIGHT_BLUE).
    MarginBottom(2)
```

---

## Future Enhancements

### 7.1 Advanced NLP Features

- **Machine Learning Model**: Train a proper ML classifier on student query data
- **Context Memory**: Multi-turn conversations with context retention
- **Semantic Understanding**: Use embeddings for better intent matching NER (Named Entity Recognition): Better course and faculty name extraction
- **Sentiment Analysis**: Detect user frustration and provide helpful responses

### 7.2 Additional Features

- [ ] Payment history and fee voucher generation
- [ ] PRS (Program Registration System) requests
- [ ] Add/drop course functionality
- [ ] Grade predictions based on current assessments
- [ ] Attendance alerts (when nearing minimum requirement)
- [ ] Assignment deadline reminders
- [ ] Course recommendations

### 7.3 Technical Improvements

- [ ] Parallel data fetching for faster load times
- [ ] Background cache updates
- [ ] Offline mode with cached data
- [ ] Export transcripts to PDF
- [ ] Data visualization (attendance graphs, grade distributions)

---

## NLP Techniques Summary

This project demonstrates the following NLP concepts:

1. **Intent Classification**: Categorizing user queries into predefined intents
2. **Entity Extraction**: Identifying specific entities (courses, semesters) from text
3. **Keyword Matching**: Pattern-based text classification
4. **Fuzzy String Matching**: Levenshtein distance for typo tolerance
5. **Confidence Scoring**: Numerical evaluation of classification certainty
6. **Context Management**: Maintaining conversation state
7. **Natural Language Interface**: Transforming structured data access into conversational interaction

---

## Conclusion

UMT Portal TUI demonstrates how Natural Language Processing can transform complex web interfaces into intuitive conversational experiences. By combining NLP techniques with modern Go development practices, we've created a tool that is not only more user-friendly than the original portal but also more reliable and performant.

The project showcases practical applications of:
- Intent classification and entity extraction
- Fuzzy matching and spelling tolerance
- Intelligent caching strategies
- Robust error handling with retries
- Modern terminal UI development

This serves as an excellent example of applied NLP in a real-world academic context, solving genuine user experience problems through natural language understanding.

---

**Author**: Sunbreeze  
**Course**: Natural Language Processing  
**Institution**: University of Management and Technology (UMT Lahore)  
**Year**: 2026
