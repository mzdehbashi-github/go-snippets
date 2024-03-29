package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// Regular expression to match wind data in METAR reports
	windRegex = regexp.MustCompile(`\d* METAR.*EGLL \d*Z [A-Z ]*(\d{5}KT|VRB\d{2}KT).*=`)
	// Regular expression to validate TAF reports
	tafValidation = regexp.MustCompile(`.*TAF.*`)
	// Regular expression to match comments
	comment = regexp.MustCompile(`\w*#.*`)
	// Regular expression to identify the end of a METAR report
	metarClose = regexp.MustCompile(`.*=`)
	// Regular expression to identify variable wind
	variableWind = regexp.MustCompile(`.*VRB\d{2}KT`)
	// Regular expression to identify valid wind data
	validWind = regexp.MustCompile(`\d{5}KT`)
	// Regular expression to extract wind direction only
	windDirOnly = regexp.MustCompile(`(\d{3})\d{2}KT`)
	// Array to hold wind direction distribution as follows:
	// windDist[0]: North
	// windDist[1]: Northeast
	// windDist[2]: East
	// windDist[3]: Southeast
	// windDist[4]: South
	// windDist[5]: Southwest
	// windDist[6]: West
	// windDist[7]: Northwest
	// How to read: for example if windDist[0] is equeal to 100,
	//it means, we had 100 times wind comming from North.
	windDist [8]int
)

// parseToArray parses text from a channel and converts METAR reports into arrays.
func parseToArray(textChannel <-chan string, metarChannel chan<- []string) {
	for text := range textChannel {
		lines := strings.Split(text, "\n")
		metarSlice := make([]string, 0, len(lines))
		metarStr := ""
		for _, line := range lines {
			if tafValidation.MatchString(line) {
				break
			}
			if !comment.MatchString(line) {
				metarStr += strings.Trim(line, " ")
			}
			if metarClose.MatchString(line) {
				metarSlice = append(metarSlice, metarStr)
				metarStr = ""
			}
		}
		metarChannel <- metarSlice
	}
}

// extractWindDirection extracts wind direction data from METAR reports.
func extractWindDirection(metarChannel <-chan []string, windsChannel chan<- []string) {
	for metars := range metarChannel {
		winds := make([]string, 0, len(metars))
		for _, metar := range metars {
			if windRegex.MatchString(metar) {
				winds = append(winds, windRegex.FindAllStringSubmatch(metar, -1)[0][1])
			}
		}
		windsChannel <- winds
	}
}

// readFromFile reads content from files and sends it to a text channel.
func readFromFile(absPath string, file os.DirEntry, textChannel chan<- string) {
	dat, err := os.ReadFile(filepath.Join(absPath, file.Name()))
	if err != nil {
		panic(err)
	}
	text := string(dat)
	textChannel <- text
}

// Calculate wind direction distribution, by reading from `windsChannel`
// and updating `windDist` array. Eventually `windDist` would contain wind distributions
// for all directions.
func aggWindDistibution(fileCount int, windsChannel <-chan []string) {
	for i := 0; i < fileCount; i++ {
		winds := <-windsChannel
		for _, wind := range winds {
			if variableWind.MatchString(wind) {
				for i := 0; i < 8; i++ {
					windDist[i]++
				}
			} else if validWind.MatchString(wind) {
				windStr := windDirOnly.FindAllStringSubmatch(wind, -1)[0][1]
				if d, err := strconv.ParseFloat(windStr, 64); err == nil {
					dirIndex := int(math.Round(d/45.0)) % 8
					windDist[dirIndex]++
				}
			}
		}
	}
}

func main() {
	// Get absolute path to the directory containing METAR files
	absPath, _ := filepath.Abs("./metarfiles/")
	// Read the list of files in the directory
	files, _ := os.ReadDir(absPath)
	// Determine the number of files
	fileCount := len(files)
	start := time.Now()

	// Channels for communication between goroutines
	textChannel := make(chan string, fileCount)
	metarChannel := make(chan []string, fileCount)
	windsChannel := make(chan []string, fileCount)

	// Launch goroutines for reading files, parsing METAR reports, and extracting wind direction
	for _, file := range files {
		go readFromFile(absPath, file, textChannel)
		go parseToArray(textChannel, metarChannel)
		go extractWindDirection(metarChannel, windsChannel)
	}

	// Eventually wait for the output from `windsChannel`,
	// to aggregate wind distributions for all directions.
	aggWindDistibution(fileCount, windsChannel)

	elapsed := time.Since(start)

	// Print results and processing time
	fmt.Printf("%v\n", windDist)
	fmt.Printf("Processing took %s\n", elapsed)
}
