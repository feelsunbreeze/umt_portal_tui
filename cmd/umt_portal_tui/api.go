package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const UMT_LOGIN_URL string = "https://online.umt.edu.pk/Account/Login"
const UMT_DATA_URL string = "https://online.umt.edu.pk/CourseRequest"
const UMT_COURSES_URL string = "https://online.umt.edu.pk/MyCourses"
const COURSES_VIEW_ASSESSMENT_URL string = "https://online.umt.edu.pk/MyCourses/ViewAssesments?id="
const COURSES_VIEW_ATTENDANCE_URL string = "https://online.umt.edu.pk/Attendance/ViewAttendance?id="
const COURSES_VIEW_ATTENDANCE_ASPX_URL string = "https://online.umt.edu.pk/Reports/Attendance.aspx"
const COURSES_VIEW_ATTENDANCE_AXD_URL string = "https://online.umt.edu.pk/Reserved.ReportViewerWebControl.axd?"
const TRANSCRIPT_URL string = "https://online.umt.edu.pk/Transcript"
const TRANSCRIPT_ASPX_URL string = "https://online.umt.edu.pk/Reports/Transcript.aspx"

func (s *Session) loginAPI(credentials Credentials) ([]*http.Cookie, ErrorCode, string) {
	if credentials.StudentID == "" || credentials.Password == "" {
		return nil, ErrInvalidCredentials, ""
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	resp, err := client.Get(UMT_LOGIN_URL)
	if err != nil {
		return nil, ErrNetworkIssue, err.Error()
	}
	resp.Body.Close()

	form := url.Values{}
	form.Set("student_id", credentials.StudentID)
	form.Set("Password", credentials.Password)
	form.Set("SecurityCode", "abcde")
	form.Set("SecurityCodeText", "abcde")

	req, err := http.NewRequest("POST", UMT_LOGIN_URL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, ErrNetworkIssue, err.Error()
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(req)
	if err != nil {
		return nil, ErrNetworkIssue, err.Error()
	}
	defer resp.Body.Close()

	u, _ := url.Parse(UMT_LOGIN_URL)
	allCookies := jar.Cookies(u)

	if len(allCookies) < 3 {
		return nil, ErrInvalidCredentials, ""
	}

	s.Student.ID = credentials.StudentID
	s.Student.Email = strings.ToUpper(s.Student.ID) + "@umt.edu.pk"
	s.Cookies = allCookies

	if err := s.fetchUserData(); err != nil {
		return allCookies, ErrParsingError, err.Error()
	}

	return allCookies, ErrNone, ""
}

func (s *Session) fetchUserData() error {
	if len(s.Cookies) == 0 {
		return fmt.Errorf("no cookies found during fetching user data")
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", UMT_DATA_URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create CourseRequest: %w", err)
	}

	for _, cookie := range s.Cookies {
		req.AddCookie(cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get CourseRequest page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse CourseRequest HTML: %w", err)
	}

	doc.Find(".widget-numbers.text-primary").Each(func(i int, sel *goquery.Selection) {
		text := strings.Join(strings.Fields(sel.Text()), " ")
		switch i {
		case 0:
			s.Student.Name = text
		case 1:
			s.Student.Batch = text
		case 2:
			s.Student.RequestedCreditHours = text
		}
	})

	doc.Find(".text-success").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		switch i {
		case 0:
			s.Student.Program = text
		case 1:
			if s.Student.Transcript.TotalCGPA != "" {
				s.Student.CgpaEarned = s.Student.Transcript.TotalCGPA
			} else {
				s.Student.CgpaEarned = text
			}
		case 2:
			s.Student.RequiredCreditHours = text
		}
	})

	doc.Find(".widget-numbers.text-info").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		switch i {
		case 0:
			s.Student.ProgramLevel = text
		case 1:
			if s.Student.Transcript.CreditHoursEarned != "" {
				s.Student.CompletedCreditHours = s.Student.Transcript.CreditHoursEarned
			} else {
				s.Student.CompletedCreditHours = text
			}
		}
	})

	s.Student.CurrentSemester = strings.TrimSpace(doc.Find(".text-warning").First().Text())
	s.Student.MaxAllowedCreditHours = strings.TrimSpace(doc.Find(".widget-numbers.text-danger").First().Text())

	return nil
}

