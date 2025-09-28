package config

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
)

type Config struct {
	// flag and file
	WordCount       int    `json:"word_count"`
	WordListAmmount int    `json:"word_list_ammount"`
	MaxLineLength   int    `json:"max_line_length"`
	TimedMode       int    `json:"timed_mode"`
	NoBackspace     bool   `json:"no_backspace"`
	CorrectOnly     bool   `json:"correct_only"`
	CursorShape     string `json:"cursor_shape"`

	// flag only
	ShowStats bool
}

func GetConfig(defaultConfig Config) (Config, error) {
	config := defaultConfig

	// get config file path
	configFilePath, err := ConfigDirGetFile("config.json")
	if err != nil {
		return defaultConfig, err
	}

	// open and read config file if it exists
	if _, err := os.Stat(configFilePath); !errors.Is(err, os.ErrNotExist) {
		configFile, err := os.Open(configFilePath)
		if err != nil {
			return defaultConfig, err
		}

		b, err := io.ReadAll(configFile)
		if err != nil {
			return defaultConfig, err
		}

		err = json.Unmarshal(b, &config)
		if err != nil {
			return defaultConfig, err
		}
	}

	// get configs from flags
	flag.IntVar(&config.WordCount, "w", config.WordCount, "number of words")
	flag.IntVar(&config.WordListAmmount, "n", config.WordListAmmount, "ammount of words to use from word list. max: 1000")
	flag.IntVar(&config.MaxLineLength, "l", config.MaxLineLength, "max length each line can be")
	flag.IntVar(&config.TimedMode, "t", config.TimedMode, "timed mode ")
	flag.BoolVar(&config.NoBackspace, "b", config.NoBackspace, "no backspace mode")
	flag.BoolVar(&config.CorrectOnly, "o", config.CorrectOnly, "only continue once the correct character is typed")
	flag.BoolVar(&config.ShowStats, "s", config.ShowStats, "show stats")
	flag.StringVar(&config.CursorShape, "c", config.CursorShape, "cursor shape 'bar' 'block' 'underline' leave blank to use default terminal cursor")
	flag.Parse()

	// word count mode has priority over timed mode
	// this fixes -w not working when timed_mode is set in the config file
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "w" {
			config.TimedMode = 0
		}
	})

	return config, nil
}

func ConfigDirGetFile(file string) (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return cfgDir + "/termtyper/" + file, nil
}
