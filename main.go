package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/samber/lo"
)

const getExamResultsURLFormat = "https://courses.sportys.com/altitude/api/test-results/%s/EXAM/results?page=%d&size=%d"
const reviewTestURLFormat = "https://courses.sportys.com/altitude/api/test-results/%d/review-questions"
const page = 0
const pageSize = 10
const timeAllowedMins = 120

const defaultImageTemplate = "https://dl.videos.sportys.com/onlinecourse/images/figures/catalog-2h/${type}-${code}_2h.jpg"

type TestResults struct {
	Content []Test `json:"content"`
}

type Test struct {
	ID        int64     `json:"id"`
	BeginTime time.Time `json:"beginTime"`
	EndTime   time.Time `json:"endTime"`
}

type ReviewTestQuestionAnswer struct {
	ID          int64  `json:"id"`
	AnswerText  string `json:"answerText"`
	Correct     bool   `json:"correct"`
	Explanation string `json:"explanation"`
}

type ReviewTestQuestion struct {
	ID            int64                      `json:"id"`
	QuestionText  string                     `json:"questionText"`
	CleanerTable  *string                    `json:"cleanerTable"`
	ImageTemplate *string                    `json:"imageTemplate"`
	Answers       []ReviewTestQuestionAnswer `json:"answers"`
	Correct       bool                       `json:"correct"`
}

type ReviewTestQuestionsResult struct {
	Questions []ReviewTestQuestion `json:"questions"`
}

