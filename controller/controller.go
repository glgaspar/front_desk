package controller

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
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
			template.Must(tmpl.ParseGlob("view/pages/*/*.html"))
			template.Must(tmpl.ParseGlob("view/components/*.html"))
		}
	}
}

func FlipPayChecker(w http.ResponseWriter, r *http.Request) {
	log.Println("flipn track")
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

	tmpl.ExecuteTemplate(w, "paycheckerCard", data)
}

func NewPayChecker(w http.ResponseWriter, r *http.Request) {
	var data paychecker.Bill
	var form components.FormData
	body, err := io.ReadAll(r.Body)
	if err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())	
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())
	}
	form.Data = data

	newBill, err := data.CreateBill()
	if err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())	
	}
	form.Data = newBill
	tmpl.ExecuteTemplate(w, "oob-paycheckerCard", form.Data)
	tmpl.ExecuteTemplate(w, "paycheckerAddNewModal", form)
}
