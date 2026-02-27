package main

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) handleIntent(msg NLPClassificationMsg) (tea.Model, tea.Cmd) {
	if m.session == nil || !m.session.loggedIn {
		m.chatHistory = append(m.chatHistory, "⚠️ Please log in first")
		return m, nil
	}

	switch msg.Intent {
	case "check_cgpa":
		m.chatHistory = append(m.chatHistory, fmt.Sprintf("📊 Your CGPA is: %s", m.session.Student.CgpaEarned))

	case "attendance":
		if msg.ExtractedCourse != nil {
			selectedCourse := *msg.ExtractedCourse
			m.chatHistory = append(m.chatHistory, fmt.Sprintf("🎯 Found course: %s", selectedCourse.Code))
			m.chatHistory = append(m.chatHistory, fmt.Sprintf("🔄 Fetching attendance for %s...", selectedCourse.Code))

			m.setLoadingState(fmt.Sprintf("📊 Getting attendance for %s...", selectedCourse.Code), "Fetching attendance records", "• Esc: Back to chat • Q: Cancel and quit")
			m.currentView = LoadingView
			m.lastView = ChatView
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					err := m.session.GetCourseAttendance(false, selectedCourse.ID)
					if err != nil {
						return CourseActionMsg{
							Action:   "attendance",
							CourseID: selectedCourse.ID,
							Error:    err,
							Success:  false,
						}
					}
					for i, c := range m.session.Student.Courses {
						if c.ID == selectedCourse.ID {
							m.selectedCourse = i
							break
						}
					}
					return CourseActionMsg{
						Action:         "attendance",
						CourseID:       selectedCourse.ID,
						Error:          nil,
						Success:        true,
						UpdatedCourses: m.session.Student.Courses,
					}
				},
			)
		} else {
			m.chatHistory = append(m.chatHistory, "🔢 Please select a course by number:")
			m.awaitingCourseSelection = true
			m.pendingAction = "attendance"
		}

	case "greeting":
		m.chatHistory = append(m.chatHistory, "👋 Hello! I'm the UMT Portal AI Assistant, created by Sunbreeze.")
		m.chatHistory = append(m.chatHistory, "I can help you check your marks, attendance, and transcript. How can I help you today?")

	case "identity":
		m.chatHistory = append(m.chatHistory, "🤖 I am the UMT Portal TUI Assistant.")
		m.chatHistory = append(m.chatHistory, "I was created by Sunbreeze to help students access their portal data easily via the terminal.")

	case "transcript":
		if msg.ExtractedSemester > 0 || msg.SpecificQuery != "" {
			if m.session.Student.Transcript.TotalCGPA == "" {
				m.chatHistory = append(m.chatHistory, "🔄 Fetching transcript data first...")
				return m, func() tea.Msg {
					err := m.session.GetTranscript(false)
					if err != nil {
						return NLPClassificationMsg{
							Query:             msg.Query,
							Intent:            msg.Intent,
							Error:             fmt.Errorf("failed to fetch transcript"),
							ExtractedSemester: msg.ExtractedSemester,
							SpecificQuery:     msg.SpecificQuery,
						}
					}
					return NLPClassificationMsg{
						Query:             msg.Query,
						Intent:            msg.Intent,
						Confidence:        msg.Confidence,
						ExtractedSemester: msg.ExtractedSemester,
						SpecificQuery:     msg.SpecificQuery,
					}
				}
			}

			transcript := m.session.Student.Transcript
			semesters := parseAndSortSemesters(transcript.Semester)

			var targetSem *SemesterKey
			if msg.ExtractedSemester > 0 {
				if msg.ExtractedSemester <= len(semesters) {
					targetSem = &semesters[msg.ExtractedSemester-1]
				} else {
					m.chatHistory = append(m.chatHistory, fmt.Sprintf("❌ Semester %d not found in your transcript.", msg.ExtractedSemester))
					return m, nil
				}
			}

			if targetSem != nil {
				semData := transcript.Semester[targetSem.semester]
				switch msg.SpecificQuery {
				case "sgpa":
					m.chatHistory = append(m.chatHistory, fmt.Sprintf("📄 Semester %s SGPA: %.2f", targetSem.semester.Name, targetSem.semester.SGPA))
				case "cgpa":
					m.chatHistory = append(m.chatHistory, fmt.Sprintf("📈 Semester %s CGPA: %.2f", targetSem.semester.Name, targetSem.semester.CGPA))
				case "courses":
					m.chatHistory = append(m.chatHistory, fmt.Sprintf("📚 Courses in %s:", targetSem.semester.Name))
					for _, c := range semData {
						m.chatHistory = append(m.chatHistory, fmt.Sprintf("  • %s: %s (%s)", c.Code, c.Title, c.Grade))
					}
				default:
					m.chatHistory = append(m.chatHistory, fmt.Sprintf("📄 %s Summary:", targetSem.semester.Name))
					m.chatHistory = append(m.chatHistory, fmt.Sprintf("  SGPA: %.2f | CGPA: %.2f | Cr. Hrs: %d", targetSem.semester.SGPA, targetSem.semester.CGPA, targetSem.semester.CreditHoursEarned))
				}
				return m, nil
			}
		}

		m.setLoadingState("📄 Getting transcript, please wait", "Fetching your complete academic transcript", "• Esc: Back to chat • Q: Cancel and quit")
		m.currentView = LoadingView
		m.lastView = ChatView
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				err := m.session.GetTranscript(false)
				if err != nil {
					m.session.Student.CgpaEarned = m.session.Student.Transcript.TotalCGPA
					return CourseActionMsg{Action: "transcript", Error: err, Success: false}
				}
				return CourseActionMsg{Action: "transcript", Error: nil, Success: true}
			},
		)

	case "course_details":
		if msg.ExtractedCourse != nil {
			selectedCourse := *msg.ExtractedCourse
			m.chatHistory = append(m.chatHistory, fmt.Sprintf("🎯 Found course: %s", selectedCourse.Code))

			for i, c := range m.courses {
				if c.ID == selectedCourse.ID {
					m.selectedCourse = i
					break
				}
			}
			m.currentView = CourseDetailView
			m.lastView = ChatView
			m.chatHistory = append(m.chatHistory, fmt.Sprintf("📖 Showing details for %s...", selectedCourse.Code))
		} else {
			m.currentView = CoursesView
			m.chatHistory = append(m.chatHistory, fmt.Sprintf("📚 You have %d enrolled courses. Select one to view details.", len(m.courses)))
		}

	case "assessment":
		if msg.ExtractedCourse != nil {
			selectedCourse := *msg.ExtractedCourse
			m.chatHistory = append(m.chatHistory, fmt.Sprintf("🎯 Found course: %s", selectedCourse.Code))
			m.chatHistory = append(m.chatHistory, fmt.Sprintf("🔄 Fetching assessments for %s...", selectedCourse.Code))

			m.setLoadingState(fmt.Sprintf("📝 Getting assessments for %s...", selectedCourse.Code), "Fetching detailed assessment information", "• Esc: Back to chat • Q: Cancel and quit")
			m.currentView = LoadingView
			m.lastView = ChatView
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					err := m.session.GetCourseAssessments(selectedCourse.ID)
					if err != nil {
						return CourseActionMsg{
							Action:   "assessments",
							CourseID: selectedCourse.ID,
							Error:    err,
							Success:  false,
						}
					}
					for i, c := range m.session.Student.Courses {
						if c.ID == selectedCourse.ID {
							m.selectedCourse = i
							break
						}
					}
					return CourseActionMsg{
						Action:         "assessments",
						CourseID:       selectedCourse.ID,
						Error:          nil,
						Success:        true,
						UpdatedCourses: m.session.Student.Courses,
					}
				},
			)
		} else {
			m.chatHistory = append(m.chatHistory, "🔢 Please select a course by number:")
			m.awaitingCourseSelection = true
			m.pendingAction = "assessment"
		}

	default:
		m.chatHistory = append(m.chatHistory, fmt.Sprintf("❓ Unknown intent: %s", msg.Intent))
	}

	return m, nil
}

