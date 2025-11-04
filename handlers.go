package main

import (
	"html/template"
	"net/http"
)

var indexTmpl = template.Must(template.ParseFiles("templates/index.html"))
var resultTmpl = template.Must(template.ParseFiles("templates/result.html"))

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = indexTmpl.Execute(w, nil)
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}

	result, err := AnalyzeURL(r.Context(), url)
	if err != nil {
		// surface a friendly error page with status code + description if possible
		status := http.StatusBadGateway
		if ae, ok := err.(*AnalyzeError); ok && ae.StatusCode != 0 {
			status = ae.StatusCode
		}
		w.WriteHeader(status)
		_ = resultTmpl.Execute(w, map[string]any{
			"Error":        err.Error(),
			"URL":          url,
			"HadError":     true,
			"StatusCode":   status,
			"ErrorMessage": err.Error(),
		})
		return
	}

	_ = resultTmpl.Execute(w, map[string]any{
		"URL":          url,
		"HTMLVersion":  result.HTMLVersion,
		"Title":        result.Title,
		"Headings":     result.Headings,
		"Links":        result.Links,
		"HasLoginForm": result.HasLoginForm,
		"HadError":     false,
		"Inaccessible": result.Inaccessible,
	})
}
