package ocr

import (
	"github.com/bbalet/stopwords"
	"github.com/otiai10/gosseract/v2"
	"strings"
)

const ASCII_LETTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVXYZ "

type KeywordExtractor interface {
	Extract() ([]string, error)
}

type ocrExtractor struct {
	imageFiles []string
	client     *gosseract.Client
}

func NewOcrExtractor(imageFiles []string) ocrExtractor {
	client := gosseract.NewClient()
	client.SetLanguage("eng")
	client.SetWhitelist(ASCII_LETTERS)
	return ocrExtractor{
		imageFiles: imageFiles,
		client:     client,
	}
}

func (e ocrExtractor) Extract() ([]string, error) {
	unfiltered := make([]string, 0)
	for _, imagePath := range e.imageFiles {
		err := e.client.SetImage(imagePath)
		if err != nil {
			return nil, err
		}
		text, err := e.client.Text()

		// Replace newlines, split text into words
		words := strings.Split(strings.Replace(text, "\n", "", -1), " ")

		clean := e.removeShortWords(e.removeStopwords(words))

		unfiltered = append(unfiltered, clean...)
	}
	e.client.Close()
	return unfiltered, nil
}

func (e ocrExtractor) removeStopwords(words []string) []string {
	res := make([]string, 0)
	for _, word := range words {
		clean := stopwords.CleanString(word, "en", false)

		if len(clean) > 0 {
			res = append(res, strings.TrimSpace(clean))
		}
	}
	return res
}

func (e ocrExtractor) removeShortWords(words []string) []string {
	res := make([]string, 0)
	for _, word := range words {
		if len(word) > 2 {
			res = append(res, word)
		}
	}
	return res
}
