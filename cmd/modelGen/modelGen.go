package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

//go:embed modelTemplate.tmpl
var templateModel string

//go:embed daoTemplate.tmpl
var templateDao string

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: modelGen <modelName>")
		os.Exit(1)
	}
	modelName := os.Args[1]
	d := modelInfo{
		NameExported: strings.ToUpper(modelName)[:1] + modelName[1:],
		NamePrivate:  strings.ToLower(modelName)[:1] + modelName[1:],
		NameReceiver: strings.ToLower(modelName)[:1],
	}

	fmt.Println("Generating model...")
	model_file, err := os.OpenFile(fmt.Sprintf("model/%s.go", d.NamePrivate), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	mt, err := template.New("model").Parse(templateModel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = mt.Execute(model_file, d)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Generating dao...")

	dao_file, err := os.OpenFile(fmt.Sprintf("dao/%s.go", d.NamePrivate), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	dt, err := template.New("dao").Parse(templateDao)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = dt.Execute(dao_file, d)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Generating mocks...")
	c := exec.Command("go", "generate", "./...")
	err = c.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = c.Wait()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Done. Don't forget to:\n\t- add the new dao to dao/dao_base.go\n\t- add the new model to cmd/tumlive/tumlive.go (automigrate)")
}

type modelInfo struct {
	NameExported string
	NamePrivate  string
	NameReceiver string
}
