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
	"stablelib.com/v1/net/nosurf"
)

type Person struct {
	Email string
}

type Family struct {
	// int key: family ID
	People []Person
	Token  string
	Name   string
}

type Response struct {
	// int key: family ID
	AttendCount GuestCap
	Note        string
	FamilyName  string
}

type GuestCap int8

type EventInstance struct {
	// string key: date
	Notes string
	Cap   GuestCap
}

func addHeaders(handler func(w http.ResponseWriter, r *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := appengine.NewContext(r)

		if r.Host != "spaghetti.sachsfam.org" {
			logError(w, ctx, "Bad host", http.StatusBadRequest)
			return
		}
		if r.URL.Scheme != "https" {
			url := r.URL
			url.Scheme = "https"
			http.Redirect(w, r, url.String(), http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		handler(w, r)
	}

}

func noXSRF(handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return handler
}

func init() {
	http.HandleFunc("/", addHeaders(handler))
	http.HandleFunc("/app/rsvp", addHeaders(rsvp))
	http.HandleFunc("/app/admin/responses", addHeaders(noXSRF(adminResponses)))
	http.HandleFunc("/app/admin/schedule", addHeaders(noXSRF(adminSchedule)))
	http.HandleFunc("/app/admin/users", addHeaders(noXSRF(adminUsers)))
}

func logError(w http.ResponseWriter, ctx context.Context, s string, status int) {
	log.Errorf(ctx, "%v", s)
	buf := make([]byte, 65536)
	runtime.Stack(buf, false)
	log.Errorf(ctx, "%v", string(buf))
	http.Error(w, s, status)
}

func loadFamily(w http.ResponseWriter, r *http.Request, ctx context.Context) (*Family, int64, bool) {
	familyIdStr := r.Form.Get("family")
	if familyIdStr == "" {
		logError(w, ctx, "Missing param family.", http.StatusBadRequest)
		return nil, 0, true
	}

	familyId, err := strconv.ParseInt(familyIdStr, 10, 64)
	if err != nil {
		logError(w, ctx, "Invalid family ID.", http.StatusBadRequest)
		return nil, 0, true
	}

	family := new(Family)
	familyKey := datastore.NewKey(ctx, "Family", "", familyId, nil)
	err = datastore.Get(ctx, familyKey, family)
	if err != nil && err != datastore.ErrNoSuchEntity {
		logError(w, ctx, err.Error(), http.StatusInternalServerError)
		return nil, 0, true
	} else if err == datastore.ErrNoSuchEntity {
		family = nil
	}

	return family, familyId, false
}

type RsvpData struct {
	Event            *EventInstance
	EventKey         *datastore.Key
	ExistingResponse *Response
	NewResponse      *Response
	ResponseKey      *datastore.Key
}

func parseRsvp(familyId int64, w http.ResponseWriter, r *http.Request, ctx context.Context) (RsvpData, bool) {
	var ret RsvpData

	date := r.Form.Get("date")
	attendingStr := r.Form.Get("attending")
	note := r.Form.Get("note")
	if date == "" || attendingStr == "" {
		logError(w, ctx, "Missing date or attending params.", http.StatusBadRequest)
		return ret, true
	}

	i, err := strconv.ParseInt(attendingStr, 10, 8)
	if err != nil {
		logError(w, ctx, "Invalid attending count.", http.StatusBadRequest)
		return ret, true
	}
	attending := GuestCap(i)

	ret.Event = new(EventInstance)
	ret.EventKey = datastore.NewKey(ctx, "EventInstance", date, 0, nil)
	err = datastore.Get(ctx, ret.EventKey, ret.Event)
	if err != nil && err != datastore.ErrNoSuchEntity {
		logError(w, ctx, err.Error(), http.StatusInternalServerError)
		return ret, true
	}
	if err == datastore.ErrNoSuchEntity {
		logError(w, ctx, "Event not found.", http.StatusNotFound)
		return ret, true
	}

	ret.ResponseKey = datastore.NewKey(ctx, "Response", "", familyId, ret.EventKey)
	ret.ExistingResponse = new(Response)
	err = datastore.Get(ctx, ret.ResponseKey, ret.ExistingResponse)
	if err != nil && err != datastore.ErrNoSuchEntity {
		logError(w, ctx, err.Error(), http.StatusInternalServerError)
		return ret, true
	}
	if err == datastore.ErrNoSuchEntity {
		ret.ExistingResponse = nil
	}

	ret.NewResponse = new(Response)
	ret.NewResponse.AttendCount = attending
	ret.NewResponse.Note = note
	return ret, false
}

func queryFutureEvents(w http.ResponseWriter, ctx context.Context) *datastore.Query {
	seattle, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		logError(w, ctx, "Couldn't find Seattle.", http.StatusInternalServerError)
		return nil
	}
	todayInSeattle := time.Now().In(seattle).Format("2006-01-02")
	return datastore.NewQuery("EventInstance").
		Filter("__key__ >=", datastore.NewKey(ctx, "EventInstance", todayInSeattle, 0, nil)).
		Order("__key__")
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

	token := r.Form.Get("token")
	if token == "" {
		logError(w, ctx, "Missing param token.", http.StatusBadRequest)
		return
	}
	family, familyId, err := loadFamily(w, r, ctx)
	if err {
		return
	}
	if family == nil || family.Token != token {
		logError(w, ctx, "Family not found or token invalid.", http.StatusNotFound)
		return
	}

	if r.Method == "POST" {
		rsvpData, err := parseRsvp(familyId, w, r, ctx)
		if err {
			return
		}

		if rsvpData.ExistingResponse == nil || rsvpData.NewResponse.AttendCount > rsvpData.ExistingResponse.AttendCount {
			var totalCount GuestCap

			q := datastore.NewQuery("Response").
				Ancestor(rsvpData.EventKey)
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

			if totalCount+rsvpData.NewResponse.AttendCount > rsvpData.Event.Cap {
				logError(w, ctx, "Too many attendees.", http.StatusUnauthorized)
				return
			}
		}

		if _, err := datastore.Put(ctx, rsvpData.ResponseKey, rsvpData.NewResponse); err != nil {
			logError(w, ctx, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Thank you.\n")
	} else {
		// Get this family's responses for those events.
		q := queryFutureEvents(w, ctx)
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
	/*
	   date: YYYY-MM-DD
	   family: id
	   attending: int
	   note: string
	   xsrf
	*/
	ctx := appengine.NewContext(r)
	r.ParseForm()
	if r.Method == "POST" {
		_, familyId, err := loadFamily(w, r, ctx)
		if err {
			return
		}
		rsvpData, err := parseRsvp(familyId, w, r, ctx)
		if err {
			return
		}

		if _, err := datastore.Put(ctx, rsvpData.ResponseKey, rsvpData.NewResponse); err != nil {
			logError(w, ctx, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Thank you.\n")
	} else {
		nosurf.Token(r) // csrf_token

		q := queryFutureEvents(w, ctx)
		for t := q.Run(ctx); ; {
			var e EventInstance
			_, err := t.Next(&e)
			if err == datastore.Done {
				break
			}
			if err != nil {
				logError(w, ctx, err.Error(), http.StatusInternalServerError)
				return
			}

			q2 := datastore.NewQuery("Response").
				Order("FamilyName")
			for t2 := q2.Run(ctx); ; {
				var r Response
				_, err := t2.Next(&r)
				if err == datastore.Done {
					break
				}
				if err != nil {
					logError(w, ctx, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	}
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
