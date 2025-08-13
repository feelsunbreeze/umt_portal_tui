package main

import (
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Attendance struct {
	LectureNumber int
	LectureDate   string
	Attendance    bool
	Faculty       string
}

type Course struct {
	ID           string
	Code         string
	Title        string
	CreditHours  string
	CourseType   string
	FacultyName  string
	FacultyEmail string
	Mode         string
	Section      string
	Semester     string

	Room                 string
	Days                 []string
	StartTime            string
	EndTime              string
	TotalLectures        int
	AttendancePercentage int
	Attendance           []Attendance
}

type TranscriptCourse struct {
	Code        string
	Title       string
	CreditHours int
	Grade       string
	GradePoint  float32
}

type Semester struct {
	Name              string  `json:"name"`
	CreditHoursEarned int     `json:"credit_hours_earned"`
	CGPA              float32 `json:"cgpa"`
	SGPA              float32 `json:"sgpa"`
}

type SemesterKey struct {
	semester Semester
	year     int
	season   int
}

type Transcript struct {
	Semester          map[Semester][]TranscriptCourse `json:"semesters"`
	CreditHoursEarned string                          `json:"credit_hours_earned"`
	CreditHoursForGPA string                          `json:"credit_hours_for_gpa"`
	TotalGradePoints  string                          `json:"total_grade_points"`
	TotalCGPA         string                          `json:"total_cgpa"`
}

type Student struct {
	Name         string
	Batch        string
	ID           string
	Program      string
	ProgramLevel string
	Email        string

	CurrentSemester string
	CgpaEarned      string

	MaxAllowedCreditHours string
	RequestedCreditHours  string
	CompletedCreditHours  string
	RequiredCreditHours   string

	Courses    []Course
	Transcript Transcript
}

type Credentials struct {
	StudentID string
	Password  string
}

type Session struct {
	loggedIn bool
	Student  Student
	Cookies  []*http.Cookie
}

func NewSession() *Session {
	return &Session{}
}

type ErrorCode int

const (
	ErrNone ErrorCode = iota
	ErrInvalidCredentials
	ErrNetworkIssue
	ErrParsingError
)

func decodeFacultyEmail(email string) string {
	data, err := hex.DecodeString(email)
	if err != nil || len(data) < 2 {
		return ""
	}

	key := data[0]

	decoded := make([]byte, len(data)-1)
	for i := 1; i < len(data); i++ {
		decoded[i-1] = data[i] ^ key
	}
	return string(decoded)
}

func SaveCreds(creds Credentials) error {
	dir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	filePath := filepath.Join(dir, "umt_tui", "creds.gob")
	os.MkdirAll(filepath.Dir(filePath), 0700)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	return enc.Encode(creds)
}

func LoadCreds() (Credentials, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return Credentials{}, err
	}
	filePath := filepath.Join(dir, "umt_tui", "creds.gob")

	file, err := os.Open(filePath)
	if err != nil {
		return Credentials{}, err
	}
	defer file.Close()

	var creds Credentials
	dec := gob.NewDecoder(file)
	err = dec.Decode(&creds)
	return creds, err
}

func deleteCreds() error {
	dir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	filePath := filepath.Join(dir, "umt_tui", "creds.gob")
	err = os.Remove(filePath)
	if err != nil {
		return err
	}
	return err
}

func (s *Session) Login(crendetials Credentials, rememberMe bool) (ErrorCode, string) {
	cookies, errorCode, errorString := s.loginAPI(crendetials)
	if errorCode == ErrNone {
		s.Cookies = cookies
		if rememberMe {
			SaveCreds(crendetials)
		}
	}
	return errorCode, errorString
}

func (s *Session) GetStudent() Student {
	return s.Student
}

func getCourseIndex(s *Session, courseId string) int {
	for i, course := range s.Student.Courses {
		if course.ID == courseId {
			return i
		}
	}
	return -1
}

func (s *Session) GetCourses() ([]Course, error) {
	if err := s.fetchUserCourses(); err != nil {
		return nil, err
	}
	return s.Student.Courses, nil
}

func (s *Session) GetCourseAssessments(courseId string) error {
	return s.fetchCourseAssessments(courseId)
}

func (s *Session) GetCourseAttendance(refresh bool, courseId string) error {
	return s.fetchCourseAttendance(refresh, courseId)
}

func (s *Session) GetTranscript(refresh bool) error {
	return s.fetchTranscript(refresh)
}

