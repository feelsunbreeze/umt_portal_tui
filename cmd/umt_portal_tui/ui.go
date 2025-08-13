package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	WHITE       = lipgloss.Color("#FFFFFF")
	BLUE        = lipgloss.Color("#0043a8")
	GREY        = lipgloss.Color("#626262")
	LAVENDER    = lipgloss.Color("#B8B8FF")
	GREEN       = lipgloss.Color("#50FA7B")
	LIGHT_GREEN = lipgloss.Color("#B9FBC0")
	PINK        = lipgloss.Color("#FFD1DC")
	RED         = lipgloss.Color("#FF5555")
	YELLOW      = lipgloss.Color("#F1FA8C")
	LIGHT_BLUE  = lipgloss.Color("#8BE9FD")
	TURQUOISE   = lipgloss.Color("#98F5E1")
	SILVER      = lipgloss.Color("#A9B2D8")
)

type ViewType int

const (
	LoginView ViewType = iota
	LoadingView
	ResultView
	CoursesView
	CourseDetailView
	AttendanceView
	TranscriptView
)

type LoginResultMsg struct {
	Code    ErrorCode
	Text    string
	Session *Session
}

type CoursesLoadedMsg struct {
	Courses []Course
	Error   error
}

type CourseActionMsg struct {
	Action   string
	CourseID string
	Error    error
	Success  bool
}

type LoadingState struct {
	Reason     string
	HelpText   string
	BottomText string
}

type model struct {
	width          int
	height         int
	currentView    ViewType
	Credentials    Credentials
	rememberMe     bool
	focusedField   int
	showPassword   bool
	submitted      bool
	loginResult    *LoginResultMsg
	session        *Session
	courses        []Course
	selectedCourse int
	courseError    error
	lastAction     string
	loadingState   LoadingState
	spinner        spinner.Model

	table               []table.Model
	transcriptSemesters []SemesterKey
	currentSemester     int
}

const (
	fieldStudentID = iota
	fieldPassword
	fieldRememberMe
	fieldLoginButton
)

func NewModel() model {
	creds, err := LoadCreds()

	startView := LoginView
	var shouldAutoLogin bool
	if err == nil && creds.StudentID != "" && creds.Password != "" {
		startView = LoadingView
		shouldAutoLogin = true
	}

	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(BLUE)
	s.Spinner = spinner.Points

	return model{
		currentView:    startView,
		Credentials:    creds,
		focusedField:   fieldStudentID,
		selectedCourse: 0,
		rememberMe:     shouldAutoLogin,
		spinner:        s,
		loadingState: LoadingState{
			Reason:     "🔐 Logging in, please wait",
			HelpText:   "Authenticating your cached credentials with the UMT portal",
			BottomText: "• Q: Cancel and quit",
		},
	}
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd

	cmds = append(cmds, m.spinner.Tick)

	if m.currentView == LoadingView && m.Credentials.StudentID != "" && m.Credentials.Password != "" {
		cmds = append(cmds, func() tea.Msg {
			session := NewSession()
			loadTranscriptCache(session)
			code, str := session.Login(m.Credentials, m.rememberMe)
			return LoginResultMsg{Code: code, Text: str, Session: session}
		})
	}

	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case LoginResultMsg:
		m.loginResult = &msg
		m.submitted = false
		if msg.Code == ErrNone {
			m.session = msg.Session
			m.currentView = ResultView
		} else {
			m.currentView = ResultView
		}

	case CoursesLoadedMsg:
		if msg.Error != nil {
			m.courseError = msg.Error
			m.currentView = ResultView
		} else {
			m.courses = msg.Courses
			m.courseError = nil
			m.currentView = CoursesView
		}

	case CourseActionMsg:
		m.lastAction = msg.Action
		if msg.Error != nil {
			m.courseError = msg.Error
			switch msg.Action {
			case "transcript":
				m.currentView = CoursesView
			case "attendance":
				m.currentView = CourseDetailView
			case "assessments":
				m.currentSemester = int(CourseDetailView)
			}
		} else {
			m.courseError = nil
			if msg.Action == "transcript" {
				transcript := m.session.Student.Transcript
				m.setTranscriptTable(transcript)
				m.currentView = TranscriptView
			} else if msg.Action == "attendance" {
				m.currentView = AttendanceView
			} else if msg.Action == "assessments" {
				m.currentView = CourseDetailView
			} else {
				m.currentView = CoursesView
			}
		}

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentView {
	case LoginView:
		return m.handleLoginKeys(msg)
	case LoadingView:
		return m.handleLoadingKeys(msg)
	case ResultView:
		return m.handleResultKeys(msg)
	case CoursesView:
		return m.handleCoursesKeys(msg)
	case CourseDetailView:
		return m.handleCourseDetailKeys(msg)
	case AttendanceView:
		return m.handleAttendanceKeys(msg)
	case TranscriptView:
		return m.handleTranscriptKeys(msg)
	default:
		return m, nil
	}
}

