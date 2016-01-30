package rsvp

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Person struct {
	Email string
}

type Family struct {
	People []Person
	Token  string
}

type Response struct {
	AttendCount uint8
	Note        string
}

type GuestCap uint8

type EventInstance struct {
	Date  string // YYYY-MM-DD
	Notes string
	Cap   GuestCap
}

func init() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/app/rsvp", rsvp)
	http.HandleFunc("/app/admin/responses", adminResponses)
	http.HandleFunc("/app/admin/schedule", adminSchedule)
	http.HandleFunc("/app/admin/users", adminUsers)
}

func rsvp(w http.ResponseWriter, r *http.Request) {
	/*
		           All requests:
			   family: familyid
			   token: string

		           POST only:
			   date: YYYY-MM-DD string (return next 2 for GET)
			   attending: int
			   note: string
	*/
	r.ParseForm()
	familyId := r.Form.Get("family")
	token := r.Form.Get("token")
	if !familyId || !token {
		http.Error(w, "Missing param family or token.", http.StatusBadRequest)
		return
	}

	familyId, err := strconv.Atoi(familyId)
	if err {
		http.Error(w, "Invalid family ID.", http.StatusBadRequest)
		return
	}

	ctx := appengine.NewContext(r)
	var family Family
	familyKey := datastore.NewKey(ctx, "Family", nil, familyId, nil)
	if err = datastore.Get(ctx, familyKey, &family); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !family || family.Token != token {
		http.Error(w, "Family not found or token invalid.", http.StatusNotFound)
		return
	}

	if r.method == "POST" {
		date := r.Form.Get("date")
		attending := r.Form.Get("attending")
		note := r.Form.Get("note")
		if !date || !attending {
			http.Error(w, "Missing date or attending params.", http.StatusBadRequest)
			return
		}
		attending, err = strconv.Atoi(attending)
		if err != nil {
			http.Error(w, "Invalid attending count.", http.StatusBadRequest)
			return
		}

		var event EventInstance
		eventKey := datastore.NewKey(ctx, "EventInstance", date, nil, nil)
		if err = datastore.Get(ctx, eventKey, &event); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !event {
			http.Error(w, "Event not found.", http.StatusNotFound)
			return
		}

		responseKey := datastore.NewKey(ctx, "Response", familyId, nil, &eventKey)
		existingCount := 0
		var response Response
		if err = datastore.Get(ctx, responseKey, &response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if response {
			existingCount = response.AttendCount
		}

		if attending > existingCount {
			totalCount := 0

			q := datastore.NewQuery("Response").
				Ancestor(&eventKey)
			for t := q.Run(ctx); ; {
				var r Response
				rFam, err := t.Next(&r)
				if err == datastore.Done {
					break
				}
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if rFam != familyId {
					totalCount += r.AttendCount
				}
			}

			if totalCount > event.Cap {
				http.Error(w, "Too many attendees.", http.StatusUnauthorized)
				return
			}
		}

		response.AttendCount = attending
		response.Note = note
		if _, err = datastore.Put(ctx, responseKey, &response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOk)
		w.Write("Thank you.")
	} else {
		// Get this family's responses for those events.
		seattle, err := time.Location.LoadLocation("America/Los_Angeles")
		if err != nil {
			http.Error(w, "Couldn't find Seattle.", http.StatusInternalServerError)
			return
		}
		todayInSeattle = time.Now().In(seattle).Format("2006-01-02")
		q := datastore.NewQuery("EventInstance").
			Filter("Date >=", todayInSeattle).
			Order("Date")
		for t := q.Run(ctx); ; {
			var e EventInstance
			eventKey, err := t.Next(&e)
			if err == datastore.Done {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			k := datastore.NewKey(ctx, "Response", nil, familyId, &eventKey)
			r := new(Response)
			if err := datastore.Get(ctx, k, r); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if r {
				fmt.Fprintf(w, "%v: Found response: %v attending, note: %q\n", e.Date, response.AttendCount, response.Note)
			} else {
				fmt.Fprintf(w, "%v: No response\n", e.Date)
			}
		}
	}
}

func adminResponses(w http.ResponseWriter, r *http.Request) {
	// GET or POST?
}

func adminSchedule(w http.ResponseWriter, r *http.Request) {
	// GET or POST?
	ctx := appengine.NewContext(r)
	// TODO: Initialize a couple of events.
	/**
	k := datastore.NewKey(ctx, "Family", nil, 1, nil)
	f := new(Family)
	f.Token = "cat"
	if _, err := datastore.Put(ctx, k, f); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}*/
}

func adminUsers(w http.ResponseWriter, r *http.Request) {
	// GET or POST?
	ctx := appengine.NewContext(r)
	k := datastore.NewKey(ctx, "Family", nil, 1, nil)
	f := new(Family)
	f.Token = "cat"
	if _, err := datastore.Put(ctx, k, f); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func sendReminders(w http.ResponseWriter, r *http.Request) {
	// Wire to a cron job
	// Send initial reminders
	// Send nag reminders
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, world! URL: %v", r.URL)
}
