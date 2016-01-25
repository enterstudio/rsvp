package rsvp

import (
	"fmt"
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

type PersonId uint64

type Person struct {
	Id             PersonId
	Name           string
	Email          string
	IsChild        bool
	ChildBirthDate time.Time
	DietNotes      string
	Notes          string
	ContactInfo    []string
	// avatar?
}

type FamilyId uint64

type Family struct {
	Id         FamilyId
	Name       string
	People     []Person
	AccessCode string
	Notes      string
	// avatar?
}

type FamilyResponse struct {
	Responses map[PersonId]bool
	Notes     string
}

type EventInstance struct {
	Date      time.Time
	Responses map[FamilyId]FamilyResponse
	Notes     string
}

func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, world! URL: %v", r.URL)
}