func (m model) handleLoadingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit
	case "esc":
		if strings.Contains(m.loadingState.Reason, "transcript") ||
			strings.Contains(m.loadingState.Reason, "attendance") ||
			strings.Contains(m.loadingState.Reason, "assessments") {
			if m.session != nil && m.session.loggedIn {
				m.currentView = CoursesView
			}
		}
	}
	return m, nil
}

func (m model) handleLoginKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit

	case "esc":
		m.showPassword = !m.showPassword

	case "tab", "down":
		m.focusedField = (m.focusedField + 1) % 4

	case "shift+tab", "up":
		m.focusedField = (m.focusedField - 1 + 4) % 4

	case "enter":
		switch m.focusedField {
		case fieldRememberMe:
			m.rememberMe = !m.rememberMe
		case fieldLoginButton:
			if m.Credentials.StudentID == "" || m.Credentials.Password == "" {
				return m, nil
			}
			m.submitted = true
			m.setLoadingState("🔐 Logging in, please wait", "Authenticating your credentials with the UMT portal", "• Q: Cancel and quit")
			m.currentView = LoadingView

			// Return both spinner tick and login command
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					session := NewSession()
					code, str := session.Login(m.Credentials, m.rememberMe)
					return LoginResultMsg{Code: code, Text: str, Session: session}
				},
			)
		}

	case " ":
		if m.focusedField == fieldRememberMe {
			m.rememberMe = !m.rememberMe
		}

	case "backspace":
		if m.focusedField == fieldStudentID && len(m.Credentials.StudentID) > 0 {
			m.Credentials.StudentID = m.Credentials.StudentID[:len(m.Credentials.StudentID)-1]
		} else if m.focusedField == fieldPassword && len(m.Credentials.Password) > 0 {
			m.Credentials.Password = m.Credentials.Password[:len(m.Credentials.Password)-1]
		}

	default:
		if m.focusedField == fieldStudentID && len(msg.String()) == 1 {
			m.Credentials.StudentID += msg.String()
		} else if m.focusedField == fieldPassword && len(msg.String()) == 1 {
			m.Credentials.Password += msg.String()
		}
	}
	return m, nil
}

func (m model) handleResultKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit
	case "enter", "c":
		if m.loginResult != nil && m.loginResult.Code == ErrNone {
			m.setLoadingState("📚 Loading courses, please wait", "Fetching your enrolled courses from the portal", "• Q: Cancel and quit")
			m.currentView = LoadingView
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					courses, err := m.session.GetCourses()
					return CoursesLoadedMsg{Courses: courses, Error: err}
				},
			)
		}
	case "r":
		m.resetToLogin()
	}
	return m, nil
}

