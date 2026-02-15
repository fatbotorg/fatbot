package updates

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

func getAppleWatchData(imageBytes []byte) string {
	lines := detectImageText(imageBytes)

	var duration float64
	var elapsedTimeDuration float64
	var otherDuration float64
	var avgHR float64
	var calories float64

	// Regex patterns
	durationRegex := regexp.MustCompile(`(?:(\d{1,2}):)?(\d{1,2}):(\d{2})`)
	hrRegex := regexp.MustCompile(`(\d{2,3})\s*BPM`)
	calRegex := regexp.MustCompile(`(\d+)\s*(?:CAL|KCAL)`)
	anyNumberRegex := regexp.MustCompile(`(\d{2,3})`)
	anyBigNumberRegex := regexp.MustCompile(`(\d{3,4})`)

	// Helper to check neighborhood for keywords
	hasLabelNearby := func(index int, keywords []string) bool {
		start := index - 2
		if start < 0 {
			start = 0
		}
		end := index + 2
		if end >= len(lines) {
			end = len(lines) - 1
		}
		for k := start; k <= end; k++ {
			lower := strings.ToLower(lines[k])
			for _, word := range keywords {
				if strings.Contains(lower, word) {
					return true
				}
			}
		}
		return false
	}

	for i, line := range lines {
		// 1. Duration Parsing
		// Look for time format like 45:00 or 1:05:00
		matches := durationRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			h := 0.0
			m := 0.0
			s := 0.0
			if matches[1] != "" {
				val, _ := strconv.Atoi(matches[1])
				h = float64(val)
			}
			valM, _ := strconv.Atoi(matches[2])
			m = float64(valM)
			valS, _ := strconv.Atoi(matches[3])
			s = float64(valS)
			parsedDuration := h*60 + m + s/60

			if hasLabelNearby(i, []string{"elapsed"}) {
				elapsedTimeDuration = parsedDuration
			} else if hasLabelNearby(i, []string{"workout time", "total time"}) {
				otherDuration = parsedDuration
			}
		}

		// 2. Heart Rate Parsing
		// Priority A: Strict "BPM" match
		hrMatches := hrRegex.FindStringSubmatch(line)
		if len(hrMatches) > 0 {
			val, _ := strconv.ParseFloat(hrMatches[1], 64)
			if val > 40 && val < 220 {
				// If we see "Avg" nearby, it's definitely Avg HR.
				// Even if not, a BPM value is a strong candidate.
				if hasLabelNearby(i, []string{"avg", "average"}) {
					avgHR = val
				} else if avgHR == 0 {
					avgHR = val // Fallback if no label
				}
			}
		} else {
			// Priority B: Number near "Avg Heart Rate" label
			// Only check this if line contains a number and we found the label nearby
			nums := anyNumberRegex.FindAllString(line, -1)
			for _, numStr := range nums {
				val, _ := strconv.ParseFloat(numStr, 64)
				if val > 40 && val < 220 {
					if hasLabelNearby(i, []string{"avg heart", "avg. heart", "average heart", "avg hr"}) {
						avgHR = val
					}
				}
			}
		}

		// 3. Calories Parsing
		calMatches := calRegex.FindStringSubmatch(line)
		if len(calMatches) > 0 {
			val, _ := strconv.ParseFloat(calMatches[1], 64)
			if hasLabelNearby(i, []string{"active", "total", "calories"}) {
				calories = val
			} else if calories == 0 {
				calories = val
			}
		} else {
			// Number near "Calories" label
			nums := anyBigNumberRegex.FindAllString(line, -1)
			for _, numStr := range nums {
				val, _ := strconv.ParseFloat(numStr, 64)
				if val > 10 && val < 10000 {
					if hasLabelNearby(i, []string{"active calories", "total calories"}) {
						calories = val
					}
				}
			}
		}
	}

	// Resolution
	if elapsedTimeDuration > 0 {
		duration = elapsedTimeDuration
	} else {
		duration = otherDuration
	}

	if duration > 0 && avgHR > 0 {
		// Strain Calculation
		maxHR := 190.0
		intensity := avgHR / maxHR
		strainRaw := duration * intensity * math.Exp(1.92*intensity)
		strain := 21.0 * (1.0 - math.Exp(-0.005*strainRaw))
		if strain > 21.0 {
			strain = 21.0
		}

		return fmt.Sprintf("\n\nStrain: %.1f\nCalories: %.0f\nAvg HR: %d\nDuration: %.0f min",
			strain, calories, int(avgHR), duration)
	}

	return ""
}