func (m model) handleChatKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit

	case "esc":
		if m.awaitingCourseSelection {
			m.awaitingCourseSelection = false
			m.pendingAction = ""
			m.chatHistory = append(m.chatHistory, "❌ Selection cancelled")
			return m, nil
		}

		if m.session != nil && m.session.loggedIn {
			m.currentView = CoursesView
		}

	case "enter":
		if m.chatInput == "" {
			return m, nil
		}

		if m.awaitingCourseSelection {
			courseNum, err := strconv.Atoi(strings.TrimSpace(m.chatInput))
			if err != nil || courseNum < 1 || courseNum > len(m.courses) {
				m.chatHistory = append(m.chatHistory, fmt.Sprintf("❌ Invalid selection. Please enter a number between 1 and %d", len(m.courses)))
				m.chatInput = ""
				return m, nil
			}

			selectedCourse := m.courses[courseNum-1]
			m.awaitingCourseSelection = false
			m.chatInput = ""

			if m.pendingAction == "attendance" {
				m.chatHistory = append(m.chatHistory, fmt.Sprintf("🔄 Fetching attendance for %s...", selectedCourse.Code))
				m.setLoadingState(fmt.Sprintf("📊 Getting attendance for %s...", selectedCourse.Code), "Fetching attendance records", "• Esc: Back to chat • Q: Cancel and quit")
				m.currentView = LoadingView
				m.lastView = ChatView
				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						err := m.session.GetCourseAttendance(false, selectedCourse.ID)
						if err != nil {
							return CourseActionMsg{
								Action:   "attendance",
								CourseID: selectedCourse.ID,
								Error:    err,
								Success:  false,
							}
						}
						for i, c := range m.session.Student.Courses {
							if c.ID == selectedCourse.ID {
								m.selectedCourse = i
								break
							}
						}
						return CourseActionMsg{
							Action:         "attendance",
							CourseID:       selectedCourse.ID,
							Error:          nil,
							Success:        true,
							UpdatedCourses: m.session.Student.Courses, // <- Add this
						}
					},
				)
			} else if m.pendingAction == "assessment" {
				m.chatHistory = append(m.chatHistory, fmt.Sprintf("🔄 Fetching assessments for %s...", selectedCourse.Code))
				m.setLoadingState(fmt.Sprintf("📝 Getting assessments for %s...", selectedCourse.Code), "Fetching detailed assessment information", "• Esc: Back to chat • Q: Cancel and quit")
				m.currentView = LoadingView
				m.lastView = ChatView
				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						err := m.session.GetCourseAssessments(selectedCourse.ID)
						if err != nil {
							return CourseActionMsg{
								Action:   "assessments",
								CourseID: selectedCourse.ID,
								Error:    err,
								Success:  false,
							}
						}
						for i, c := range m.session.Student.Courses {
							if c.ID == selectedCourse.ID {
								m.selectedCourse = i
								break
							}
						}
						return CourseActionMsg{
							Action:         "assessments",
							CourseID:       selectedCourse.ID,
							Error:          nil,
							Success:        true,
							UpdatedCourses: m.session.Student.Courses,
						}
					},
				)
			}

			m.pendingAction = ""
			return m, nil
		}

		if m.matcher == nil {
			m.chatHistory = append(m.chatHistory, "❌ Intent matcher not available")
			m.chatInput = ""
			return m, nil
		}

		query := strings.TrimSpace(m.chatInput)
		m.chatInput = ""

		result := m.matcher.Classify(query, m.courses)

		return m, func() tea.Msg {
			return NLPClassificationMsg{
				Query:             query,
				Intent:            result.Intent,
				Confidence:        result.Confidence,
				ExtractedCourse:   result.ExtractedCourse,
				ExtractedSemester: result.ExtractedSemester,
				SpecificQuery:     result.SpecificQuery,
				Error:             nil,
			}
		}

	case "backspace":
		if len(m.chatInput) > 0 {
			m.chatInput = m.chatInput[:len(m.chatInput)-1]
		}

	default:
		if len(msg.String()) == 1 {
			m.chatInput += msg.String()
		}
	}

	return m, nil
}

