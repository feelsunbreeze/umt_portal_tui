package main

import (
	"strings"
)

type IntentMatcher struct {
	keywords map[string][]string
}

type MatchResult struct {
	Intent            string
	Confidence        float64
	ExtractedCourse   *Course
	ExtractedSemester int
	SpecificQuery     string
}

func NewIntentMatcher() *IntentMatcher {
	return &IntentMatcher{
		keywords: map[string][]string{
			"check_cgpa":     {"cgpa", "gpa", "grade point"},
			"attendance":     {"attendance", "present", "absent", "lectures"},
			"transcript":     {"transcript", "grades", "academic record", "marks", "result", "sgpa", "semester"},
			"course_details": {"courses", "enrolled", "subjects", "classes", "teacher", "faculty", "instructor", "who is", "course details", "tell me about"},
			"assessment":     {"assessment", "assignments", "quizzes", "exam", "test", "asesment", "asessment", "assesment"},
			"greeting":       {"hi", "hello", "hey", "how are you", "greetings", "salam", "aoa"},
			"identity":       {"who are you", "who made you", "creator", "your name", "developer"},
		},
	}
}

func (im *IntentMatcher) Classify(query string, courses []Course) MatchResult {
	query = strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(query)

	matches := make(map[string]float64)

	for intentName, keywords := range im.keywords {
		for _, keyword := range keywords {
			normalizedKeyword := strings.ToLower(keyword)

			if strings.Contains(query, normalizedKeyword) {
				matches[intentName] += 10.0
				continue
			}

			keywordParts := strings.Fields(normalizedKeyword)
			for _, keyPart := range keywordParts {
				for _, queryWord := range queryWords {
					if queryWord == keyPart {
						matches[intentName] += 2.0
						continue
					}

					dist := levenshtein(queryWord, keyPart)
					if dist <= 2 && len(keyPart) > 3 {

						score := 1.0 - (float64(dist) / float64(len(keyPart)))
						matches[intentName] += score
					}
				}
			}
		}
	}

	maxScore := 0.0
	bestIntent := ""

	for intentName, score := range matches {
		if score > maxScore {
			maxScore = score
			bestIntent = intentName
		}
	}

	result := MatchResult{
		Intent:     "unknown",
		Confidence: 0.0,
	}

	if bestIntent != "" && maxScore >= 1.0 {
		result.Intent = bestIntent
		confidence := maxScore / 5.0
		if confidence > 1.0 {
			confidence = 1.0
		}
		result.Confidence = confidence
	}

	semMap := map[string]int{
		"1st": 1, "first": 1, "one": 1, "1": 1,
		"2nd": 2, "second": 2, "two": 2, "2": 2,
		"3rd": 3, "third": 3, "three": 3, "3": 3,
		"4th": 4, "fourth": 4, "four": 4, "4": 4,
		"5th": 5, "fifth": 5, "five": 5, "5": 5,
		"6th": 6, "sixth": 6, "six": 6, "6": 6,
		"7th": 7, "seventh": 7, "seven": 7, "7": 7,
		"8th": 8, "eighth": 8, "eight": 8, "8": 8,
	}

	for word, sem := range semMap {
		if strings.Contains(query, word+" sem") || strings.Contains(query, "sem "+word) ||
			strings.Contains(query, word+" semester") || strings.Contains(query, "semester "+word) {
			result.ExtractedSemester = sem
			if result.Intent == "unknown" {
				result.Intent = "transcript"
				result.Confidence = 0.8
			}
			break
		}
	}

	if result.Intent == "transcript" || result.ExtractedSemester > 0 {
		if strings.Contains(query, "sgpa") {
			result.SpecificQuery = "sgpa"
			result.Intent = "transcript"
		} else if strings.Contains(query, "cgpa") {
			result.SpecificQuery = "cgpa"
			result.Intent = "transcript"
		} else if strings.Contains(query, "courses") || strings.Contains(query, "subjects") {
			result.SpecificQuery = "courses"
			result.Intent = "transcript"
		}
	}

	if len(courses) > 0 {
		var bestCourse *Course
		bestCourseScore := 0.0

		for i := range courses {
			course := &courses[i]
			score := 0.0

			title := strings.ToLower(course.Title)
			code := strings.ToLower(course.Code) // e.g., "cs 201"

			titleWords := strings.Fields(title)

			for _, queryWord := range queryWords {
				if len(queryWord) < 3 && !strings.Contains(code, queryWord) {
					continue
				}

				if strings.Contains(code, queryWord) {
					score += 5.0
				}

				for _, titleWord := range titleWords {
					if queryWord == titleWord {
						score += 3.0
						continue
					}

					if len(titleWord) > 3 {
						dist := levenshtein(queryWord, titleWord)
						if dist <= 2 {
							sim := 1.0 - (float64(dist) / float64(len(titleWord)))
							score += 2.0 * sim
						}
					}
				}
			}

			if strings.Contains(title, query) || strings.Contains(query, " "+title+" ") {
				score += 10.0
			}

			if score > bestCourseScore && score > 2.0 {
				bestCourseScore = score
				bestCourse = course
			}
		}

		result.ExtractedCourse = bestCourse
	}

	return result
}

func levenshtein(s, t string) int {
	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
	}
	for i := range d {
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for j := 1; j <= len(t); j++ {
		for i := 1; i <= len(s); i++ {
			if s[i-1] == t[j-1] {
				d[i][j] = d[i-1][j-1]
			} else {
				min := d[i-1][j]
				if d[i][j-1] < min {
					min = d[i][j-1]
				}
				if d[i-1][j-1] < min {
					min = d[i-1][j-1]
				}
				d[i][j] = min + 1
			}
		}
	}
	return d[len(s)][len(t)]
}