func (s *Session) fetchUserCourses() error {

	if len(s.Cookies) == 0 {
		return fmt.Errorf("no cookies found during fetching user courses")
	}

	s.Student.Courses = nil

	client := &http.Client{}
	req, err := http.NewRequest("GET", UMT_COURSES_URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create courses request: %w", err)
	}

	for _, cookie := range s.Cookies {
		req.AddCookie(cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get courses page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse courses HTML: %w", err)
	}

	doc.Find(".table tr").Each(func(rowIndex int, row *goquery.Selection) {
		if row.Find("th").Length() > 0 {
			return
		}

		var rowData []string
		var assignedID string

		row.Find("td").Each(func(cellIndex int, cell *goquery.Selection) {
			if emailLink := cell.Find("a.__cf_email__"); emailLink.Length() > 0 {
				if encodedEmail, exists := emailLink.Attr("data-cfemail"); exists {
					decodedEmail := decodeFacultyEmail(encodedEmail)
					if decodedEmail != "" {
						rowData = append(rowData, decodedEmail)
					} else {
						rowData = append(rowData, "[email protected]")
					}
				} else {
					rowData = append(rowData, strings.TrimSpace(cell.Text()))
				}
			} else if assignedLink := cell.Find("a.assesment"); assignedLink.Length() > 0 {
				if id, exists := assignedLink.Attr("data-assigned-id"); exists {
					assignedID = id
				}
				rowData = append(rowData, "")
			} else {
				cellText := strings.TrimSpace(cell.Text())
				rowData = append(rowData, cellText)
			}
		})

		if len(rowData) >= 9 {
			course := Course{
				ID:           assignedID,
				Code:         rowData[0],
				Title:        rowData[1],
				CreditHours:  rowData[2],
				CourseType:   rowData[3],
				FacultyName:  rowData[4],
				FacultyEmail: rowData[5],
				Mode:         rowData[6],
				Section:      rowData[7],
				Semester:     rowData[8],
			}
			s.Student.Courses = append(s.Student.Courses, course)
		}
	})

	return nil
}

func (s *Session) fetchCourseAssessments(courseId string) error {
	if len(s.Cookies) == 0 {
		return fmt.Errorf("no cookies found during fetching course assessments")
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", COURSES_VIEW_ASSESSMENT_URL+courseId, nil)
	if err != nil {
		return fmt.Errorf("failed to create assessment request: %w", err)
	}

	for _, cookie := range s.Cookies {
		req.AddCookie(cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get assessment page: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read assessment response: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to parse HTML document: %w", err)
	}

	index := getCourseIndex(s, courseId)
	if index == -1 {
		return fmt.Errorf("course not found")
	}

	course := &s.Student.Courses[index]
	var assessmentRecords []Assessment

	doc.Find("table").Each(func(tableIndex int, table *goquery.Selection) {
		headerRow := table.Find("tr").First()
		hasNameHeader := false
		hasTotalMarksHeader := false
		hasObtainedMarksHeader := false
		hasAssignedDateHeader := false

		headerRow.Find("th").Each(func(i int, th *goquery.Selection) {
			headerText := strings.ToLower(strings.TrimSpace(th.Text()))
			if strings.Contains(headerText, "name") {
				hasNameHeader = true
			}
			if strings.Contains(headerText, "total marks") {
				hasTotalMarksHeader = true
			}
			if strings.Contains(headerText, "obtained marks") {
				hasObtainedMarksHeader = true
			}
			if strings.Contains(headerText, "assigned date") {
				hasAssignedDateHeader = true
			}
		})

		if hasNameHeader && hasTotalMarksHeader && hasObtainedMarksHeader && hasAssignedDateHeader {
			table.Find("tr").Each(func(rowIndex int, row *goquery.Selection) {
				if rowIndex == 0 {
					return
				}

				cells := row.Find("td")
				if cells.Length() >= 4 {
					name := strings.TrimSpace(cells.Eq(0).Text())
					totalMarksStr := strings.TrimSpace(cells.Eq(1).Text())
					obtainedMarksStr := strings.TrimSpace(cells.Eq(2).Text())
					assignedDate := strings.TrimSpace(cells.Eq(3).Text())

					totalMarks := 0.0
					if totalMarksFloat, err := strconv.ParseFloat(totalMarksStr, 64); err == nil {
						totalMarks = totalMarksFloat
					}

					obtainedMarks := 0.0
					if obtainedMarksFloat, err := strconv.ParseFloat(obtainedMarksStr, 64); err == nil {
						obtainedMarks = obtainedMarksFloat
					}

					if name != "" {
						assessmentRecords = append(assessmentRecords, Assessment{
							name:          name,
							obtainedMarks: float32(obtainedMarks),
							totalMarks:    float32(totalMarks),
							assignedDate:  assignedDate,
						})
					}
				}
			})
		}
	})

	course.Assessment = assessmentRecords
	return nil
}

func (s *Session) fetchCourseAttendance(refresh bool, courseId string) error {
	if len(s.Cookies) == 0 {
		return fmt.Errorf("no cookies found during fetching course attendance")
	}

	// Cache
	if !refresh {
		index := getCourseIndex(s, courseId)
		if index == -1 {
			return fmt.Errorf("course not found")
		}
		if len(s.Student.Courses[index].Attendance) > 0 {
			return nil
		}
	}

	maxRetries := 10
	for range maxRetries {
		client := &http.Client{}

		req, err := http.NewRequest("GET", COURSES_VIEW_ATTENDANCE_URL+courseId, nil)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}

		for _, cookie := range s.Cookies {
			req.AddCookie(cookie)
		}

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}
		resp.Body.Close()

		req, err = http.NewRequest("GET", COURSES_VIEW_ATTENDANCE_ASPX_URL, nil)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}

		for _, cookie := range s.Cookies {
			req.AddCookie(cookie)
		}

		resp, err = client.Do(req)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}

		bodyString := string(bodyBytes)
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyString))
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}

		var viewState, viewStateGen, eventValidation string

		doc.Find("input[name='__VIEWSTATE']").Each(func(i int, sel *goquery.Selection) {
			if val, exists := sel.Attr("value"); exists {
				viewState = val
			}
		})

		doc.Find("input[name='__VIEWSTATEGENERATOR']").Each(func(i int, sel *goquery.Selection) {
			if val, exists := sel.Attr("value"); exists {
				viewStateGen = val
			}
		})

		doc.Find("input[name='__EVENTVALIDATION']").Each(func(i int, sel *goquery.Selection) {
			if val, exists := sel.Attr("value"); exists {
				eventValidation = val
			}
		})

		if viewState == "" || viewStateGen == "" || eventValidation == "" {
			time.Sleep(time.Second * 2)
			continue
		}

		data := url.Values{}
		data.Set("__VIEWSTATE", viewState)
		data.Set("__VIEWSTATEGENERATOR", viewStateGen)
		data.Set("__EVENTVALIDATION", eventValidation)
		data.Set("__EVENTTARGET", "Attendance_Report$ctl13$Reserved_AsyncLoadTarget")
		data.Set("__EVENTARGUMENT", "")
		data.Set("Attendance_Report$ctl03$ctl00", "")
		data.Set("Attendance_Report$ctl03$ctl01", "")
		data.Set("Attendance_Report$isReportViewerInVs", "")
		data.Set("Attendance_Report$ctl14", "")
		data.Set("Attendance_Report$ctl15", "standards")
		data.Set("Attendance_Report$AsyncWait$HiddenCancelField", "False")
		data.Set("Attendance_Report$ToggleParam$store", "")
		data.Set("Attendance_Report$ToggleParam$collapse", "false")
		data.Set("Attendance_Report$ctl12$ClientClickedId", "")
		data.Set("Attendance_Report$ctl11$store", "")
		data.Set("Attendance_Report$ctl11$collapse", "false")
		data.Set("Attendance_Report$ctl13$VisibilityState$ctl00", "None")
		data.Set("Attendance_Report$ctl13$ScrollPosition", "")
		data.Set("Attendance_Report$ctl13$ReportControl$ctl02", "")
		data.Set("Attendance_Report$ctl13$ReportControl$ctl03", "")
		data.Set("Attendance_Report$ctl13$ReportControl$ctl04", "100")

		req, err = http.NewRequest("POST", COURSES_VIEW_ATTENDANCE_ASPX_URL, strings.NewReader(data.Encode()))
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Referer", "https://online.umt.edu.pk/Reports/Attendance.aspx")

		for _, cookie := range s.Cookies {
			req.AddCookie(cookie)
		}

		resp, err = client.Do(req)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}
		defer resp.Body.Close()

		finalBodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}

		if len(finalBodyBytes) < 30000 {
			time.Sleep(time.Second * 2)
			continue
		}

		doc, err = goquery.NewDocumentFromReader(strings.NewReader(string(finalBodyBytes)))
		if err != nil {
			continue
		}

		var extractedData []string
		doc.Find("div.canGrowTextBoxInTablix.cannotShrinkTextBoxInTablix").Each(func(i int, s *goquery.Selection) {
			currentText := strings.TrimSpace(s.Text())
			if currentText != "" && !strings.Contains(currentText, "canGrowTextBoxInTablix") {
				extractedData = append(extractedData, currentText)
			}
			sibling := s.Next()
			if sibling.Length() > 0 {
				siblingText := strings.TrimSpace(sibling.Text())
				if siblingText != "" {
					extractedData = append(extractedData, siblingText)
				}
			}
		})

		index := getCourseIndex(s, courseId)
		if index == -1 {
			return fmt.Errorf("course not found")
		} else {
			course := &s.Student.Courses[index]
			if len(extractedData) < 6 {
				course.Attendance = []Attendance{}
			} else {
				var attendanceRecords []Attendance

				startIndex := 4
				endIndex := len(extractedData) - 2

				for i := startIndex; i < endIndex; i += 4 {
					if i+3 >= endIndex {
						break
					}

					lectureNumStr := strings.TrimPrefix(extractedData[i], "Lecture No. ")
					lectureNum, err := strconv.Atoi(lectureNumStr)
					if err != nil {
						continue
					}

					date := extractedData[i+1]
					present := strings.EqualFold(extractedData[i+2], "Present")
					faculty := extractedData[i+3]

					attendanceRecords = append(attendanceRecords, Attendance{
						LectureNumber: lectureNum,
						LectureDate:   date,
						Attendance:    present,
						Faculty:       faculty,
					})
				}

				totalLecturesStr := strings.TrimPrefix(extractedData[len(extractedData)-2], "Total Lectures : ")
				totalLectures, err := strconv.Atoi(totalLecturesStr)
				if err != nil {
					totalLectures = 0
				}

				percentageStr := extractedData[len(extractedData)-1]
				percentageStr = strings.TrimSuffix(percentageStr, " % Attandence")
				percentageStr = strings.TrimSuffix(percentageStr, " % Attendance")
				attendancePercentage, err := strconv.Atoi(strings.TrimSpace(percentageStr))
				if err != nil {
					attendancePercentage = 0
				}

				course.TotalLectures = totalLectures
				course.AttendancePercentage = attendancePercentage
				course.Attendance = attendanceRecords

			}
		}
		return nil
	}
	return fmt.Errorf("failed to fetch attendance after %d attempts", maxRetries)
}

