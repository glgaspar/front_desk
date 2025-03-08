package controller

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"strconv"

	"github.com/glgaspar/front_desk/features/paychecker"
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

func FlipPayChecker(w http.ResponseWriter, r *http.Request) {
	var data = new(paychecker.Bill)
	id, err := strconv.Atoi(r.URL.Query().Get("billId"))
	if err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
		return
	}
	data.Id = id
	if err = data.FlipTrack(); err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
		return
	}

	tmpl.ExecuteTemplate(w, "index.html", nil)
}

func NewPayChecker(w http.ResponseWriter, r *http.Request) {
	var data paychecker.Bill
	body, err := io.ReadAll(r.Body)
	if err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		tmpl.ExecuteTemplate(w, "index.html", err)
	}

}
