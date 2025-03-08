package router

import (
	"front_desk/features/paychecker"
	"front_desk/features/root"
	"html/template"
	"log"
	"net/http"
)

var tmpl *template.Template

func init() {
	if tmpl == nil {
		if tmpl == nil {
			tmpl = template.Must(tmpl.ParseGlob("view/layouts/*.html"))
			template.Must(tmpl.ParseGlob("view/components/*.html"))
		}
	}
}

func Root(w http.ResponseWriter, r *http.Request) {
	log.Println("Fetching apps")
	template.Must(tmpl.ParseGlob("view/pages/root/*.html"))
	var data = root.RootConfig{}
	if err := data.Generate(); err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
		return
	}
	err := tmpl.ExecuteTemplate(w, "index.html", data) 
	if err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
			return
	}
		
}

func ShowPayChecker(w http.ResponseWriter, r *http.Request) {
	log.Println("Fetching paychecker bills")
	template.Must(tmpl.ParseGlob("view/pages/paychecker/*.html"))
	data, err := new(paychecker.Bill).GetAllBills()
	if err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
		return
	}
	if len(data) == 0 {
		tmpl.ExecuteTemplate(w, "index.html", nil)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
		return
	}
}