func (s *Session) fetchTranscript(refresh bool) error {
	if !refresh {
		err := loadTranscriptCache(s)
		if err == nil {
			return nil
		}
	}
	if len(s.Cookies) == 0 {
		return fmt.Errorf("no cookies found during fetching user transcript")
	}
	maxRetries := 10
	var lastErr error
	for range maxRetries {
		client := &http.Client{}
		req, err := http.NewRequest("GET", TRANSCRIPT_URL, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}
		for _, cookie := range s.Cookies {
			req.AddCookie(cookie)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to get transcript page: %w", err)
			continue
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}
		if err := os.WriteFile("transcript_initial.txt", bodyBytes, 0644); err != nil {
			lastErr = fmt.Errorf("failed to write initial transcript file: %w", err)
			continue
		}
		req2, err := http.NewRequest("GET", TRANSCRIPT_ASPX_URL, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create transcript ASPX request: %w", err)
			continue
		}
		for _, cookie := range s.Cookies {
			req2.AddCookie(cookie)
		}
		resp2, err := client.Do(req2)
		if err != nil {
			lastErr = fmt.Errorf("failed to get transcript ASPX page: %w", err)
			continue
		}
		bodyBytes2, err := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read transcript ASPX response: %w", err)
			continue
		}
		if len(bodyBytes2) < 30000 {
			lastErr = fmt.Errorf("response too small: %d bytes", len(bodyBytes2))
			continue
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes2)))
		if err != nil {
			lastErr = fmt.Errorf("failed to parse HTML document: %w", err)
			continue
		}

		spans := []string{}
		doc.Find("span").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			spans = append(spans, text)
		})

		if err := parseSpanData(s, spans); err != nil {
			lastErr = fmt.Errorf("failed to parse span data: %w", err)
			continue
		}

		var extractedData []string
		doc.Find("div.canGrowTextBoxInTablix.cannotShrinkTextBoxInTablix").Each(func(i int, s *goquery.Selection) {
			currentText := strings.TrimSpace(s.Text())
			if currentText != "" && !strings.Contains(currentText, "canGrowTextBoxInTablix") {
				extractedData = append(extractedData, currentText)
			}
			sibling := s.Next()
			if sibling.Length() > 0 {
				siblingText := strings.TrimSpace(sibling.Text())
				if siblingText != "" {
					extractedData = append(extractedData, siblingText)
				}
			}
		})

		if len(extractedData) == 0 {
			lastErr = fmt.Errorf("no transcript data found in response")
			continue
		}
		err = parseTranscript(s, extractedData)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse transcript: %w", err)
			continue
		}
		if err := saveTranscriptCache(s); err != nil {
			fmt.Printf("Warning: failed to save transcript cache: %v\n", err)
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("failed to fetch transcript after %d attempts: %w", maxRetries, lastErr)
	}
	return fmt.Errorf("failed to fetch transcript after %d attempts", maxRetries)
}