func saveTranscriptCache(s *Session) error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get user cache dir: %w", err)
	}

	appCacheDir := filepath.Join(cacheDir, "umt_tui")
	if err := os.MkdirAll(appCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create app cache dir: %w", err)
	}

	serializableTranscript := s.Student.Transcript.ToSerializable()

	data, err := json.MarshalIndent(serializableTranscript, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal transcript: %w", err)
	}

	cacheFile := filepath.Join(appCacheDir, "transcript.json")
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

func loadTranscriptCache(s *Session) error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get user cache dir: %w", err)
	}

	cacheFile := filepath.Join(cacheDir, "umt_tui", "transcript.json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	var serializableTranscript SerializableTranscript
	if err := json.Unmarshal(data, &serializableTranscript); err != nil {
		return fmt.Errorf("failed to unmarshal transcript: %w", err)
	}

	s.Student.Transcript = serializableTranscript.ToTranscript()

	return nil
}

func deleteTranscriptCache() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get user cache dir: %w", err)
	}

	cacheFile := filepath.Join(cacheDir, "umt_tui", "transcript.json")
	err = os.Remove(cacheFile)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) IsLoggedIn() bool {
	return len(s.Cookies) > 2
}

func (s *Session) Logout() {
	s.Cookies = nil
	s.Student = Student{}
}

func parseAndSortSemesters(semesterData map[Semester][]TranscriptCourse) []SemesterKey {
	var semesterKeys []SemesterKey
	for sem := range semesterData {
		parts := strings.Fields(sem.Name)
		if len(parts) < 2 {
			continue
		}

		year, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		var season int
		switch strings.ToLower(parts[0]) {
		case "spring":
			season = 1
		case "summer":
			season = 2
		case "fall":
			season = 3
		default:
			continue
		}

		semesterKeys = append(semesterKeys, SemesterKey{
			semester: sem,
			year:     year,
			season:   season,
		})
	}

	sort.Slice(semesterKeys, func(i, j int) bool {
		if semesterKeys[i].year == semesterKeys[j].year {
			return semesterKeys[i].season < semesterKeys[j].season
		}
		return semesterKeys[i].year < semesterKeys[j].year
	})

	return semesterKeys
}

type SerializableTranscript struct {
	Semesters         []SerializableSemester `json:"semesters"`
	CreditHoursEarned string                 `json:"credit_hours_earned"`
	CreditHoursForGPA string                 `json:"credit_hours_for_gpa"`
	TotalGradePoints  string                 `json:"total_grade_points"`
	TotalCGPA         string                 `json:"total_cgpa"`
}

type SerializableSemester struct {
	Name              string             `json:"name"`
	CreditHoursEarned string             `json:"credit_hours_earned"`
	CGPA              string             `json:"cgpa"`
	SGPA              string             `json:"sgpa"`
	Courses           []TranscriptCourse `json:"courses"`
}

func (t *Transcript) ToSerializable() SerializableTranscript {
	var semesters []SerializableSemester
	for semester, courses := range t.Semester {
		serializableSem := SerializableSemester{
			Name:              semester.Name,
			CreditHoursEarned: strconv.Itoa(semester.CreditHoursEarned),
			CGPA:              fmt.Sprintf("%.2f", semester.CGPA),
			SGPA:              fmt.Sprintf("%.2f", semester.SGPA),
			Courses:           courses,
		}
		semesters = append(semesters, serializableSem)
	}
	return SerializableTranscript{
		Semesters:         semesters,
		CreditHoursEarned: t.CreditHoursEarned,
		CreditHoursForGPA: t.CreditHoursForGPA,
		TotalGradePoints:  t.TotalGradePoints,
		TotalCGPA:         t.TotalCGPA,
	}
}

func (st *SerializableTranscript) ToTranscript() Transcript {
	semesterMap := make(map[Semester][]TranscriptCourse)
	for _, serializableSem := range st.Semesters {
		// Parse string values back to appropriate types for Semester struct
		creditHours, _ := strconv.Atoi(serializableSem.CreditHoursEarned)
		cgpa, _ := strconv.ParseFloat(serializableSem.CGPA, 32)
		sgpa, _ := strconv.ParseFloat(serializableSem.SGPA, 32)

		semester := Semester{
			Name:              serializableSem.Name,
			CreditHoursEarned: creditHours,
			CGPA:              float32(cgpa),
			SGPA:              float32(sgpa),
		}
		semesterMap[semester] = serializableSem.Courses
	}
	return Transcript{
		Semester:          semesterMap,
		CreditHoursEarned: st.CreditHoursEarned,
		CreditHoursForGPA: st.CreditHoursForGPA,
		TotalGradePoints:  st.TotalGradePoints,
		TotalCGPA:         st.TotalCGPA,
	}
}
