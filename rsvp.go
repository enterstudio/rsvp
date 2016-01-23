package rsvp

import (
	"fmt"
	"html/template"
	"net/http"
	"time"
)

/*
Admin:
- Edit users (or maybe just sync w/ spreadsheet / address book?)
- Edit schedule

System:
- RSVP cap
- RSVP reminders

Users:
- Edit profile
- RSVP for next N
-- Bring a dish
- See RSVPs
*/

type Person struct {
	Name           string
	Email          string
	IsChild        bool
	ChildBirthDate time.Time
	DietNotes      string
	Notes          string
	ContactInfo    []string
	// avatar?
}

type Family struct {
	Name       string
	People     []Person
	AccessCode string
	Notes      string
	// avatar?
}

type Head struct {
	Title string
}

type Foot struct {
}

func init() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/editProfile", editProfile)
}

func editProfile(w http.ResponseWriter, r *http.Request) {
	type EditProfile struct {
	}
	body := EditProfile{}

	tmpl, err := template.ParseFiles("head.tmpl")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Template error: %v", err)
		return
	}
	if tmpl.Execute(w, Head{"Edit Profile"}) != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Template execute error: %v", err)
	}

	tmpl, err = template.ParseFiles("editProfile.tmpl")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Template error: %v", err)
		return
	}
	if tmpl.Execute(w, body) != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Template execute error: %v", err)
	}

	tmpl, err = template.ParseFiles("foot.tmpl")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Template error: %v", err)
		return
	}
	if tmpl.Execute(w, Foot{}) != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Template execute error: %v", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}