func (m model) handleCoursesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit

	case "up", "k":
		if m.selectedCourse > 0 {
			m.selectedCourse--
		}

	case "down", "j":
		if m.selectedCourse < len(m.courses)-1 {
			m.selectedCourse++
		}

	case "enter":
		if len(m.courses) > 0 {
			m.currentView = CourseDetailView
		}

	case "r":
		m.setLoadingState("🔄 Refreshing courses, please wait", "Updating course information from the portal", "• Esc: Back to courses • Q: Cancel and quit")
		m.currentView = LoadingView
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				courses, err := m.session.GetCourses()
				return CoursesLoadedMsg{Courses: courses, Error: err}
			},
		)

	case "l":
		m.resetToLogin()

	case "t":
		m.setLoadingState("📄 Getting transcript, please wait", "Fetching your complete academic transcript", "• Esc: Back to courses • Q: Cancel and quit")
		m.currentView = LoadingView
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
	}
	return m, nil
}

func (m model) handleCourseDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit
	case "esc", "enter":
		m.currentView = CoursesView
	case "a":
		if len(m.courses) > 0 && m.selectedCourse < len(m.courses) {
			courseID := m.courses[m.selectedCourse].ID
			courseName := m.courses[m.selectedCourse].Code
			m.setLoadingState(fmt.Sprintf("📊 Getting attendance for %s...", courseName), "Fetching attendance records", "• Esc: Back to courses • Q: Cancel and quit")
			m.currentView = LoadingView
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					err := m.session.GetCourseAttendance(false, courseID)
					if err != nil {
						return CourseActionMsg{Action: "attendance", CourseID: courseID, Error: err, Success: false}
					}
					return CourseActionMsg{Action: "attendance", CourseID: courseID, Error: nil, Success: true}
				},
			)
		}
	case "s":
		if len(m.courses) > 0 && m.selectedCourse < len(m.courses) {
			courseID := m.courses[m.selectedCourse].ID
			courseName := m.courses[m.selectedCourse].Code
			m.setLoadingState(fmt.Sprintf("📝 Getting assessments for %s...", courseName), "Fetching detailed assessment information", "• Esc: Back to courses • Q: Cancel and quit")
			m.currentView = LoadingView
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					err := m.session.GetCourseAssessments(courseID)
					if err != nil {
						return CourseActionMsg{Action: "assessments", CourseID: courseID, Error: err, Success: false}
					}
					return CourseActionMsg{Action: "assessments", CourseID: courseID, Error: nil, Success: true}
				},
			)
		}
	}
	return m, nil
}

func (m *model) setLoadingState(reason, helpText, bottomText string) {
	m.loadingState = LoadingState{
		Reason:     reason,
		HelpText:   helpText,
		BottomText: bottomText,
	}
}

func (m *model) resetToLogin() {
	deleteCreds()
	deleteTranscriptCache()
	m.rememberMe = false
	m.currentView = LoginView
	m.loginResult = nil
	m.Credentials.StudentID = ""
	m.Credentials.Password = ""
	m.focusedField = fieldStudentID
	m.courses = nil
	m.selectedCourse = 0
	m.courseError = nil
	m.session = nil
}

func (m model) View() string {
	switch m.currentView {
	case LoginView:
		return m.renderLogin()
	case LoadingView:
		return m.renderLoading()
	case ResultView:
		return m.renderResult()
	case CoursesView:
		return m.renderCourses()
	case CourseDetailView:
		return m.renderCourseDetail()
	case AttendanceView:
		return m.renderAttendance()
	case TranscriptView:
		return m.renderTranscript()
	default:
		return "Unknown view"
	}
}