func parseSpanData(s *Session, spans []string) error {
	for i, span := range spans {
		span = strings.TrimSpace(span)

		if span == "Credit Hours Earned :" && i+1 < len(spans) {
			s.Student.Transcript.CreditHoursEarned = strings.TrimSpace(spans[i+1])
		}

		if span == "Credit Hours for GPA :" && i+1 < len(spans) {
			s.Student.Transcript.CreditHoursForGPA = strings.TrimSpace(spans[i+1])
		}

		if span == "Total Grade Points :" && i+1 < len(spans) {
			s.Student.Transcript.TotalGradePoints = strings.TrimSpace(spans[i+1])
		}

		if span == "CGPA :" && i+1 < len(spans) {
			cgpaValue := strings.TrimSpace(spans[i+1])
			if parts := strings.Split(cgpaValue, " /"); len(parts) > 0 {
				s.Student.Transcript.TotalCGPA = strings.TrimSpace(parts[0])
			} else {
				s.Student.Transcript.TotalCGPA = cgpaValue
			}
		}
	}
	return nil
}

func parseTranscript(s *Session, extractedData []string) error {
	semesterData := make(map[Semester][]TranscriptCourse)
	var currentSemester Semester
	var courses []TranscriptCourse
	var totalCreditHours int
	var totalCreditHoursForGrade int
	var totalGradePoints float32

	isZeroGradePointGrade := func(grade string) bool {
		zeroGrades := []string{"P", "I", "W", "SA", "S", "NC", "F"}
		return slices.Contains(zeroGrades, grade)
	}

	i := 0
	for i < len(extractedData) {
		line := strings.TrimSpace(extractedData[i])

		if line == "Course Code" || line == "Course Title" || line == "Cr. Hrs" || line == "Grade" || line == "G.P." {
			i++
			continue
		}

		if strings.Contains(line, "Fall") || strings.Contains(line, "Spring") || strings.Contains(line, "Summer") {
			if currentSemester.Name != "" && len(courses) > 0 {
				semesterData[currentSemester] = courses
			}

			currentSemester = Semester{Name: line}
			courses = []TranscriptCourse{}
			i++
			continue
		}

		if strings.Contains(line, "Cr. Hrs. Earned:") {
			parts := strings.Split(line, "CGPA:")
			if len(parts) >= 2 {
				creditHoursPart := strings.TrimSpace(strings.Replace(parts[0], "Cr. Hrs. Earned:", "", 1))
				if creditHours, err := strconv.Atoi(creditHoursPart); err == nil {
					currentSemester.CreditHoursEarned = creditHours
					totalCreditHours += creditHours
				}

				cgpaStr := strings.TrimSpace(parts[1])
				if cgpa, err := strconv.ParseFloat(cgpaStr, 32); err == nil {
					currentSemester.CGPA = float32(cgpa)
				}
			}
			i++
			continue
		}

		if strings.Contains(line, "SGPA:") {
			sgpaStr := strings.TrimSpace(strings.Replace(line, "SGPA:", "", 1))
			if sgpa, err := strconv.ParseFloat(sgpaStr, 32); err == nil {
				currentSemester.SGPA = float32(sgpa)
			}
			i++
			continue
		}

		if i+3 < len(extractedData) {
			code := strings.TrimSpace(line)
			title := strings.TrimSpace(extractedData[i+1])
			creditHoursStr := strings.TrimSpace(extractedData[i+2])
			grade := strings.TrimSpace(extractedData[i+3])

			if creditHours, err := strconv.Atoi(creditHoursStr); err == nil {
				var gradePoint float32
				fieldsToSkip := 4 // code, title, credit hours, grade

				if isZeroGradePointGrade(grade) {
					gradePoint = 0.0
					if grade == "F" && !strings.Contains(title, "[R]") {
						totalCreditHoursForGrade += creditHours
					}
				} else {
					if i+4 < len(extractedData) {
						gradePointStr := strings.TrimSpace(extractedData[i+4])

						if gp, err := strconv.ParseFloat(gradePointStr, 32); err == nil &&
							!strings.Contains(gradePointStr, "Cr. Hrs. Earned:") &&
							!strings.Contains(gradePointStr, "Fall") &&
							!strings.Contains(gradePointStr, "Spring") &&
							!strings.Contains(gradePointStr, "Summer") &&
							!strings.Contains(gradePointStr, "Course Code") {
							gradePoint = float32(gp)
							if gradePoint != 0 {
								totalCreditHoursForGrade += creditHours
							}
							fieldsToSkip = 5
						} else {
							gradePoint = 0.0
							fieldsToSkip = 4
						}
					} else {
						gradePoint = 0.0
					}
				}

				course := TranscriptCourse{
					Code:        code,
					Title:       title,
					CreditHours: creditHours,
					Grade:       grade,
					GradePoint:  gradePoint,
				}
				courses = append(courses, course)

				if !isZeroGradePointGrade(grade) && gradePoint > 0 {
					totalGradePoints += gradePoint
				}

				i += fieldsToSkip
				continue
			}
		}
		i++
	}

	if currentSemester.Name != "" && len(courses) > 0 {
		semesterData[currentSemester] = courses
	}

	semesterKeys := parseAndSortSemesters(semesterData)

	s.Student.Transcript.Semester = make(map[Semester][]TranscriptCourse)
	for _, key := range semesterKeys {
		s.Student.Transcript.Semester[key.semester] = semesterData[key.semester]
	}

	return nil
}
