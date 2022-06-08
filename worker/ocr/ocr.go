package ocr

import (
	"github.com/bbalet/stopwords"
	"github.com/otiai10/gosseract/v2"
	"strings"
)

const ASCII_LETTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVXYZ "

type postprocess []func([]string) []string

type KeywordExtractor interface {
	Extract() ([]string, error)
}

type ocrExtractor struct {
	imageFiles []string
	client     *gosseract.Client
	pipeline   postprocess
}

func Merge(keywords1, keywords2 []string) {

}

func NewOcrExtractor(imageFiles []string, languages []string) KeywordExtractor {
	client := gosseract.NewClient()
	_ = client.SetLanguage(languages...)
	_ = client.SetWhitelist(ASCII_LETTERS)
	return ocrExtractor{
		imageFiles: imageFiles,
		client:     client,
		pipeline: postprocess{
			removeStopwords,
			removeShortWords,
			removeDuplicates,
		},
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
		for _, processor := range e.pipeline {
			words = processor(words)
		}

		unfiltered = append(unfiltered, words...)
	}
	e.client.Close()
	return unfiltered, nil
}

func removeStopwords(words []string) []string {
	res := make([]string, 0)
	for _, word := range words {
		clean := stopwords.CleanString(word, "en", false)

		if len(clean) > 0 {
			res = append(res, strings.TrimSpace(clean))
		}
	}
	return res
}

func removeShortWords(words []string) []string {
	res := make([]string, 0)
	for _, word := range words {
		if len(word) > 2 {
			res = append(res, word)
		}
	}
	return res
}

func removeDuplicates(words []string) []string {
	check := map[string]bool{}
	res := make([]string, 0)
	for _, word := range words {
		if !check[word] {
			check[word] = true
			res = append(res, word)
		}
	}
	return res
}