func main() {
	jwtToken := flag.String("j", "", "JWT token for authentication (required)")
	courseType := flag.String("c", "", "Course type (required)")

	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if strings.TrimSpace(*jwtToken) == "" || strings.TrimSpace(*courseType) == "" {
		fmt.Println("Usage:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(getExamResultsURLFormat, *courseType, page, pageSize),
		nil,
	)
	if err != nil {
		logger.Error("failed to create request", "error", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+*jwtToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("failed to send request", "error", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	testResults := TestResults{}
	if err := json.NewDecoder(resp.Body).Decode(&testResults); err != nil {
		logger.Error("failed to decode test results response", "error", err)
		os.Exit(1)
	}

	seen := map[int64]bool{}
	var dedupedQuestions []ReviewTestQuestion

	for _, test := range testResults.Content {
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf(reviewTestURLFormat, test.ID), strings.NewReader("{}"))
		if err != nil {
			logger.Error("failed to create request", "error", err)
			os.Exit(1)
		}
		req.Header.Set("Authorization", "Bearer "+*jwtToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Error("failed to send request", "error", err)
			os.Exit(1)
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			logger.Error("failed to read review test response body", "error", readErr)
			os.Exit(1)
		}

		var reviewTestQuestionsResult ReviewTestQuestionsResult
		if err := json.Unmarshal(bodyBytes, &reviewTestQuestionsResult); err != nil {
			logger.Error("failed to decode review test questions response", "error", err)
			os.Exit(1)
		}

		for _, q := range reviewTestQuestionsResult.Questions {
			if !seen[q.ID] && !q.Correct {
				seen[q.ID] = true
				dedupedQuestions = append(dedupedQuestions, q)
			}
		}

		timeTakenMins := math.Round(test.EndTime.Sub(test.BeginTime).Minutes())
		logger.Info("Test results",
			"test_id", test.ID,
			"correct", lo.CountBy(reviewTestQuestionsResult.Questions, func(q ReviewTestQuestion) bool { return q.Correct }),
			"incorrect", lo.CountBy(reviewTestQuestionsResult.Questions, func(q ReviewTestQuestion) bool { return !q.Correct }),
			"time_taken_mins", timeTakenMins,
			"time_remaining_mins", timeAllowedMins-timeTakenMins,
		)
	}

	sort.Slice(dedupedQuestions, func(i, j int) bool {
		return dedupedQuestions[i].ID < dedupedQuestions[j].ID
	})

	practiceFile := "questions_practice.pdf"
	withAnswersFile := "questions_with_answers.pdf"

	if err := writePracticePDF(practiceFile, dedupedQuestions, defaultImageTemplate); err != nil {
		logger.Error("failed to write practice PDF", "error", err)
		os.Exit(1)
	}
	logger.Info("Wrote practice PDF", "path", practiceFile)

	if err := writeWithAnswersPDF(withAnswersFile, dedupedQuestions, defaultImageTemplate); err != nil {
		logger.Error("failed to write with-answers PDF", "error", err)
		os.Exit(1)
	}
	logger.Info("Wrote with-answers PDF", "path", withAnswersFile)
}

// Regex to match anchor tags with class containing 'figure' and capture data-figure and data-type attributes.
// Example matches: <a href='#' class='figure' data-figure='71' data-type='figure'>71</a>
var figureRe = regexp.MustCompile(`(?i)<a[^>]*class=['"]?[^'">]*\bfigure\b[^'">]*['"]?[^>]*data-figure=['"]?([^'"\\s>]+)['"]?[^>]*data-type=['"]?([^'"\\s>]+)['"]?[^>]*>.*?</a>`)

// processQuestionForFigures finds the first figure anchor in the question text (if any) and returns:
// - the question text with the anchor replaced by a placeholder like "[Figure 71]",
// - a resolved image URL (substituted into the provided template) or "" if none could be built.
func processQuestionForFigures(questionText string, qTemplate *string, globalTemplate string) (string, string) {
	m := figureRe.FindStringSubmatch(questionText)
	if len(m) == 0 {
		return questionText, ""
	}
	code := m[1]
	typ := m[2]

	// Choose template: question-level overrides global
	template := globalTemplate
	if qTemplate != nil && strings.TrimSpace(*qTemplate) != "" {
		template = *qTemplate
	}
	if strings.TrimSpace(template) == "" {
		return figureRe.ReplaceAllString(questionText, fmt.Sprintf("[Figure %s]", code)), ""
	}

	// Substitute ${type} and ${code}
	imgURL := strings.ReplaceAll(template, "${type}", typ)
	imgURL = strings.ReplaceAll(imgURL, "${code}", code)

	processed := figureRe.ReplaceAllString(questionText, fmt.Sprintf("[Figure %s]", code))
	return processed, imgURL
}

// downloadImageBytes downloads an image and returns its bytes and a gofpdf-compatible image type string ("JPG" or "PNG")
func downloadImageBytes(imgURL string) ([]byte, string, error) {
	resp, err := http.Get(imgURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("bad status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Determine type from Content-Type header or URL extension
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	imgType := ""
	if strings.Contains(ct, "png") {
		imgType = "PNG"
	} else if strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg") {
		imgType = "JPG"
	} else {
		// fallback to extension
		ext := strings.ToLower(path.Ext(imgURL))
		switch ext {
		case ".png":
			imgType = "PNG"
		case ".jpg", ".jpeg":
			imgType = "JPG"
		default:
			// default to JPG
			imgType = "JPG"
		}
	}
	return data, imgType, nil
}

// encodeLatin1 converts a UTF-8 string into a Latin-1 (ISO-8859-1) byte sequence represented as a Go string.
// Characters that can be represented in Latin-1 are output as their single byte; common punctuation substitutions
// are applied for characters outside Latin-1; unknown mappings become '?'.
// This lets gofpdf (which expects 8-bit encoded strings for standard fonts) render characters like '°' properly.
func encodeLatin1(s string) string {
	var b []byte
	for _, r := range s {
		if r <= 0xFF {
			b = append(b, byte(r))
			continue
		}
		// Map a few common useful characters into Latin-1-compatible bytes
		switch r {
		case '–', '—':
			b = append(b, '-') // en/em dash -> hyphen
		case '“', '”':
			b = append(b, '"')
		case '‘', '’', '‚':
			b = append(b, '\'')
		case '…':
			b = append(b, '.', '.', '.')
		// some currency/symbols that exist in Latin-1 but not in lower byte range won't occur here,
		// degree sign U+00B0 is <= 0xFF so it won't hit this branch.
		default:
			// fallback
			b = append(b, '?')
		}
	}
	return string(b)
}

func writePracticePDF(filename string, questions []ReviewTestQuestion, globalImageTemplate string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(encodeLatin1("Practice Questions (No Answers)"), false)
	pdf.SetAuthor(encodeLatin1("sportys-scraper"), false)
	pdf.SetMargins(15, 20, 15)
	pdf.SetAutoPageBreak(true, 20)

	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, encodeLatin1("Practice Questions (Answers Hidden)"), "", 1, "C", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Arial", "", 12)

	for i, question := range questions {
		// Process figures in question text and get image URL if available
		processedQuestion, imgURL := processQuestionForFigures(question.QuestionText, question.ImageTemplate, globalImageTemplate)

		if question.CleanerTable != nil && *question.CleanerTable != "" {
			processedQuestion += "\n\n" + encodeLatin1(*question.CleanerTable)
		}

		// Question header
		qNumStr := fmt.Sprintf("Q%d:", i+1)
		pdf.SetFont("Arial", "B", 12)
		pdf.MultiCell(0, 6, encodeLatin1(qNumStr), "", "L", false)
		pdf.SetFont("Arial", "", 11)
		// Question text
		pdf.MultiCell(0, 6, encodeLatin1(strings.TrimSpace(processedQuestion)), "", "L", false)
		pdf.Ln(2)

		// If we have an image URL, attempt to download and render inline
		if strings.TrimSpace(imgURL) != "" {
			downloaded := false
			if data, imgType, err := downloadImageBytes(imgURL); err == nil {
				// Register image from bytes
				info := pdf.RegisterImageOptionsReader(imgURL, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: false}, bytes.NewReader(data))
				if info != nil {
					// Determine a reasonable display width (max 140mm)
					maxW := 140.0
					origW, origH := info.Extent()
					w := maxW
					h := 0.0
					if origW > 0 {
						h = (origH / origW) * w
					} else {
						// fallback dimensions
						w = 80.0
						h = 60.0
					}
					x := (210.0 - w) / 2.0 // A4 page width 210mm
					// use the imgURL as the link parameter so the image is clickable
					pdf.ImageOptions(imgURL, x, pdf.GetY(), w, h, false, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: false}, 0, imgURL)
					pdf.Ln(h + 2)
					downloaded = true
				}
			}
			// Print the image URL below the figure in small blue font regardless of download success
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(0, 102, 204)
			pdf.MultiCell(0, 5, encodeLatin1(imgURL), "", "L", false)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 11)
			if downloaded {
				pdf.Ln(2)
			} else {
				pdf.Ln(4)
			}
		}

		// Answers labeled A), B), ...
		for ai, a := range question.Answers {
			label := fmt.Sprintf("%s) ", indexToLabel(ai))
			pdf.SetFont("Arial", "", 11)
			pdf.CellFormat(10, 6, encodeLatin1(label), "", 0, "", false, 0, "")
			pdf.MultiCell(0, 6, encodeLatin1(strings.TrimSpace(a.AnswerText)), "", "L", false)
		}
		pdf.Ln(4)

		// Page break if near bottom
		if pdf.GetY() > 260 {
			pdf.AddPage()
		}
	}

	return pdf.OutputFileAndClose(filename)
}

func writeWithAnswersPDF(filename string, questions []ReviewTestQuestion, globalImageTemplate string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(encodeLatin1("Questions with Answers and Explanations"), false)
	pdf.SetAuthor(encodeLatin1("sportys-scraper"), false)
	pdf.SetMargins(15, 20, 15)
	pdf.SetAutoPageBreak(true, 20)

	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, encodeLatin1("Questions with Answers and Explanations"), "", 1, "C", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Arial", "", 12)

	for i, q := range questions {
		// Process figures in question text and get image URL if available
		processedQuestion, imgURL := processQuestionForFigures(q.QuestionText, q.ImageTemplate, globalImageTemplate)

		// Question header
		qNumStr := fmt.Sprintf("Q%d:", i+1)
		pdf.SetFont("Arial", "B", 12)
		pdf.MultiCell(0, 6, encodeLatin1(qNumStr), "", "L", false)
		pdf.SetFont("Arial", "", 11)
		// Question text
		pdf.MultiCell(0, 6, encodeLatin1(strings.TrimSpace(processedQuestion)), "", "L", false)
		pdf.Ln(2)

		// If we have an image URL, attempt to download and render inline
		if strings.TrimSpace(imgURL) != "" {
			downloaded := false
			if data, imgType, err := downloadImageBytes(imgURL); err == nil {
				info := pdf.RegisterImageOptionsReader(imgURL, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: false}, bytes.NewReader(data))
				if info != nil {
					maxW := 140.0
					origW, origH := info.Extent()
					w := maxW
					h := 0.0
					if origW > 0 {
						h = (origH / origW) * w
					} else {
						w = 80.0
						h = 60.0
					}
					x := (210.0 - w) / 2.0
					// use the imgURL as the link parameter so the image is clickable
					pdf.ImageOptions(imgURL, x, pdf.GetY(), w, h, false, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: false}, 0, imgURL)
					pdf.Ln(h + 2)
					downloaded = true
				}
			}
			// Always include the raw image URL below the figure in small blue font
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(0, 102, 204)
			pdf.MultiCell(0, 5, encodeLatin1(imgURL), "", "L", false)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 11)
			if downloaded {
				pdf.Ln(2)
			} else {
				pdf.Ln(4)
			}
		}

		// Answers labeled A), B), ...
		for ai, a := range q.Answers {
			label := fmt.Sprintf("%s) ", indexToLabel(ai))
			if a.Correct {
				// Mark correct answer in bold
				pdf.CellFormat(10, 6, encodeLatin1(label), "", 0, "", false, 0, "")
				pdf.SetFont("Arial", "B", 11)
				pdf.MultiCell(0, 6, encodeLatin1(strings.TrimSpace(a.AnswerText)), "", "L", false)
				pdf.SetFont("Arial", "I", 10)
				if strings.TrimSpace(a.Explanation) != "" {
					expl := fmt.Sprintf("Explanation: %s", a.Explanation)
					pdf.MultiCell(0, 6, encodeLatin1(expl), "", "L", false)
				}
				pdf.SetFont("Arial", "", 11)
			} else {
				pdf.CellFormat(10, 6, encodeLatin1(label), "", 0, "", false, 0, "")
				pdf.MultiCell(0, 6, encodeLatin1(strings.TrimSpace(a.AnswerText)), "", "L", false)
			}
		}
		pdf.Ln(4)

		// Page break if near bottom
		if pdf.GetY() > 260 {
			pdf.AddPage()
		}
	}

	return pdf.OutputFileAndClose(filename)
}

func indexToLabel(i int) string {
	// 0 -> A, 1 -> B, ...
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if i < 0 {
		return "?"
	}
	if i < len(letters) {
		return string(letters[i])
	}
	// If beyond Z, return numeric
	return fmt.Sprintf("%d", i+1)
}
