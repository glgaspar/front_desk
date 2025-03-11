package controller

import (
	"html/template"
	"net/http"
	"strconv"
	"github.com/glgaspar/front_desk/features/components"
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
		tmpl.ExecuteTemplate(w, "errorPopUp", err)
		return
	}
	data.Id = id
	if err = data.FlipTrack(); err != nil {
		tmpl.ExecuteTemplate(w, "errorPopUp", err)
		return
	}

	tmpl.ExecuteTemplate(w, "paycheckerCard", nil)
}

func NewPayChecker(w http.ResponseWriter, r *http.Request) {
	var err error
	var data paychecker.Bill
	form := components.FormData{
		Data: data,
	}

	data.Description = r.Form.Get("description")
	data.Path = r.Form.Get("path")
	data.ExpDay, err = strconv.Atoi(r.Form.Get("expDay"))
	*data.Track = true
	if err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())
	}

	newBill, err := data.CreateBill()
	if err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())	
	}
	form.Data = newBill
	tmpl.ExecuteTemplate(w, "oob-paycheckerCard", form.Data)
	tmpl.ExecuteTemplate(w, "paycheckerAddNewModal", form)
}
