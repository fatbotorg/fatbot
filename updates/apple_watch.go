package updates

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

func getAppleWatchData(imageBytes []byte) string {
	lines := detectImageText(imageBytes)
	
	var duration float64
	var avgHR float64
	var calories float64
	var isAppleWatch bool

	// Check for Apple Watch indicators
	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "total time") || 
		   strings.Contains(lowerLine, "active calories") ||
		   strings.Contains(lowerLine, "avg heart rate") ||
		   strings.Contains(lowerLine, "apple watch") {
			isAppleWatch = true
			break
		}
	}

	if !isAppleWatch {
		return ""
	}

	// Regex patterns
	// Duration: matches H:MM:SS or MM:SS or M:SS
	// Examples: "43:36", "1:05:20"
	durationRegex := regexp.MustCompile(`^(\d{0,2}:)?(\d{1,2}):(\d{2})$`)
	// HR: matches "169 BPM" or "169BPM" or just "169" if following label
	hrRegex := regexp.MustCompile(`(\d{2,3})\s*BPM`)
	// Cal: matches "559CAL" or "559KCAl"
	calRegex := regexp.MustCompile(`(\d+)\s*(?:CAL|KCAL)`)
	// Simple number regex
	numberRegex := regexp.MustCompile(`^(\d+)$`)

	for i, line := range lines {
		lowerLine := strings.ToLower(line)

		// Parse Duration
		if strings.Contains(lowerLine, "total time") {
			// Look in current and next 2 lines
			for j := 0; j <= 2 && i+j < len(lines); j++ {
				// Clean the string of "total time" if it's on the same line
				candidate := strings.TrimSpace(strings.ReplaceAll(strings.ToLower(lines[i+j]), "total time", ""))
				matches := durationRegex.FindStringSubmatch(candidate)
				if len(matches) > 0 {
					// matches[0] is full match
					// matches[1] is HH: (optional)
					// matches[2] is MM
					// matches[3] is SS
					
				h := 0.0
				m := 0.0
				s := 0.0

					if matches[1] != "" {
						hStr := strings.TrimSuffix(matches[1], ":")
						hVal, _ := strconv.Atoi(hStr)
						h = float64(hVal)
					}
				mVal, _ := strconv.Atoi(matches[2])
				m = float64(mVal)
				sVal, _ := strconv.Atoi(matches[3])
				s = float64(sVal)

				duration = h*60 + m + s/60
				break
				}
			}
		}

		// Parse Avg Heart Rate
		if strings.Contains(lowerLine, "avg heart rate") || strings.Contains(lowerLine, "avg hr") {
			for j := 0; j <= 2 && i+j < len(lines); j++ {
				candidate := lines[i+j]
				// Check for explicitly marked BPM
				matches := hrRegex.FindStringSubmatch(candidate)
				if len(matches) > 0 {
					val, _ := strconv.ParseFloat(matches[1], 64)
					avgHR = val
					break
				}
				// Check for just a number if on next line
				if j > 0 && numberRegex.MatchString(strings.TrimSpace(candidate)) {
					val, _ := strconv.ParseFloat(strings.TrimSpace(candidate), 64)
					// Sanity check for HR (e.g. 40-220)
					if val > 40 && val < 220 {
						avgHR = val
						break
					}
				}
			}
		}

		// Parse Active Calories
		if strings.Contains(lowerLine, "active calories") {
			for j := 0; j <= 2 && i+j < len(lines); j++ {
				candidate := lines[i+j]
				matches := calRegex.FindStringSubmatch(candidate)
				if len(matches) > 0 {
					val, _ := strconv.ParseFloat(matches[1], 64)
					calories = val
					break
				}
				// Check for just a number
				if j > 0 && numberRegex.MatchString(strings.TrimSpace(candidate)) {
					val, _ := strconv.ParseFloat(strings.TrimSpace(candidate), 64)
					if val > 0 && val < 10000 {
						calories = val
						break
					}
				}
			}
		}
	}

	log.Debugf("Apple Watch Data: Duration=%.2f, AvgHR=%.0f, Calories=%.0f", duration, avgHR, calories)

	if duration > 0 && avgHR > 0 {
		// Strain Calculation
		// Intensity (I): AvgHR / MaxHR
		// Assuming MaxHR of 190 as per requirement example
		maxHR := 190.0
		intensity := avgHR / maxHR
		
		// Formula: Duration (min) * Intensity * e^(1.92 * Intensity)
		strainRaw := duration * intensity * math.Exp(1.92*intensity)

		return fmt.Sprintf("\n\nStrain: %.1f\nCalories: %.0f\nAvg HR: %d\nDuration: %.0f min",
			strainRaw, calories, int(avgHR), duration)
	}

	return ""
}
