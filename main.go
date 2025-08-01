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

// TODO: timed mode
// TODO: log stats
// TODO: custom word lists
// TODO: add mode were when you get a character wrong you cant continue until you correct the character
func main() {
	wordList := DefaultWordList

	// get flags
	wordListAmmount := flag.Int("n", len(wordList), "ammount of words to use from word list. max: 1000")
	wordsNum := flag.Int("w", 25, "number of words")
	noBackspace := flag.Bool("b", false, "no backspace mode")
	cursorShape := flag.String("c", "", "cursor shape 'bar' 'block' 'underline' leave blank to use default terminal cursor")
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

	// generate words string
	wordsString := ""
	for i, word := range words {
		wordsString += word

		// only add space if its not the last word
		if i != len(words)-1 {
			wordsString += " "
		}
	}

	// get terminal info
	termHandle := int(os.Stderr.Fd())
	width, _, err := term.GetSize(termHandle)
	if err != nil {
		log.Fatal(err)
	}
	// calculate the number of lines the words will take up
	linesNum := len(wordsString) / width

	// print words and add placeholder info line
	fmt.Println("0s — WPM: 0")
	printfColor(backgroundColor, "%s\r", wordsString)

	// move cursor up if lines wrap
	if linesNum > 0 {
		fmt.Printf("\033[%dA", linesNum)
	}

	// save cursor position at "(0, 0)"
	fmt.Printf("\033[1A\0337\033[1B")

	// put terminal into raw mode
	oldState, err := term.MakeRaw(termHandle)
	if err != nil {
		log.Fatal(err)
	}
	defer term.Restore(termHandle, oldState)

	// setup input reader
	reader := bufio.NewReader(os.Stdin)
	b := make([]byte, 16)

	// typing variables
	firstInput := true
	cursorIndex := 0
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
			printfColor(infoColor, "%s — WPM: %.0f", time.Since(start).Round(time.Second), float64(len(typedChars))/5/time.Since(start).Minutes())

			// move cursor back down
			fmt.Printf("\033[%dB\033[%dG", (cursorIndex/width)+1, (cursorIndex%width)+1)
		}
	}()

	// typing logic
	for {
		// get input
		reader.Read(b)
		char := rune(b[0])

		// quit on ctrl-c
		if char == 3 {
			fmt.Printf("\0338\r")
			return
		}

		styledChar := ""

		switch {
		case char == 8 && *noBackspace == false || char == 127 && *noBackspace == false: // backspace
			// dont backspace out of bounds
			if cursorIndex-1 < 0 {
				break
			}
			cursorIndex -= 1

			// remove char from typed chars
			typedChars = typedChars[:len(typedChars)-1]
			typedCharsStyled = typedCharsStyled[:len(typedCharsStyled)-1]

			// move cursor back and replace letter
			fmt.Printf("\033[1D")
			printfColor(backgroundColor, "%c", getIndexString(cursorIndex, wordsString))
			fmt.Printf("\033[1D")
		case char == ' ': // space
			if firstInput {
				start = time.Now()
				firstInput = false
			}

			if getIndexString(cursorIndex, wordsString) == ' ' {
				styledChar = sprintfColor(typedColor, "·")
			} else {
				styledChar = sprintfColor(errorColor, "%c", getIndexString(cursorIndex, wordsString))
			}

			typedChars = append(typedChars, char)
			typedCharsStyled = append(typedCharsStyled, styledChar)
			cursorIndex += 1
		case char >= 'a' && char <= 'z': // a-z
			if firstInput {
				start = time.Now()
				firstInput = false
			}

			if getIndexString(cursorIndex, wordsString) == char {
				styledChar = sprintfColor(typedColor, "%c", char)
			} else if getIndexString(cursorIndex, wordsString) == ' ' {
				styledChar = sprintfColor(errorColor, "·")
			} else {
				styledChar = sprintfColor(errorColor, "%c", getIndexString(cursorIndex, wordsString))
			}

			typedChars = append(typedChars, char)
			typedCharsStyled = append(typedCharsStyled, styledChar)
			cursorIndex += 1
		}

		fmt.Printf("%s", styledChar)

		// end game
		if cursorIndex == len(wordsString) {
			break
		}
	}

	// turn off raw mode
	term.Restore(termHandle, oldState)

	if len(typedChars) != len([]rune(wordsString)) {
		log.Fatal("slices have different size")
	}

	// clear info line
	fmt.Printf("\0338\033[2K")

	// clear typed text and re print it one line up
	fmt.Printf("\0338\033[1B\033[2K")
	for i := range linesNum {
		fmt.Printf("\0338\033[%dB\033[2K", i+2)
	}
	fmt.Printf("\0338%s\n", strings.Join(typedCharsStyled, ""))
	fmt.Printf("%s\n", string(typedChars))

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

	fmt.Println()

	fmt.Println("-- STATS --")
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