func (m model) renderLogin() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(LIGHT_BLUE).
		MarginBottom(2)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(WHITE)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(WHITE).
		Padding(0, 1).
		Width(30).
		MarginBottom(1)

	focusedInputStyle := inputStyle.
		BorderForeground(BLUE)

	checkboxStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(WHITE)

	focusedStyle := checkboxStyle.
		Foreground(BLUE)

	buttonStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(WHITE).
		Padding(0, 2).
		Margin(1, 0).
		Border(lipgloss.RoundedBorder())

	focusedButtonStyle := buttonStyle.
		Background(BLUE)

	helpStyle := lipgloss.NewStyle().
		Foreground(GREY)

	title := titleStyle.Render("UMT Portal TUI by Sunbreeze")

	var studentIDInput string
	studentIDValue := m.Credentials.StudentID
	if m.focusedField == fieldStudentID {
		studentIDValue += "│"
		studentIDInput = focusedInputStyle.Render(studentIDValue)
	} else {
		if studentIDValue == "" {
			studentIDValue = "Enter your student ID"
		}
		studentIDInput = inputStyle.Render(studentIDValue)
	}

	studentIDLabel := labelStyle.Render("Student ID:")
	studentIDField := lipgloss.JoinVertical(lipgloss.Left, studentIDLabel, studentIDInput)

	var passwordInput string
	var passwordValue string
	if m.showPassword {
		passwordValue = m.Credentials.Password
	} else {
		passwordValue = strings.Repeat("*", len(m.Credentials.Password))
	}
	if m.focusedField == fieldPassword {
		passwordValue += "│"
		passwordInput = focusedInputStyle.Render(passwordValue)
	} else {
		if len(m.Credentials.Password) == 0 {
			passwordValue = "Enter your password"
		}
		passwordInput = inputStyle.Render(passwordValue)
	}

	passwordLabel := labelStyle.Render("Password:")
	passwordField := lipgloss.JoinVertical(lipgloss.Left, passwordLabel, passwordInput)

	checkboxChar := "○"
	if m.rememberMe {
		checkboxChar = "●"
	}

	var rememberMeField string
	if m.focusedField == fieldRememberMe {
		rememberMeField = focusedStyle.Render(fmt.Sprintf("%s Remember me", checkboxChar))
	} else {
		rememberMeField = checkboxStyle.Render(fmt.Sprintf("%s Remember me", checkboxChar))
	}

	// Login button
	var loginButton string
	if m.focusedField == fieldLoginButton {
		loginButton = focusedButtonStyle.Render("Login")
	} else {
		loginButton = buttonStyle.Render("Login")
	}

	helpText := helpStyle.Render("• ↑/↓: Navigate • Esc: Show password • Enter/Space: Select • Ctrl+C/Q: Quit")

	content := lipgloss.JoinVertical(lipgloss.Center, title, studentIDField, passwordField, rememberMeField, loginButton, "", helpText)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) renderLoading() string {
	reasonStyle := lipgloss.NewStyle().
		Foreground(WHITE).
		Bold(true).
		MarginBottom(1)

	helpStyle := lipgloss.NewStyle().
		Foreground(GREY).
		MarginTop(1)

	quitStyle := lipgloss.NewStyle().
		Foreground(GREY).
		MarginTop(1)

	spinnerView := m.spinner.View()

	content := lipgloss.JoinVertical(lipgloss.Center,
		reasonStyle.Render(m.loadingState.Reason),
		spinnerView,
		helpStyle.Render(m.loadingState.HelpText),
		quitStyle.Render(m.loadingState.BottomText),
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) renderResult() string {
	var statusText string
	var color lipgloss.Color

	if m.courseError != nil {
		color = RED
		statusText = fmt.Sprintf("❌ Error: %s", m.courseError.Error())
	} else if m.loginResult != nil {
		switch m.loginResult.Code {
		case ErrNone:
			color = GREEN
			statusText = "✅ You have successfully logged in to the UMT portal!\n"
			m.session.loggedIn = true
		case ErrNetworkIssue:
			color = RED
			statusText = "🌐 Network issue encountered! Please check your internet.\n"
		case ErrInvalidCredentials:
			color = RED
			statusText = "❌ Invalid credentials! Please check your student ID and password.\n"
		case ErrParsingError:
			color = RED
			statusText = "❓ Error parsing the response! Please try again later.\n"
		default:
			color = RED
			statusText = "❓ An unknown error occurred! Please try again later.\n"
		}
	}

	responseStyle := lipgloss.NewStyle().
		Foreground(color)

	helpStyle := lipgloss.NewStyle().
		Foreground(GREY)

	var helpText string
	if m.loginResult != nil && m.loginResult.Code == ErrNone && m.courseError == nil {
		helpText = helpStyle.Render("• Enter: Continue to courses • R: Retry • Q: Quit")
	} else {
		helpText = helpStyle.Render("• R: Retry • Q: Quit")
	}

	content := lipgloss.JoinVertical(lipgloss.Center, responseStyle.Render(statusText), helpText)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) renderCourses() string {

	headerStyle := lipgloss.NewStyle().
		Bold(true).Foreground(LIGHT_BLUE)

	creditHoursStyle := headerStyle.Foreground(WHITE).UnsetBold()

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(WHITE).
		Background(BLUE).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(SILVER).
		Padding(0, 1)

	helpStyle := lipgloss.NewStyle().
		Foreground(GREY).
		MarginTop(1)

	turquoiseStyle := lipgloss.NewStyle().Foreground(TURQUOISE).Bold(true)
	lavenderStyle := lipgloss.NewStyle().Foreground(LAVENDER).Bold(true)
	lightGreenStyle := lipgloss.NewStyle().Foreground(LIGHT_GREEN).Bold(true)
	pinkStyle := lipgloss.NewStyle().Foreground(PINK).Bold(true)

	student := m.session.GetStudent()
	var studentInfo string
	if m.session != nil {
		studentInfo = fmt.Sprintf("%s, %s | %s | %s: %s",
			headerStyle.Render("Welcome"),
			turquoiseStyle.Render(student.Name),
			lavenderStyle.Render(student.Program),
			headerStyle.Render("CGPA"),
			lightGreenStyle.MarginBottom(1).Render(student.CgpaEarned),
		)
	}

	var creditHoursInfo string
	if m.session != nil {
		creditHoursInfo = fmt.Sprintf(
			"%s %s/%s | %s %s/%s",
			creditHoursStyle.Render("C.Hrs. Registered:"),
			turquoiseStyle.UnsetBold().Render(student.RequestedCreditHours),
			pinkStyle.UnsetBold().Render(student.MaxAllowedCreditHours),
			creditHoursStyle.Render("C.Hrs. Earned:"),
			lightGreenStyle.UnsetBold().Render(student.CompletedCreditHours),
			lavenderStyle.UnsetBold().MarginBottom(1).Render(student.RequiredCreditHours),
		)
	}

	if len(m.courses) == 0 {
		noCoursesStyle := lipgloss.NewStyle().
			Foreground(YELLOW)

		content := lipgloss.JoinVertical(lipgloss.Center,
			studentInfo,
			creditHoursInfo,
			noCoursesStyle.Render("No courses found."),
			helpStyle.Render("• T: Transcript • R: Refresh • L: Log out • Q: Quit"),
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	// Course list
	var courseList []string
	for i, course := range m.courses {
		courseText := fmt.Sprintf("%s - %s (%s CH)", course.Code, course.Title, course.CreditHours)
		if i == m.selectedCourse {
			courseList = append(courseList, selectedStyle.Render(fmt.Sprintf("→ %s", courseText)))
		} else {
			courseList = append(courseList, normalStyle.Render(fmt.Sprintf("  %s", courseText)))
		}
	}

	coursesDisplay := strings.Join(courseList, "\n")

	helpText := helpStyle.Render("• ↑/↓: Navigate • Enter: Details • T: Transcript • R: Refresh • L: Log out • Q: Quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		studentInfo,
		creditHoursInfo,
		coursesDisplay,
		helpText,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) renderCourseDetail() string {
	if len(m.courses) == 0 || m.selectedCourse >= len(m.courses) {
		return m.renderCourses()
	}

	course := m.courses[m.selectedCourse]

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(BLUE).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(LIGHT_BLUE)

	valueStyle := lipgloss.NewStyle().
		Foreground(WHITE)

	helpStyle := lipgloss.NewStyle().
		Foreground(GREY).MarginTop(1)

	title := titleStyle.Render(fmt.Sprintf("📖 Course Details: %s", course.Code))

	details := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("Title:"), valueStyle.Render(course.Title)),
		fmt.Sprintf("%s %s", labelStyle.Render("Credit Hours:"), valueStyle.Render(course.CreditHours)),
		fmt.Sprintf("%s %s", labelStyle.Render("Type:"), valueStyle.Render(course.CourseType)),
		fmt.Sprintf("%s %s", labelStyle.Render("Faculty:"), valueStyle.Render(course.FacultyName)),
		fmt.Sprintf("%s %s", labelStyle.Render("Email:"), valueStyle.Render(course.FacultyEmail)),
		fmt.Sprintf("%s %s", labelStyle.Render("Mode:"), valueStyle.Render(course.Mode)),
		fmt.Sprintf("%s %s", labelStyle.Render("Section:"), valueStyle.Render(course.Section)),
		fmt.Sprintf("%s %s", labelStyle.Render("Semester:"), valueStyle.Render(course.Semester)),
	}

	detailsDisplay := strings.Join(details, "\n")

	helpText := helpStyle.Render("• A: Get Attendance • S: Get Assessments • Esc: Back to courses • Q: Quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		detailsDisplay,
		helpText,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) renderAttendance() string {
	if len(m.courses) == 0 || m.selectedCourse >= len(m.courses) {
		return m.renderCourses()
	}

	course := m.courses[m.selectedCourse]

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(BLUE).
		MarginBottom(1)

	summaryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(LIGHT_BLUE).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(WHITE).
		Background(BLUE).
		Padding(0, 1)

	presentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")) // Green

	absentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000")) // Red

	neutralStyle := lipgloss.NewStyle().
		Foreground(WHITE)

	helpStyle := lipgloss.NewStyle().
		Foreground(GREY).
		MarginTop(1)

	// Title
	title := titleStyle.Render(fmt.Sprintf("📊 Attendance Report: %s", course.Code))

	// Summary
	var summaryColor lipgloss.Color
	switch {
	case course.AttendancePercentage >= 85:
		summaryColor = lipgloss.Color("#00FF00") // Green
	case course.AttendancePercentage >= 75:
		summaryColor = lipgloss.Color("#FFFF00") // Yellow
	default:
		summaryColor = lipgloss.Color("#FF0000") // Red
	}

	summaryText := fmt.Sprintf("Total Lectures: %d | Attendance: %d%%",
		course.TotalLectures, course.AttendancePercentage)
	summary := summaryStyle.Foreground(summaryColor).Render(summaryText)

	// Check if attendance data exists
	if len(course.Attendance) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(GREY).
			MarginTop(2).
			MarginBottom(2)

		noData := noDataStyle.Render("📝 No attendance records available")
		helpText := helpStyle.Render("• Esc/Enter: Back to course details • Q: Quit")

		content := lipgloss.JoinVertical(lipgloss.Center,
			title,
			summary,
			noData,
			helpText,
		)

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	// Table headers
	headers := []string{
		headerStyle.Render("#") + strings.Repeat(" ", 3),
		headerStyle.Render("Date") + strings.Repeat(" ", 3),
		headerStyle.Render("Status") + strings.Repeat(" ", 2),
		headerStyle.Render("Faculty"),
	}

	// Calculate column widths
	lectureWidth := 3
	dateWidth := 12
	statusWidth := 8
	facultyWidth := 15

	// Build table rows
	var rows []string
	rows = append(rows, strings.Join([]string{
		fmt.Sprintf("%-*s", lectureWidth, headers[0]),
		fmt.Sprintf("%-*s", dateWidth, headers[1]),
		fmt.Sprintf("%-*s", statusWidth, headers[2]),
		fmt.Sprintf("%-*s", facultyWidth, headers[3]),
	}, " "))

	// Add separator line
	separator := strings.Repeat("─", lectureWidth+dateWidth+statusWidth+facultyWidth+3)
	rows = append(rows, neutralStyle.Render(separator))

	// Add attendance records
	for _, record := range course.Attendance {
		lectureNum := fmt.Sprintf("%-*d", lectureWidth, record.LectureNumber)
		date := fmt.Sprintf("%-*s", dateWidth, record.LectureDate)

		var status string
		if record.Attendance {
			status = presentStyle.Render(fmt.Sprintf("%-*s", statusWidth, "Present"))
		} else {
			status = absentStyle.Render(fmt.Sprintf("%-*s", statusWidth, "Absent"))
		}

		faculty := neutralStyle.Render(fmt.Sprintf("%-*s", facultyWidth, record.Faculty))

		row := strings.Join([]string{
			neutralStyle.Render(lectureNum),
			neutralStyle.Render(date),
			status,
			faculty,
		}, " ")

		rows = append(rows, row)
	}

	// Create table
	tableStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BLUE).
		Padding(1, 2)

	table := tableStyle.Render(strings.Join(rows, "\n"))

	// Help text
	helpText := helpStyle.Render("• Esc/Enter: Back to course details • Q: Quit")

	// Combine all elements
	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		summary,
		table,
		helpText,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *model) setTranscriptTable(t Transcript) {
	m.transcriptSemesters = parseAndSortSemesters(t.Semester)
	m.table = m.initTranscriptTable(t)
	m.currentSemester = 0
}

func (m model) handleTranscriptKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit
	case "esc":
		m.currentView = CoursesView

	case "r":
		m.setLoadingState("📄 Getting transcript, please wait", "Refreshing your transcript from the portal", "Esc: Back to courses• Q: Cancel and quit")
		m.currentView = LoadingView
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				err := m.session.GetTranscript(true)
				if err != nil {
					m.session.Student.CgpaEarned = m.session.Student.Transcript.TotalCGPA
					return CourseActionMsg{Action: "transcript", Error: err, Success: false}
				}
				return CourseActionMsg{Action: "transcript", Error: nil, Success: true}
			},
		)

	case "left", "h":
		if m.currentSemester > 0 {
			m.currentSemester--
		}
	case "right", "l":
		if m.currentSemester < len(m.transcriptSemesters)-1 {
			m.currentSemester++
		}

	case "up", "k":
		if len(m.table) > m.currentSemester {
			var cmd tea.Cmd
			m.table[m.currentSemester], cmd = m.table[m.currentSemester].Update(msg)
			return m, cmd
		}
	case "down", "j":
		if len(m.table) > m.currentSemester {
			var cmd tea.Cmd
			m.table[m.currentSemester], cmd = m.table[m.currentSemester].Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// Add this new renderTranscript function
func (m model) renderTranscript() string {
	if len(m.table) == 0 || len(m.transcriptSemesters) == 0 {
		errorStyle := lipgloss.NewStyle().Foreground(RED)
		content := errorStyle.Render("No transcript data available")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	if m.currentSemester >= len(m.table) || m.currentSemester >= len(m.transcriptSemesters) {
		m.currentSemester = 0
	}

	// Header with current semester info
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(LIGHT_BLUE).
		MarginBottom(1).
		Align(lipgloss.Center)

	currentSem := m.transcriptSemesters[m.currentSemester].semester
	semesterInfo := fmt.Sprintf("📄 Academic Transcript - %s", currentSem.Name)

	// Semester stats
	statsStyle := lipgloss.NewStyle().
		Foreground(WHITE).
		Align(lipgloss.Center)

	totalStatsStyle := statsStyle.UnsetMarginBottom().MarginTop(1)

	turquoiseStyle := lipgloss.NewStyle().Foreground(TURQUOISE)
	lightGreenStyle := lipgloss.NewStyle().Foreground(LIGHT_GREEN)
	lavenderStyle := lipgloss.NewStyle().Foreground(LAVENDER)
	pinkStyle := lipgloss.NewStyle().Foreground(PINK)

	// Convert int and floats to strings
	creditHoursStr := strconv.Itoa(currentSem.CreditHoursEarned)
	sgpaStr := fmt.Sprintf("%.2f", currentSem.SGPA)
	cgpaStr := fmt.Sprintf("%.2f", currentSem.CGPA)

	stats := fmt.Sprintf("%s %s | %s %s | %s %s",
		statsStyle.Render("C.Hrs. Earned:"),
		turquoiseStyle.Render(creditHoursStr),
		statsStyle.Render("SGPA:"),
		lavenderStyle.Render(sgpaStr),
		statsStyle.Render("CGPA:"),
		lightGreenStyle.MarginBottom(1).Render(cgpaStr),
	)

	totalStats := fmt.Sprintf(
		"%s %s | %s %s | %s %s | %s %s/%s",
		statsStyle.Render("C.Hrs. Earned:"),
		turquoiseStyle.Render(m.session.Student.Transcript.CreditHoursEarned),
		statsStyle.Render("C.Hrs. for GPA:"),
		lavenderStyle.Render(m.session.Student.Transcript.CreditHoursForGPA),
		statsStyle.Render("Total G.P:"),
		turquoiseStyle.Render(m.session.Student.Transcript.TotalGradePoints),
		statsStyle.Render("CGPA:"),
		lightGreenStyle.Render(m.session.Student.Transcript.TotalCGPA),
		pinkStyle.Render("4.00"),
	)

	// Navigation indicator
	navStyle := lipgloss.NewStyle().
		Foreground(GREY).
		MarginBottom(1).
		Align(lipgloss.Center)

	navIndicator := fmt.Sprintf("Semester %d of %d", m.currentSemester+1, len(m.transcriptSemesters))

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(GREY).
		MarginTop(1).
		Align(lipgloss.Center)

	helpText := "• ← →: Switch semesters • ↑ ↓: Navigate • Esc: Back • R: Refresh • Q: Quit"

	// Get current table
	currentTable := m.table[m.currentSemester].View()

	// Combine all elements
	content := lipgloss.JoinVertical(lipgloss.Center,
		headerStyle.Render(semesterInfo),
		statsStyle.Render(stats),
		navStyle.Render(navIndicator),
		currentTable,
		totalStatsStyle.Render(totalStats),
		helpStyle.Render(helpText),
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) handleAttendanceKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.rememberMe {
			deleteTranscriptCache()
		}
		return m, tea.Quit
	case "esc":
		m.currentView = CourseDetailView
	case "r":
		if len(m.courses) > 0 && m.selectedCourse < len(m.courses) {
			courseID := m.courses[m.selectedCourse].ID
			courseName := m.courses[m.selectedCourse].Code
			m.setLoadingState(fmt.Sprintf("📊 Getting attendance for %s...", courseName), "Refreshing attendance record", "• Esc: Back to courses • Q: Cancel and quit")
			m.currentView = LoadingView
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					err := m.session.GetCourseAttendance(true, courseID)
					if err != nil {
						return CourseActionMsg{Action: "attendance", CourseID: courseID, Error: err, Success: false}
					}
					return CourseActionMsg{Action: "attendance", CourseID: courseID, Error: nil, Success: true}
				},
			)
		}
	}

	return m, nil
}

