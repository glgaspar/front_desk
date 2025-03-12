package router

import (
	"html/template"
	"log"
	"net/http"

	"github.com/glgaspar/front_desk/features/root"
	"github.com/glgaspar/front_desk/features/paychecker"
)

var tmpl *template.Template

func init() {
	if tmpl == nil {
		if tmpl == nil {
			tmpl = template.Must(tmpl.ParseGlob("view/layouts/*.html"))
			template.Must(tmpl.ParseGlob("view/components/*.html"))
			template.Must(tmpl.ParseGlob("view/pages/*/*.html"))
		}
	}
}

func Root(w http.ResponseWriter, r *http.Request) {
	log.Println("Fetching apps")
	var data = root.RootConfig{}
	if err := data.Generate(); err != nil {
		tmpl.ExecuteTemplate(w, "error", err)
		return
	}
	err := tmpl.ExecuteTemplate(w, "rootdata", data)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		tmpl.ExecuteTemplate(w, "error", err)
		return
	}
}

func ShowPayChecker(w http.ResponseWriter, r *http.Request) {
	log.Println("Fetching paychecker bills")
	data, err := new(paychecker.Bill).GetAllBills()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		tmpl.ExecuteTemplate(w, "error", err)
		return
	}

	err = tmpl.ExecuteTemplate(w, "paychecker", data)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		tmpl.ExecuteTemplate(w, "error", err)
		return
	}
}

