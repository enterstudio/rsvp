package rsvp

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

type Person struct {
	Email string
}

type Family struct {
	// int key: family ID
	People []Person
	Token  string
}

type Response struct {
	// int key: family ID
	AttendCount GuestCap
	Note        string
}

type GuestCap int8

type EventInstance struct {
	// string key: date
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

func logError(w http.ResponseWriter, ctx context.Context, s string, status int) {
	log.Errorf(ctx, "%v", s)
	buf := make([]byte, 65536)
	runtime.Stack(buf, false)
	log.Errorf(ctx, "%v", string(buf))
	http.Error(w, s, status)
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
	ctx := appengine.NewContext(r)
	r.ParseForm()

	familyIdStr := r.Form.Get("family")
	token := r.Form.Get("token")
	if familyIdStr == "" || token == "" {
		logError(w, ctx, "Missing param family or token.", http.StatusBadRequest)
		return
	}

	familyId, err := strconv.ParseInt(familyIdStr, 10, 64)
	if err != nil {
		logError(w, ctx, "Invalid family ID.", http.StatusBadRequest)
		return
	}

	family := new(Family)
	familyKey := datastore.NewKey(ctx, "Family", "", familyId, nil)
	err = datastore.Get(ctx, familyKey, family)
	if err != nil && err != datastore.ErrNoSuchEntity {
		logError(w, ctx, err.Error(), http.StatusInternalServerError)
		return
	}
	if err == datastore.ErrNoSuchEntity || family.Token != token {
		logError(w, ctx, "Family not found or token invalid.", http.StatusNotFound)
		return
	}

	if r.Method == "POST" {
		date := r.Form.Get("date")
		attendingStr := r.Form.Get("attending")
		note := r.Form.Get("note")
		if date == "" || attendingStr == "" {
			logError(w, ctx, "Missing date or attending params.", http.StatusBadRequest)
			return
		}

		i, err := strconv.ParseInt(attendingStr, 10, 8)
		if err != nil {
			logError(w, ctx, "Invalid attending count.", http.StatusBadRequest)
			return
		}
		attending := GuestCap(i)

		event := new(EventInstance)
		eventKey := datastore.NewKey(ctx, "EventInstance", date, 0, nil)
		err = datastore.Get(ctx, eventKey, event)
		if err != nil && err != datastore.ErrNoSuchEntity {
			logError(w, ctx, err.Error(), http.StatusInternalServerError)
			return
		}
		if err == datastore.ErrNoSuchEntity {
			logError(w, ctx, "Event not found.", http.StatusNotFound)
			return
		}

		responseKey := datastore.NewKey(ctx, "Response", "", familyId, eventKey)
		var existingCount GuestCap
		response := new(Response)
		err = datastore.Get(ctx, responseKey, response)
		if err != nil && err != datastore.ErrNoSuchEntity {
			logError(w, ctx, err.Error(), http.StatusInternalServerError)
			return
		}
		if err != datastore.ErrNoSuchEntity {
			existingCount = response.AttendCount
		}

		log.Infof(ctx, "Checking cap: %v new, %v current", attending, existingCount)
		if attending > existingCount {
			var totalCount GuestCap

			q := datastore.NewQuery("Response").
				Ancestor(eventKey)
			for t := q.Run(ctx); ; {
				var r Response
				rFam, err := t.Next(&r)
				if err == datastore.Done {
					break
				}
				if err != nil {
					logError(w, ctx, err.Error(), http.StatusInternalServerError)
					return
				}
				if rFam.IntID() != familyId {
					totalCount += r.AttendCount
				}
			}

			log.Infof(ctx, "Cap check: Cap is %v, existing %v, attending %v", event.Cap, totalCount, attending)
			if totalCount+attending > event.Cap {
				logError(w, ctx, "Too many attendees.", http.StatusUnauthorized)
				return
			}
		}

		response.AttendCount = attending
		response.Note = note
		if _, err = datastore.Put(ctx, responseKey, response); err != nil {
			logError(w, ctx, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Thank you.\n")
	} else {
		// Get this family's responses for those events.
		seattle, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			logError(w, ctx, "Couldn't find Seattle.", http.StatusInternalServerError)
			return
		}
		todayInSeattle := time.Now().In(seattle).Format("2006-01-02")
		q := datastore.NewQuery("EventInstance").
			Filter("__key__ >=", datastore.NewKey(ctx, "EventInstance", todayInSeattle, 0, nil)).
			Order("__key__")
		for t := q.Run(ctx); ; {
			var e EventInstance
			eventKey, err := t.Next(&e)
			if err == datastore.Done {
				break
			}
			if err != nil {
				logError(w, ctx, err.Error(), http.StatusInternalServerError)
				return
			}

			k := datastore.NewKey(ctx, "Response", "", familyId, eventKey)
			r := new(Response)
			err = datastore.Get(ctx, k, r)
			if err != nil && err != datastore.ErrNoSuchEntity {
				logError(w, ctx, err.Error(), http.StatusInternalServerError)
				return
			}
			if err == datastore.ErrNoSuchEntity {
				fmt.Fprintf(w, "%v: No response\n", eventKey.StringID())
			} else {
				fmt.Fprintf(w, "%v: Found response: %v attending, note: %q\n", eventKey.StringID(), r.AttendCount, r.Note)
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
	k := datastore.NewKey(ctx, "EventInstance", "2016-02-01", 0, nil)
	f := new(EventInstance)
	f.Cap = 2
	if _, err := datastore.Put(ctx, k, f); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func adminUsers(w http.ResponseWriter, r *http.Request) {
	// GET or POST?
	ctx := appengine.NewContext(r)
	k := datastore.NewKey(ctx, "Family", "", 1, nil)
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
