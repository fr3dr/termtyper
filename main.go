package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fr3dr/termtyper/db"
	"golang.org/x/term"
)

// ANSI color escape codes
var resetColor = "\033[0m"
var backgroundColor = "\033[90m"
var infoStartColor = "\033[2;92m"
var infoColor = "\033[92m"
var infoDoneColor = "\033[2;33m"
var typedColor = "\033[97m"
var errorColor = "\033[1;4;31m"

// xterm cursor shape escape codes
var defaultCursor = "\033[0 q"
var blockCursor = "\033[1 q"
var underlineCursor = "\033[3 q"
var barCursor = "\033[5 q"

var wordsString string

// TODO: dont update info line while in a typing logic sycle
// TODO: show mistyped chars
// TODO: dont generate words longer than maxLineLength
// TODO: timed mode
// TODO: track more stats like time taken to type character
// TODO: custom word lists, allow users to pipe wordlist file
// TODO: add mode were when you get a character wrong you cant continue until you correct the character
// TODO: multiplayer racing
// TODO: add config file functionality
func main() {
	wordList := DefaultWordList

	// get stats db file
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dbFile := cfgDir + "/termtyper/stats.db"

	// get terminal info
	termHandle := int(os.Stderr.Fd())
	width, _, err := term.GetSize(termHandle)
	if err != nil {
		log.Fatal(err)
	}

	// get flags
	wordListAmmount := flag.Int("n", len(wordList), "ammount of words to use from word list. max: 1000")
	wordsNum := flag.Int("w", 25, "number of words")
	noBackspace := flag.Bool("b", false, "no backspace mode")
	cursorShape := flag.String("c", "", "cursor shape 'bar' 'block' 'underline' leave blank to use default terminal cursor")
	maxLineLength := flag.Int("l", width, "max length each line can be")
	showStats := flag.Bool("s", false, "show stats")
	flag.Parse()

	if *showStats {
		// get stats from database
		results, charStats, err := db.GetAll(dbFile)
		if err != nil {
			log.Fatalf("Failed to get stats: %v", err)
		}

		// print char stats
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
		fmt.Fprintln(w, "char \tcorrect \tincorrect \taccuracy")
		fmt.Fprintln(w, "---- \t------- \t--------- \t--------")
		for _, v := range charStats {
			fmt.Fprintf(w, "%c\t%d\t%d\t%.2f%%\n", v.Char, v.Correct, v.Incorrect, v.Accuracy)
		}
		w.Flush()

		// print general stats
		var ammount float64
		var sumWPM float64
		var sumAccuracy float64
		var sumMistakes float64
		var totalTime time.Duration
		for _, v := range results {
			ammount++
			sumWPM += v.WPM
			sumAccuracy += v.Accuracy
			sumMistakes += float64(v.Mistakes)
			totalTime += time.Duration(v.TimeTaken) * time.Second
		}
		fmt.Printf("Average WPM: %.2f\n", sumWPM/ammount)
		fmt.Printf("Average Accuracy: %.2f%%\n", sumAccuracy/ammount)
		fmt.Printf("Average Mistakes: %.2f\n", sumMistakes/ammount)
		fmt.Printf("Time spent typing: %v\n", totalTime.Round(time.Millisecond))

		return
	}

	// set cursor shape
	switch *cursorShape {
	case "block":
		fmt.Printf("%s", blockCursor)
	case "bar":
		fmt.Printf("%s", barCursor)
	case "underline":
		fmt.Printf("%s", underlineCursor)
	default:
		fmt.Printf("%s", defaultCursor)
	}
	defer fmt.Printf("%s", defaultCursor)

	// generate words
	words := generateWords(*wordsNum, *wordListAmmount, wordList)

	// generate lines
	line := ""
	var lines []string
	for i, word := range words {
		// plus one for the space at the end
		if len(line)+len(word)+1 > *maxLineLength {
			lines = append(lines, line)
			wordsString += line
			line = ""
		}

		line += word
		if i+1 < len(words) {
			line += " "
		} else {
			lines = append(lines, line)
			wordsString += line
		}
	}
	linesNum := len(lines)

	// print words and placeholder info line
	printfColor(infoStartColor, "000wpm  0s  0/%d/0  100%%\n", len(wordsString))
	printfColor(backgroundColor, "%s\r", strings.Join(lines, "\n"))

	// save cursor position at "(0, 0)" and move down to text start
	fmt.Printf("\033[%dA\0337\033[1B", linesNum)

	// put terminal into raw mode
	oldState, err := term.MakeRaw(termHandle)
	if err != nil {
		log.Fatal(err)
	}
	defer term.Restore(termHandle, oldState)

	// typing variables
	firstInput := true
	cursorIndex := 0
	cursorRow := 0
	cursorColumn := 0
	correct := 0
	mistakes := 0
	var typedChars []rune
	var charStats map[rune]db.CharStat = make(map[rune]db.CharStat, 95)
	var start time.Time

	// draw info line
	stopInfo := make(chan bool)
	go func() {
		for range time.Tick(100 * time.Millisecond) {
			select {
			case <-stopInfo:
				return
			default:
				if firstInput {
					continue
				}
				fmt.Printf("\0338\033[2K\r")
				printfColor(infoColor, "%03.0fwpm  %s  %d/%d/%d  %.2f%%", float64(len(typedChars))/5/time.Since(start).Minutes(), time.Since(start).Round(time.Second), correct, len(wordsString), mistakes, float64(correct)/float64(len(typedChars))*100)
				fmt.Printf("\033[%dB\033[%dG", cursorRow+1, cursorColumn+1)
			}
		}
	}()

	// typing logic
	reader := bufio.NewReader(os.Stdin)
	b := make([]byte, 1)
	for {
		reader.Read(b)
		char := rune(b[0])

		// quit on ctrl-c
		if char == 3 {
			stopInfo <- true
			fmt.Printf("\0338")
			for range linesNum + 1 {
				fmt.Printf("\033[2K\033[1B")
			}
			fmt.Printf("\0338\r")
			return
		}

		switch {
		case char == 8 && *noBackspace == false || char == 127 && *noBackspace == false: // backspace
			// dont backspace out of bounds
			if cursorIndex <= 0 {
				break
			}
			cursorIndex--
			cursorColumn--

			// line wrapping
			if cursorColumn < 0 { // line wraps
				cursorRow--
				cursorColumn = len(lines[cursorRow]) - 1
				fmt.Printf("\033[1A\033[%dG", cursorColumn+1)
				printfColor(backgroundColor, "%c", getChar(cursorIndex))
				fmt.Printf("\033[1D")
			} else { // line does not wrap
				fmt.Printf("\033[1D")
				printfColor(backgroundColor, "%c", getChar(cursorIndex))
				fmt.Printf("\033[1D")
			}

			if typedChars[cursorIndex] == getChar(cursorIndex) {
				correct--
			}

			typedChars = typedChars[:len(typedChars)-1]
		case char >= 32 && char <= 126:
			if firstInput {
				start = time.Now()
				firstInput = false
			}

			charStat := charStats[getChar(cursorIndex)]

			if getChar(cursorIndex) == char {
				printfColor(typedColor, "%c", char)
				correct++
				charStat.Correct++
			} else if getChar(cursorIndex) == ' ' {
				printfColor(errorColor, "_")
				charStat.Incorrect++
				mistakes++
			} else {
				printfColor(errorColor, "%c", getChar(cursorIndex))
				charStat.Incorrect++
				mistakes++
			}

			charStats[getChar(cursorIndex)] = charStat

			typedChars = append(typedChars, char)
			cursorIndex++
			cursorColumn++

			// line wrapping
			if cursorColumn >= len(lines[cursorRow]) && cursorIndex < len(wordsString) {
				cursorRow++
				cursorColumn = 0
				fmt.Printf("\033[1B\r")
			}
		}

		// end game
		if cursorIndex == len(wordsString) {
			break
		}
	}

	stopInfo <- true
	term.Restore(termHandle, oldState)

	if len(typedChars) != len(wordsString) {
		log.Fatal("typedChars is not the same length as wordsString")
	}

	// stats
	timeTaken := time.Since(start)
	wpm := float64(correct) / 5 / timeTaken.Minutes()
	accuracy := float64(correct) / float64(len(typedChars)) * 100

	fmt.Printf("\0338\033[2K\r")
	printfColor(infoDoneColor, "%03.0fwpm  %s  %d/%d/%d  %.2f%%", wpm, timeTaken.Round(time.Second), correct, len(typedChars), mistakes, accuracy)
	fmt.Printf("\033[%dB\n", linesNum)

	// save result
	result := db.Result{
		WPM:       wpm,
		Accuracy:  accuracy,
		Correct:   correct,
		Total:     len(typedChars),
		Mistakes:  mistakes,
		TimeTaken: timeTaken.Seconds(),
	}
	err = db.Save(result, charStats, dbFile)
	if err != nil {
		log.Fatalf("Failed to save result: %v", err)
	}
}

func generateWords(ammount int, maxAmmount int, wordList []string) []string {
	var words []string
	for range ammount {
		words = append(words, wordList[rand.IntN(maxAmmount)])
	}
	return words
}

func getChar(index int) rune {
	return []rune(wordsString)[index]
}

func printfColor(colorCode string, format string, a ...any) (n int, err error) {
	return fmt.Fprintf(os.Stdout, colorCode+format+resetColor, a...)
}