func (m model) renderChat() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(LIGHT_BLUE).
		MarginBottom(1)

	historyStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BLUE).
		Padding(1, 2).
		Width(min(m.width-4, 90)).
		Height(max(10, m.height-10))

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BLUE).
		Padding(0, 1).
		Width(min(m.width-4, 90))

	helpStyle := lipgloss.NewStyle().
		Foreground(GREY).
		MarginTop(1)

	userMsgStyle := lipgloss.NewStyle().
		Foreground(WHITE).
		Bold(true).
		MarginLeft(2)

	botMsgStyle := lipgloss.NewStyle().
		Foreground(TURQUOISE)

	title := titleStyle.Render("🤖 AI Assistant")

	displayHistory := m.chatHistory
	if len(displayHistory) > 15 {
		displayHistory = displayHistory[len(displayHistory)-15:]
	}

	var historyText string
	if len(displayHistory) == 0 {
		welcomeStyle := lipgloss.NewStyle().Foreground(SILVER).Italic(true)
		historyText = welcomeStyle.Render("👋 Hi! Ask me about your CGPA, grades, courses, attendance, or transcript!\n\nExamples:\n• What's my CGPA?\n• Show me my attendance\n• Check my grades\n• Who are you?")
	} else {
		var styledMessages []string
		for _, msg := range displayHistory {
			if strings.HasPrefix(msg, "🧑 You:") {
				// User message
				styledMessages = append(styledMessages, userMsgStyle.Render(msg))
			} else {
				styledMessages = append(styledMessages, botMsgStyle.Render(msg))
			}
		}
		historyText = strings.Join(styledMessages, "\n\n")

		if m.awaitingCourseSelection && len(m.courses) > 0 {
			historyText += "\n\n"
			courseListStyle := lipgloss.NewStyle().Foreground(YELLOW)
			var courseLines []string
			for i, course := range m.courses {
				line := fmt.Sprintf("%d. %s - %s", i+1, course.Code, course.Title)
				courseLines = append(courseLines, courseListStyle.Render(line))
			}
			historyText += strings.Join(courseLines, "\n")
		}
	}

	history := historyStyle.Render(historyText)

	inputDisplay := m.chatInput + "│"
	input := inputStyle.Render(inputDisplay)

	helpText := helpStyle.Render("• Type your query and press Enter • Esc: Back to courses • Q: Quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		history,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(WHITE).Render("Your query:"),
		input,
		helpText,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
