package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"go/ast"
	"go/parser"
	"go/token"
)

// Package constants
var DefaultFilename = "__fart.go"
var GoFileExt = ".go"
var ProtoBufFileExt = ".proto"
var TypeScriptFileExt = ".ts"
var DefaultTargetExt = TypeScriptFileExt

// Custom errors
var ErrStatusBadRequest = errors.New("Invalid path. Expects /<author>/<repository>/<branch>/path/to/file...")
var ErrFileNotFound = errors.New("File not found.")

func removeExt(fragments []string) (string, string) {
	filename := fragments[len(fragments)-1]

	extIndex := strings.LastIndex(filename, ".")
	if extIndex == -1 {
		return strings.Join(fragments, "/"), ""
	}

	ext := filename[extIndex:]
	fragments[len(fragments)-1] = filename[:extIndex]
	return strings.Join(fragments, "/"), ext
}

// An expected request path may be
// "/<author>/<repository>/<branch>/path/to/file" which needs to be
// transformed into a list of possible GitHub URLs.
func parsePath(path string) ([]string, string, error) {
	fragments := strings.Split(path, "/")

	if len(fragments) < 3 {
		return nil, "", ErrStatusBadRequest
	}

	author, repository, branch := fragments[0], fragments[1], fragments[2]
	pathToFile := fragments[3:]

	if len(pathToFile) == 0 {
		pathToFile = []string{DefaultFilename}
	}

	pathWithoutExt, ext := removeExt(pathToFile)
	if len(ext) == 0 {
		ext = DefaultTargetExt
	}

	possibleGitHubURLs := []string{}
	for _, lang := range []string{GoFileExt, ProtoBufFileExt} {
		nextPossibleGitHubURLFragments := []string{"https://github.com", author, repository, branch, "raw", pathWithoutExt + lang}
		nextPossibleGitHubURL := strings.Join(nextPossibleGitHubURLFragments, "/")
		possibleGitHubURLs = append(possibleGitHubURLs, nextPossibleGitHubURL)
	}

	return possibleGitHubURLs, ext, nil
}

func findFirstExistingFile(possibleGitHubURLs []string) (string, string, error) {
	for _, possibleGitHubURL := range possibleGitHubURLs {
		resp, err := http.Get(possibleGitHubURL)

		if err == nil {
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return "", "", err
			}

			return possibleGitHubURL, string(body), nil
		}
	}

	return "", "", ErrFileNotFound
}

func convertGoTypeDefsToTypeScript(src, fileContent string) string {
	// Create the AST by parsing src.
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, src, fileContent, 0)
	if err != nil {
		panic(err)
	}

	result := ""

	// Inspect the AST and print all identifiers and literals.
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			result += "export interface " + x.Name.Name + " {}\n"
		}

		return true
	})

	return result
}

func HandleRequest(w http.ResponseWriter, req *http.Request) {
	pathsOnGitHub, _ /*=srcExt*/, err := parsePath(req.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	targetURL, fileContent, err := findFirstExistingFile(pathsOnGitHub)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// TODO: Add a handler for ProtoBuf target URLs using
	// <https://pkg.go.dev/github.com/emicklei/proto#section-readme>
	if strings.HasSuffix(targetURL, GoFileExt) {
		w.Header().Set("Content-Type", "application/typescript")
		w.Write([]byte(convertGoTypeDefsToTypeScript(targetURL, fileContent)))
	}
}

// Based on <https://github.com/golang/go/blob/6eb58cdffa1ab334493776a25ccccfa89c2ca7ac/src/go/ast/example_test.go#L17>.
func main() {
	http.HandleFunc("/", HandleRequest)
	http.ListenAndServe(":8080", nil)
}
