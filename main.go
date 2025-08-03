package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// ANSI color escape codes
var resetColor = "\033[0m"
var backgroundColor = "\033[90m"
var infoColor = "\033[92m"
var typedColor = "\033[1;37m"
var errorColor = "\033[1;31m"

// xterm cursor shape escape codes
var defaultCursor = "\033[0 q"
var blockCursor = "\033[1 q"
var underlineCursor = "\033[3 q"
var barCursor = "\033[5 q"

// TODO: show mistyped chars
// TODO: dont generate words longer than maxLineLength
// TODO: timed mode
// TODO: log stats to database
// TODO: track more stats like character error rate, time taken to type character, most mistyped words
// TODO: custom word lists, allow users to pipe wordlist file
// TODO: add mode were when you get a character wrong you cant continue until you correct the character
func main() {
	wordList := DefaultWordList

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
	flag.Parse()

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
	lines := []string{}
	for i, word := range words {
		// plus one for the space at the end
		if len(line)+len(word)+1 > *maxLineLength {
			lines = append(lines, line)
			line = ""
		}

		line += word
		if i+1 < len(words) {
			line += " "
		} else {
			lines = append(lines, line)
		}
	}
	linesNum := len(lines)
	wordsString := strings.Join(lines, "\n")

	// print words and add placeholder info line
	fmt.Println("0s -- WPM: 0")
	printfColor(backgroundColor, "%s\r", wordsString)

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
	cursorRow := 1
	cursorColumn := 1
	var typedChars []rune
	var typedCharsStyled []string
	var start time.Time

	// draw info line
	go func() {
		for firstInput {
			// wait until first input to update info line
		}
		for range time.Tick(time.Millisecond * 100) {
			// move cursor up to the position of the info line
			fmt.Printf("\0338\033[2K\r")

			// draw timer and wpm
			printfColor(infoColor, "%s -- WPM: %.0f", time.Since(start).Round(time.Second), float64(len(typedChars))/5/time.Since(start).Minutes())

			// move cursor back down
			fmt.Printf("\033[%dB\033[%dG", cursorRow, cursorColumn)
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
			if cursorIndex-1 < 0 {
				break
			}
			cursorIndex--
			cursorColumn--

			// remove char from typed chars
			typedChars = typedChars[:len(typedChars)-1]
			typedCharsStyled = typedCharsStyled[:len(typedCharsStyled)-1]

			// line wrapping
			if getIndexString(cursorIndex, wordsString) == '\n' {
				cursorIndex--
				cursorRow--
				cursorColumn = len(lines[cursorRow-1])
				typedCharsStyled = typedCharsStyled[:len(typedCharsStyled)-1]
				fmt.Printf("\033[1A\033[%dG", cursorColumn)
				printfColor(backgroundColor, "%c", getIndexString(cursorIndex, wordsString))
				fmt.Printf("\033[1D")
			} else {
				// move cursor back and replace letter
				fmt.Printf("\033[1D")
				printfColor(backgroundColor, "%c", getIndexString(cursorIndex, wordsString))
				fmt.Printf("\033[1D")
			}
		case char >= 32 && char <= 126:
			if firstInput {
				start = time.Now()
				firstInput = false
			}

			styledChar := ""

			if getIndexString(cursorIndex, wordsString) == char {
				styledChar = sprintfColor(typedColor, "%c", char)
			} else if getIndexString(cursorIndex, wordsString) == ' ' {
				styledChar = sprintfColor(errorColor, "_")
			} else {
				styledChar = sprintfColor(errorColor, "%c", getIndexString(cursorIndex, wordsString))
			}

			fmt.Printf("%s", styledChar)

			typedChars = append(typedChars, char)
			typedCharsStyled = append(typedCharsStyled, styledChar)
			cursorIndex++
			cursorColumn++

			// line wrapping
			if cursorIndex < len(wordsString) && getIndexString(cursorIndex, wordsString) == '\n' {
				cursorIndex++
				cursorRow++
				cursorColumn = 1
				typedCharsStyled = append(typedCharsStyled, "\n")
				fmt.Printf("\033[1B\r")
			}
		}

		// end game
		if cursorIndex == len(wordsString) {
			break
		}
	}

	// turn off raw mode
	term.Restore(termHandle, oldState)

	// clear everything and re print typed text one line up
	fmt.Printf("\0338")
	for range linesNum + 1 {
		fmt.Printf("\033[2K\033[1B")
	}
	fmt.Printf("\0338%s\n", strings.Join(typedCharsStyled, ""))

	// remove newlines from word string
	wordsString = strings.ReplaceAll(wordsString, "\n", "")
	if len(typedChars) != len([]rune(wordsString)) {
		log.Fatal("slices have different size")
	}

	// get and draw stats
	timeTaken := time.Since(start)
	correct := 0
	incorrect := 0
	for i := range typedChars {
		if typedChars[i] == []rune(wordsString)[i] {
			correct += 1
		} else {
			incorrect += 1
		}
	}

	fmt.Printf("\n-- STATS --\n")
	fmt.Printf("WPM: %.2f\n", float64(correct)/5/timeTaken.Minutes())
	fmt.Printf("Raw WPM: %.2f\n", float64(len(typedChars))/5/timeTaken.Minutes())
	fmt.Printf("Correct: %d/%d %.2f%%\n", correct, len(typedChars), float64(correct)/float64(len(wordsString))*100)
	fmt.Printf("Time taken: %s\n", timeTaken.Round(time.Second))
}

func generateWords(ammount int, maxAmmount int, wordList []string) []string {
	var words []string
	for range ammount {
		words = append(words, wordList[rand.IntN(maxAmmount)])
	}
	return words
}

func getIndexString(index int, s string) rune {
	return []rune(s)[index]
}

func printfColor(colorCode string, format string, a ...any) (n int, err error) {
	return fmt.Fprintf(os.Stdout, colorCode+format+resetColor, a...)
}

func sprintfColor(colorCode string, format string, a ...any) string {
	return fmt.Sprintf(colorCode+format+resetColor, a...)
}