// Update the initTranscriptTable function to be more robust
func (m model) initTranscriptTable(t Transcript) []table.Model {
	var tables []table.Model

	semesterKeys := parseAndSortSemesters(t.Semester)

	columns := []table.Column{
		{Title: "Code", Width: 8},
		{Title: "Course Title", Width: 62},
		{Title: "Cr. Hrs", Width: 7},
		{Title: "Grade", Width: 6},
		{Title: "G.P.", Width: 6},
	}

	for _, sk := range semesterKeys {
		sem := sk.semester
		var rows []table.Row

		courses := t.Semester[sem]
		for _, c := range courses {
			rows = append(rows, table.Row{
				c.Code,
				c.Title,
				fmt.Sprintf("%d", c.CreditHours),
				c.Grade,
				fmt.Sprintf("%.2f", c.GradePoint),
			})
		}

		tableHeight := min(max(len(rows)+1, 5), 15)

		tbl := table.New(
			table.WithColumns(columns),
			table.WithRows(rows),
			table.WithHeight(tableHeight),
			table.WithFocused(true),
		)

		s := table.DefaultStyles()
		s.Header = s.Header.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(BLUE).
			BorderBottom(true).
			Bold(true)
		s.Selected = s.Selected.
			Foreground(WHITE).
			Background(BLUE).
			Bold(true)
		tbl.SetStyles(s)

		tables = append(tables, tbl)
	}

	return tables
}
